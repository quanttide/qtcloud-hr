package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
	"github.com/quanttide/qtcloud-human/src/provider/internal/recruitment"
	"github.com/quanttide/qtcloud-human/src/provider/internal/store"
)

type fakeRecruitmentAdapter struct {
	reports []domain.RecruitmentReportRequest
	actions []domain.RecruitmentAdapterActionCall
	err     error
}

func (f *fakeRecruitmentAdapter) CreateReport(ctx context.Context, req domain.RecruitmentReportRequest) (domain.RecruitmentAdapterReportResult, error) {
	f.reports = append(f.reports, req)
	if f.err != nil {
		return domain.RecruitmentAdapterReportResult{}, f.err
	}
	return domain.RecruitmentAdapterReportResult{Markdown: "# 招聘统计报告\n\n- 投递：2"}, nil
}

func (f *fakeRecruitmentAdapter) RunAction(ctx context.Context, candidate domain.RecruitmentCandidate, req domain.RecruitmentActionRequest) (domain.RecruitmentAdapterActionResult, error) {
	f.actions = append(f.actions, domain.RecruitmentAdapterActionCall{Candidate: candidate, Request: req})
	if f.err != nil {
		return domain.RecruitmentAdapterActionResult{}, f.err
	}
	status := domain.RecruitmentActionStatusSent
	if req.IsDryRun(false) {
		status = domain.RecruitmentActionStatusDryRun
	}
	if req.Action == domain.RecruitmentActionCreateInterviewNotice && !req.IsDryRun(false) {
		status = domain.RecruitmentActionStatusDraft
	}
	return domain.RecruitmentAdapterActionResult{Status: status, ExternalMessageID: "msg_123"}, nil
}

func newRecruitmentTestServer(t *testing.T, adapter *fakeRecruitmentAdapter) (*httptest.Server, string) {
	t.Helper()
	logDir := t.TempDir()
	recruitmentStore := store.NewRecruitmentStore([]domain.RecruitmentCandidate{
		{
			ID:        "cand_001",
			Name:      "张三",
			Email:     "zhangsan@example.com",
			Position:  "数据工程师",
			Stage:     "new",
			Status:    "pending",
			HasResume: true,
		},
	})
	h := NewRecruitmentHandler(recruitmentStore, adapter, recruitment.NewFileAuditLogger(logDir), RecruitmentHandlerConfig{})
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux), logDir
}

func postJSON(t *testing.T, url string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Operator", "tester")
	req.Header.Set("X-Recruitment-Permission", "write")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestRecruitmentReportEndpointCreatesDryRunReport(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, logDir := newRecruitmentTestServer(t, adapter)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/v1/recruitment/reports", `{"days":30,"dry_run":true}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var got domain.RecruitmentReportResult
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ReportID == "" || got.Status != "dry_run" || !strings.Contains(got.Markdown, "招聘统计报告") {
		t.Fatalf("unexpected report response: %+v", got)
	}
	if len(adapter.reports) != 1 || adapter.reports[0].Days == nil || *adapter.reports[0].Days != 30 || !adapter.reports[0].IsDryRun(false) {
		t.Fatalf("adapter did not receive structured report request: %+v", adapter.reports)
	}
	data, err := os.ReadFile(filepath.Join(logDir, "recruitment-actions.jsonl"))
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if !strings.Contains(string(data), "create_report") || strings.Contains(string(data), "QTRECURIT") {
		t.Fatalf("audit log should contain action metadata only: %s", data)
	}
}

func TestRecruitmentReportDefaultsToDryRunWhenConfigured(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	logDir := t.TempDir()
	recruitmentStore := store.NewRecruitmentStore(nil)
	h := NewRecruitmentHandler(recruitmentStore, adapter, recruitment.NewFileAuditLogger(logDir), RecruitmentHandlerConfig{DryRunDefault: true})
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/v1/recruitment/reports", `{"days":30}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if len(adapter.reports) != 1 || !adapter.reports[0].IsDryRun(false) {
		t.Fatalf("adapter should receive normalized dry_run=true: %+v", adapter.reports)
	}
}

