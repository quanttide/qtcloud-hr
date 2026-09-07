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
	"time"

	"github.com/quanttide/qtcloud-human/src/provider/internal/auth"
	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
	"github.com/quanttide/qtcloud-human/src/provider/internal/recruitment"
	"github.com/quanttide/qtcloud-human/src/provider/internal/store"
)

type fakeRecruitmentAdapter struct {
	reports    []domain.RecruitmentReportRequest
	actions    []domain.RecruitmentAdapterActionCall
	inboxSyncs []domain.RecruitmentInboxSyncRequest
	resumes    []domain.RecruitmentResumeAttachment
	status     domain.RecruitmentProviderStatus
	resumePath string
	err        error
}

type fakeUserInfoAuthorizer struct {
	principal auth.Principal
	err       error
}

func (f fakeUserInfoAuthorizer) Authorize(context.Context, string) (auth.Principal, error) {
	if f.err != nil {
		return auth.Principal{}, f.err
	}
	return f.principal, nil
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

func (f *fakeRecruitmentAdapter) SyncInbox(ctx context.Context, req domain.RecruitmentInboxSyncRequest) (domain.RecruitmentAdapterInboxSyncResult, error) {
	f.inboxSyncs = append(f.inboxSyncs, req)
	if f.err != nil {
		return domain.RecruitmentAdapterInboxSyncResult{}, f.err
	}
	return domain.RecruitmentAdapterInboxSyncResult{
		Status:   "synced",
		Mailbox:  req.Mailbox,
		Folder:   req.Folder,
		Scanned:  3,
		Imported: 1,
		Candidates: []domain.RecruitmentCandidate{
			{
				ID:             "cand_imported",
				Name:           "李四",
				Email:          "lisi@example.com",
				Subject:        "应聘后端开发",
				Body:           "HR 您好，我想投递后端开发岗位，附件是我的简历。",
				Position:       "后端开发",
				Stage:          "new",
				Status:         "pending",
				HasResume:      true,
				HasCoverLetter: true,
				ResumeAttachments: []domain.RecruitmentResumeAttachment{
					{
						FileName:    "李四-后端开发简历.pdf",
						ContentType: "application/pdf",
						URL:         "https://files.example.test/resumes/cand_imported.pdf",
					},
				},
			},
		},
	}, nil
}

func (f *fakeRecruitmentAdapter) FetchResume(ctx context.Context, candidate domain.RecruitmentCandidate, attachment domain.RecruitmentResumeAttachment) (domain.RecruitmentAdapterResumeResult, error) {
	f.resumes = append(f.resumes, attachment)
	if f.err != nil {
		return domain.RecruitmentAdapterResumeResult{}, f.err
	}
	return domain.RecruitmentAdapterResumeResult{
		Path:        f.resumePath,
		FileName:    attachment.FileName,
		ContentType: attachment.ContentType,
		SizeBytes:   7,
	}, nil
}

func (f *fakeRecruitmentAdapter) CheckProviderStatus(ctx context.Context) domain.RecruitmentProviderStatus {
	if f.status.Status != "" {
		return f.status
	}
	return domain.RecruitmentProviderStatus{
		Status:  "ready",
		Ready:   true,
		Mailbox: "hr@quanttide.com",
		Components: []domain.RecruitmentProviderStatusComponent{
			{Name: "qtrecurit", Status: "ok", Message: "qtrecurit 可执行"},
			{Name: "hr_mailbox", Status: "ok", Message: "HR 邮箱可访问"},
		},
	}
}

func newRecruitmentTestServer(t *testing.T, adapter *fakeRecruitmentAdapter) (*httptest.Server, string) {
	return newRecruitmentTestServerWithConfig(t, adapter, RecruitmentHandlerConfig{})
}

func newRecruitmentTestServerWithConfig(t *testing.T, adapter *fakeRecruitmentAdapter, config RecruitmentHandlerConfig) (*httptest.Server, string) {
	t.Helper()
	logDir := t.TempDir()
	recruitmentStore := store.NewRecruitmentStore([]domain.RecruitmentCandidate{
		{
			ID:              "cand_001",
			Name:            "张三",
			Email:           "zhangsan@example.com",
			Position:        "数据工程师",
			Stage:           "new",
			Status:          "pending",
			HasResume:       true,
			SourceMessageID: "message_001",
			ResumeAttachments: []domain.RecruitmentResumeAttachment{
				{
					FileName:    "张三-后端开发简历.pdf",
					ContentType: "application/pdf",
					SourceID:    "attachment_001",
				},
			},
		},
	})
	h := NewRecruitmentHandler(recruitmentStore, adapter, recruitment.NewFileAuditLogger(logDir), config)
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

func TestRecruitmentProviderStatusEndpointReturnsSanitizedDiagnostics(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{
		status: domain.RecruitmentProviderStatus{
			Status:  "blocked",
			Ready:   false,
			Mailbox: "hr@quanttide.com",
			Message: "provider 环境未就绪",
			Components: []domain.RecruitmentProviderStatusComponent{
				{Name: "qtrecurit", Status: "ok", Message: "qtrecurit 可执行", Version: "qtrecurit 0.1.0"},
				{Name: "hr_mailbox", Status: "failed", Message: "HR 邮箱认证态不可用"},
			},
		},
	}
	ts, _ := newRecruitmentTestServer(t, adapter)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/recruitment/provider/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Operator", "tester")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got domain.RecruitmentProviderStatus
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if got.Ready || got.Status != "blocked" || got.Mailbox != "hr@quanttide.com" || len(got.Components) != 2 {
		t.Fatalf("unexpected provider status: %+v", got)
	}
	serialized, _ := json.Marshal(got)
	for _, leak := range []string{"token=", "authorization", "password", "mail body"} {
		if strings.Contains(strings.ToLower(string(serialized)), leak) {
			t.Fatalf("provider status leaked %q: %s", leak, serialized)
		}
	}
}

func TestRecruitmentProviderStatusRequiresOperator(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, _ := newRecruitmentTestServer(t, adapter)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/recruitment/provider/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRecruitmentAuthenticatorRejectsLegacyHeaders(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, _ := newRecruitmentTestServerWithConfig(t, adapter, RecruitmentHandlerConfig{
		Authenticator: fakeUserInfoAuthorizer{
			principal: auth.Principal{Subject: "user-001"},
		},
	})
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/recruitment/candidates", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Operator", "spoofed")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated request to succeed, got %d", resp.StatusCode)
	}
}

func TestRecruitmentAuthenticatorControlsWritePermission(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, _ := newRecruitmentTestServerWithConfig(t, adapter, RecruitmentHandlerConfig{
		Authenticator: fakeUserInfoAuthorizer{
			principal: auth.Principal{Subject: "user-readonly"},
		},
		RecruitmentWriters: []string{"user-writer"},
	})
	defer ts.Close()

	req, err := http.NewRequest(
		http.MethodPost,
		ts.URL+"/api/v1/recruitment/inbox/sync",
		strings.NewReader(`{"mailbox":"hr@quanttide.com","folder":"INBOX","page_size":25,"dry_run":true}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer readonly-token")
	req.Header.Set("X-Recruitment-Permission", "write")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected readonly principal to be rejected, got %d", resp.StatusCode)
	}
	if len(adapter.inboxSyncs) != 0 {
		t.Fatalf("adapter should not be called for readonly principal")
	}
}

func TestRecruitmentGatewaySecretBlocksDirectAccess(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, _ := newRecruitmentTestServerWithConfig(t, adapter, RecruitmentHandlerConfig{
		GatewaySecret: "gateway-secret",
	})
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/recruitment/candidates", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Operator", "tester")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected direct access to be rejected, got %d", resp.StatusCode)
	}

	reqWithGateway, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/recruitment/candidates", nil)
	if err != nil {
		t.Fatal(err)
	}
	reqWithGateway.Header.Set("X-Operator", "tester")
	reqWithGateway.Header.Set("X-Qtcloud-Gateway-Secret", "gateway-secret")
	allowedResp, err := http.DefaultClient.Do(reqWithGateway)
	if err != nil {
		t.Fatal(err)
	}
	defer allowedResp.Body.Close()
	if allowedResp.StatusCode != http.StatusOK {
		t.Fatalf("expected gateway request to succeed, got %d", allowedResp.StatusCode)
	}
}

func TestRecruitmentGatewaySecretProtectsResumeView(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, _ := newRecruitmentTestServerWithConfig(t, adapter, RecruitmentHandlerConfig{
		GatewaySecret: "gateway-secret",
	})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/recruitment/resume-view/token")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected direct resume access to be rejected, got %d", resp.StatusCode)
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

func TestRecruitmentLiveAdapterCallsDisabledByDefault(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, _ := newRecruitmentTestServer(t, adapter)
	defer ts.Close()

	cases := []struct {
		name string
		url  string
		body string
	}{
		{name: "report", url: "/api/v1/recruitment/reports", body: `{"days":30,"dry_run":false}`},
		{name: "sync", url: "/api/v1/recruitment/inbox/sync", body: `{"mailbox":"hr@quanttide.com","folder":"INBOX","page_size":25,"dry_run":false}`},
		{name: "action", url: "/api/v1/recruitment/candidates/cand_001/actions", body: `{"action":"send_exam","dry_run":false,"params":{}}`},
		{name: "resume", url: "/api/v1/recruitment/candidates/cand_001/resume/0/view", body: `{}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postJSON(t, ts.URL+tc.url, tc.body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("expected 403, got %d", resp.StatusCode)
			}
		})
	}
	if len(adapter.reports) != 0 || len(adapter.inboxSyncs) != 0 || len(adapter.actions) != 0 || len(adapter.resumes) != 0 {
		t.Fatalf("adapter should not be called when live actions are disabled: %+v", adapter)
	}
}