func TestRecruitmentCandidateActionsValidateWhitelistAndRequiredParams(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, _ := newRecruitmentTestServer(t, adapter)
	defer ts.Close()

	cases := []struct {
		name string
		body string
	}{
		{name: "unknown action", body: `{"action":"pass","dry_run":true,"params":{}}`},
		{name: "extra exam params", body: `{"action":"send_exam","dry_run":true,"params":{"unsafe":"--debug"}}`},
		{name: "missing interview position", body: `{"action":"create_interview_notice","dry_run":true,"params":{"time":"2026-09-10 10:00"}}`},
		{name: "invalid survey link", body: `{"action":"send_survey","dry_run":true,"params":{"link":"not a url"}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postJSON(t, ts.URL+"/api/v1/recruitment/candidates/cand_001/actions", tc.body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.StatusCode)
			}
		})
	}
	if len(adapter.actions) != 0 {
		t.Fatalf("adapter should not be called on invalid input: %+v", adapter.actions)
	}
}

func TestRecruitmentCandidateActionsCallAdapterAndAudit(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, logDir := newRecruitmentTestServer(t, adapter)
	defer ts.Close()

	validBodies := []string{
		`{"action":"send_survey","dry_run":true,"params":{"link":"https://example.com/survey"}}`,
		`{"action":"send_training_invite","dry_run":true,"params":{"qr":"oss://qtcloud-human-studio/assets/training-group.png"}}`,
		`{"action":"send_exam","dry_run":true,"params":{}}`,
		`{"action":"create_interview_notice","dry_run":true,"params":{"position":"数据工程师","time":"2026-09-10 10:00"}}`,
	}

	for _, body := range validBodies {
		resp := postJSON(t, ts.URL+"/api/v1/recruitment/candidates/cand_001/actions", body)
		if resp.StatusCode != http.StatusCreated {
			resp.Body.Close()
			t.Fatalf("expected 201 for %s, got %d", body, resp.StatusCode)
		}
		var got domain.RecruitmentActionResult
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			resp.Body.Close()
			t.Fatalf("decode action response: %v", err)
		}
		resp.Body.Close()
		if got.Status != domain.RecruitmentActionStatusDryRun || got.ActionID == "" || got.CandidateID != "cand_001" {
			t.Fatalf("unexpected action response: %+v", got)
		}
	}

	if len(adapter.actions) != 4 {
		t.Fatalf("expected 4 adapter calls, got %d", len(adapter.actions))
	}
	data, err := os.ReadFile(filepath.Join(logDir, "recruitment-actions.jsonl"))
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	logText := string(data)
	for _, want := range []string{"send_survey", "send_training_invite", "send_exam", "create_interview_notice", "tester"} {
		if !strings.Contains(logText, want) {
			t.Fatalf("audit log missing %q: %s", want, logText)
		}
	}
	for _, leak := range []string{"https://example.com/survey", "oss://qtcloud-human-studio", "数据工程师", "2026-09-10"} {
		if strings.Contains(logText, leak) {
			t.Fatalf("audit log leaked request params %q: %s", leak, logText)
		}
	}
}

func TestRecruitmentCandidateActionDefaultsToDryRunWhenConfigured(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	logDir := t.TempDir()
	recruitmentStore := store.NewRecruitmentStore([]domain.RecruitmentCandidate{
		{
			ID:     "cand_001",
			Name:   "张三",
			Email:  "zhangsan@example.com",
			Stage:  "new",
			Status: "pending",
		},
	})
	h := NewRecruitmentHandler(recruitmentStore, adapter, recruitment.NewFileAuditLogger(logDir), RecruitmentHandlerConfig{DryRunDefault: true})
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/v1/recruitment/candidates/cand_001/actions", `{"action":"send_exam","params":{}}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if len(adapter.actions) != 1 || !adapter.actions[0].Request.IsDryRun(false) {
		t.Fatalf("adapter should receive normalized dry_run=true: %+v", adapter.actions)
	}
}

func TestRecruitmentEndpointRequiresOperator(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, _ := newRecruitmentTestServer(t, adapter)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/recruitment/reports", "application/json", strings.NewReader(`{"days":30}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRecruitmentSendActionRequiresWritePermission(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, _ := newRecruitmentTestServer(t, adapter)
	defer ts.Close()

	req, err := http.NewRequest(
		http.MethodPost,
		ts.URL+"/api/v1/recruitment/candidates/cand_001/actions",
		strings.NewReader(`{"action":"send_exam","dry_run":true,"params":{}}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Operator", "tester")
	req.Header.Set("X-Recruitment-Permission", "read")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
	if len(adapter.actions) != 0 {
		t.Fatalf("adapter should not be called without write permission")
	}
}

func TestRecruitmentAdapterErrorsAreSanitized(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{err: errors.New("qtrecurit access survey failed token=secret stack trace mail body")}
	ts, logDir := newRecruitmentTestServer(t, adapter)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/v1/recruitment/candidates/cand_001/actions", `{"action":"send_survey","dry_run":false,"params":{}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", resp.StatusCode)
	}
	body := new(bytes.Buffer)
	body.ReadFrom(resp.Body)
	for _, leak := range []string{"qtrecurit", "token=secret", "stack", "mail body"} {
		if strings.Contains(body.String(), leak) {
			t.Fatalf("response leaked %q: %s", leak, body.String())
		}
	}
	data, err := os.ReadFile(filepath.Join(logDir, "recruitment-actions.jsonl"))
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	for _, leak := range []string{"token=secret", "stack trace", "mail body"} {
		if strings.Contains(string(data), leak) {
			t.Fatalf("audit log leaked %q: %s", leak, data)
		}
	}
}