func TestRecruitmentLiveActionsRequireReadyProvider(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{
		status: domain.RecruitmentProviderStatus{
			Status:  "blocked",
			Ready:   false,
			Mailbox: "hr@quanttide.com",
		},
	}
	ts, _ := newRecruitmentTestServerWithConfig(t, adapter, RecruitmentHandlerConfig{
		AllowRealActions: true,
	})
	defer ts.Close()

	cases := []struct {
		name string
		url  string
		body string
	}{
		{name: "report", url: "/api/v1/recruitment/reports", body: `{"days":30,"dry_run":false}`},
		{name: "sync", url: "/api/v1/recruitment/inbox/sync", body: `{"dry_run":false}`},
		{name: "action", url: "/api/v1/recruitment/candidates/cand_001/actions", body: `{"action":"send_exam","dry_run":false,"params":{}}`},
		{name: "resume", url: "/api/v1/recruitment/candidates/cand_001/resume/0/view", body: `{}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postJSON(t, ts.URL+tc.url, tc.body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d", resp.StatusCode)
			}
		})
	}
	if len(adapter.reports) != 0 || len(adapter.inboxSyncs) != 0 || len(adapter.actions) != 0 || len(adapter.resumes) != 0 {
		t.Fatalf("adapter should not execute live operations while provider is blocked: reports=%d syncs=%d actions=%d resumes=%d", len(adapter.reports), len(adapter.inboxSyncs), len(adapter.actions), len(adapter.resumes))
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

func TestRecruitmentCandidateStatusUpdatePersistsManualDecision(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, logDir := newRecruitmentTestServer(t, adapter)
	defer ts.Close()

	req, err := http.NewRequest(
		http.MethodPatch,
		ts.URL+"/api/v1/recruitment/candidates/cand_001",
		strings.NewReader(`{"status":"passed"}`),
	)
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var updated domain.RecruitmentCandidate
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode status update response: %v", err)
	}
	if updated.Status != "passed" {
		t.Fatalf("status = %q, want passed", updated.Status)
	}

	listReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/recruitment/candidates", nil)
	if err != nil {
		t.Fatal(err)
	}
	listReq.Header.Set("X-Operator", "tester")
	listResp, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	var candidates []domain.RecruitmentCandidate
	if err := json.NewDecoder(listResp.Body).Decode(&candidates); err != nil {
		t.Fatalf("decode candidate list: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Status != "passed" {
		t.Fatalf("manual decision was not persisted in list response: %+v", candidates)
	}
	if len(adapter.actions) != 0 {
		t.Fatalf("manual status update should not call action adapter: %+v", adapter.actions)
	}

	data, err := os.ReadFile(filepath.Join(logDir, "recruitment-actions.jsonl"))
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	logText := string(data)
	if !strings.Contains(logText, "update_candidate_status") || strings.Contains(logText, "zhangsan@example.com") {
		t.Fatalf("audit log should contain status metadata only: %s", logText)
	}
}

func TestRecruitmentInboxSyncCallsAdapterUpsertsCandidatesAndAudits(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, logDir := newRecruitmentTestServerWithConfig(t, adapter, RecruitmentHandlerConfig{AllowRealActions: true})
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/v1/recruitment/inbox/sync", `{"mailbox":"hr@quanttide.com","folder":"INBOX","page_size":25,"dry_run":false}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got domain.RecruitmentInboxSyncResult
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode sync response: %v", err)
	}
	if got.SyncID == "" || got.Status != "synced" || got.Imported != 1 || len(got.Candidates) != 1 {
		t.Fatalf("unexpected sync response: %+v", got)
	}
	if len(adapter.inboxSyncs) != 1 || adapter.inboxSyncs[0].PageSize == nil || *adapter.inboxSyncs[0].PageSize != 25 {
		t.Fatalf("adapter did not receive structured inbox request: %+v", adapter.inboxSyncs)
	}

	listReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/recruitment/candidates", nil)
	if err != nil {
		t.Fatal(err)
	}
	listReq.Header.Set("X-Operator", "tester")
	listResp, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	var candidates []domain.RecruitmentCandidate
	if err := json.NewDecoder(listResp.Body).Decode(&candidates); err != nil {
		t.Fatalf("decode candidate list: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected inbox sync to replace current candidate snapshot, got %+v", candidates)
	}
	foundImported := false
	for _, candidate := range candidates {
		if candidate.ID == "cand_imported" {
			foundImported = true
			if candidate.Subject != "应聘后端开发" || !strings.Contains(candidate.Body, "投递后端开发岗位") {
				t.Fatalf("expected imported candidate to keep email subject/body, got %+v", candidate)
			}
			if len(candidate.ResumeAttachments) != 1 || candidate.ResumeAttachments[0].URL == "" {
				t.Fatalf("expected imported candidate to keep resume attachment metadata, got %+v", candidate.ResumeAttachments)
			}
		}
	}
	if !foundImported {
		t.Fatalf("imported candidate missing from list: %+v", candidates)
	}

	data, err := os.ReadFile(filepath.Join(logDir, "recruitment-actions.jsonl"))
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	logText := string(data)
	if !strings.Contains(logText, "sync_inbox") || strings.Contains(logText, "lisi@example.com") {
		t.Fatalf("audit log should contain sync metadata only: %s", logText)
	}
}

func TestRecruitmentInboxSyncValidatesInputs(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, _ := newRecruitmentTestServer(t, adapter)
	defer ts.Close()

	cases := []struct {
		name string
		body string
	}{
		{name: "invalid mailbox", body: `{"mailbox":"not-an-email","folder":"INBOX","page_size":25,"dry_run":true}`},
		{name: "invalid folder", body: `{"mailbox":"hr@quanttide.com","folder":"INBOX\n--debug","page_size":25,"dry_run":true}`},
		{name: "page too large", body: `{"mailbox":"hr@quanttide.com","folder":"INBOX","page_size":500,"dry_run":true}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postJSON(t, ts.URL+"/api/v1/recruitment/inbox/sync", tc.body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.StatusCode)
			}
		})
	}
	if len(adapter.inboxSyncs) != 0 {
		t.Fatalf("adapter should not be called on invalid input: %+v", adapter.inboxSyncs)
	}
}

func TestRecruitmentResumeViewCreatesShortLivedURLAndServesPDF(t *testing.T) {
	resumeDir := t.TempDir()
	cacheRoot := filepath.Join(resumeDir, "qtrecurit", "inbox", "resume-files")
	resumeDir = filepath.Join(cacheRoot, "resume-key")
	if err := os.MkdirAll(resumeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	resumePath := filepath.Join(resumeDir, "resume.pdf")
	if err := os.WriteFile(resumePath, []byte("pdfdata"), 0o600); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeRecruitmentAdapter{resumePath: resumePath}
	ts, logDir := newRecruitmentTestServerWithConfig(t, adapter, RecruitmentHandlerConfig{ResumeCacheRoot: cacheRoot, AllowRealActions: true})
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/v1/recruitment/candidates/cand_001/resume/0/view", `{}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var view domain.RecruitmentResumeViewResult
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		t.Fatalf("decode resume view response: %v", err)
	}
	if !strings.HasPrefix(view.URL, "/api/v1/recruitment/resume-view/") || view.ExpiresAt.IsZero() {
		t.Fatalf("unexpected resume view response: %+v", view)
	}
	if len(adapter.resumes) != 1 || adapter.resumes[0].SourceID != "attachment_001" {
		t.Fatalf("adapter should receive selected attachment: %+v", adapter.resumes)
	}

	fileResp, err := http.Get(ts.URL + view.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer fileResp.Body.Close()
	if fileResp.StatusCode != http.StatusOK {
		t.Fatalf("expected file response 200, got %d", fileResp.StatusCode)
	}
	if contentType := fileResp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/pdf") {
		t.Fatalf("content-type = %q, want application/pdf", contentType)
	}
	if disposition := fileResp.Header.Get("Content-Disposition"); !strings.HasPrefix(disposition, "inline;") {
		t.Fatalf("content-disposition = %q, want inline", disposition)
	}

	downloadResp, err := http.Get(ts.URL + view.URL + "?download=1")
	if err != nil {
		t.Fatal(err)
	}
	defer downloadResp.Body.Close()
	if downloadResp.StatusCode != http.StatusOK {
		t.Fatalf("expected download response 200, got %d", downloadResp.StatusCode)
	}
	if disposition := downloadResp.Header.Get("Content-Disposition"); !strings.HasPrefix(disposition, "attachment;") {
		t.Fatalf("download content-disposition = %q, want attachment", disposition)
	}
	auditData, err := os.ReadFile(filepath.Join(logDir, "recruitment-actions.jsonl"))
	if err != nil {
		t.Fatalf("read resume audit log: %v", err)
	}
	if !strings.Contains(string(auditData), `"action":"view_resume"`) || !strings.Contains(string(auditData), `"action":"download_resume"`) {
		t.Fatalf("resume access should be audited: %s", auditData)
	}
}

func TestRecruitmentResumeViewLoadsStateAcrossHandlerInstances(t *testing.T) {
	resumeRoot := filepath.Join(t.TempDir(), "qtrecurit", "inbox", "resume-files")
	resumeDir := filepath.Join(resumeRoot, "resume-key")
	if err := os.MkdirAll(resumeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	resumePath := filepath.Join(resumeDir, "resume.pdf")
	if err := os.WriteFile(resumePath, []byte("pdfdata"), 0o600); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(t.TempDir(), "state", "resume-views.json")
	config := RecruitmentHandlerConfig{
		AllowRealActions:    true,
		ResumeCacheRoot:     resumeRoot,
		ResumeViewStatePath: statePath,
	}

	first, _ := newRecruitmentTestServerWithConfig(t, &fakeRecruitmentAdapter{resumePath: resumePath}, config)
	resp := postJSON(t, first.URL+"/api/v1/recruitment/candidates/cand_001/resume/0/view", `{}`)
	defer resp.Body.Close()
	var view domain.RecruitmentResumeViewResult
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		t.Fatalf("decode resume view response: %v", err)
	}
	first.Close()

	second, _ := newRecruitmentTestServerWithConfig(t, &fakeRecruitmentAdapter{resumePath: resumePath}, config)
	defer second.Close()
	fileResp, err := http.Get(second.URL + view.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer fileResp.Body.Close()
	if fileResp.StatusCode != http.StatusOK {
		t.Fatalf("expected cross-instance file response 200, got %d", fileResp.StatusCode)
	}
}

func TestRecruitmentResumeViewRejectsFilesOutsideCacheRoot(t *testing.T) {
	cacheRoot := filepath.Join(t.TempDir(), "qtrecurit", "inbox", "resume-files")
	outsidePath := filepath.Join(t.TempDir(), "resume.pdf")
	if err := os.WriteFile(outsidePath, []byte("pdfdata"), 0o600); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeRecruitmentAdapter{resumePath: outsidePath}
	ts, _ := newRecruitmentTestServerWithConfig(t, adapter, RecruitmentHandlerConfig{ResumeCacheRoot: cacheRoot, AllowRealActions: true})
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/v1/recruitment/candidates/cand_001/resume/0/view", `{}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", resp.StatusCode)
	}
}

func TestRecruitmentResumeViewRejectsPersistedFilesOutsideCacheRoot(t *testing.T) {
	cacheRoot := filepath.Join(t.TempDir(), "qtrecurit", "inbox", "resume-files")
	outsidePath := filepath.Join(t.TempDir(), "resume.pdf")
	if err := os.WriteFile(outsidePath, []byte("pdfdata"), 0o600); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(t.TempDir(), "state", "resume-views.json")
	views := map[string]resumeView{
		"persisted-token": {
			Path:        outsidePath,
			FileName:    "resume.pdf",
			ContentType: "application/pdf",
			ExpiresAt:   time.Now().UTC().Add(time.Minute),
		},
	}
	data, err := json.Marshal(views)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	ts, _ := newRecruitmentTestServerWithConfig(t, &fakeRecruitmentAdapter{}, RecruitmentHandlerConfig{
		ResumeCacheRoot:     cacheRoot,
		ResumeViewStatePath: statePath,
	})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/recruitment/resume-view/persisted-token")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for persisted file outside cache root, got %d", resp.StatusCode)
	}
}

func TestRecruitmentResumeViewRequiresOperator(t *testing.T) {
	adapter := &fakeRecruitmentAdapter{}
	ts, _ := newRecruitmentTestServer(t, adapter)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/recruitment/candidates/cand_001/resume/0/view", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	if len(adapter.resumes) != 0 {
		t.Fatalf("adapter should not be called without operator")
	}
}

func TestRecruitmentResumeViewRequiresWriter(t *testing.T) {
	resumeRoot := filepath.Join(t.TempDir(), "qtrecurit", "inbox", "resume-files")
	resumeDir := filepath.Join(resumeRoot, "resume-key")
	if err := os.MkdirAll(resumeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	resumePath := filepath.Join(resumeDir, "resume.pdf")
	if err := os.WriteFile(resumePath, []byte("pdfdata"), 0o600); err != nil {
		t.Fatal(err)
	}
	adapter := &fakeRecruitmentAdapter{resumePath: resumePath}
	ts, _ := newRecruitmentTestServerWithConfig(t, adapter, RecruitmentHandlerConfig{
		AllowRealActions: true,
		ResumeCacheRoot:  resumeRoot,
	})
	defer ts.Close()

	req, err := http.NewRequest(
		http.MethodPost,
		ts.URL+"/api/v1/recruitment/candidates/cand_001/resume/0/view",
		strings.NewReader(`{}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Operator", "readonly")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected readonly resume access to be rejected, got %d", resp.StatusCode)
	}
	if len(adapter.resumes) != 0 {
		t.Fatalf("adapter should not fetch resume for readonly operator")
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
	ts, logDir := newRecruitmentTestServerWithConfig(t, adapter, RecruitmentHandlerConfig{AllowRealActions: true})
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
