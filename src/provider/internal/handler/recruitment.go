package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
	"github.com/quanttide/qtcloud-human/src/provider/internal/recruitment"
	"github.com/quanttide/qtcloud-human/src/provider/internal/store"
)

const defaultDryRun = true

var isoDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type RecruitmentAdapter interface {
	CreateReport(ctx context.Context, req domain.RecruitmentReportRequest) (domain.RecruitmentAdapterReportResult, error)
	RunAction(ctx context.Context, candidate domain.RecruitmentCandidate, req domain.RecruitmentActionRequest) (domain.RecruitmentAdapterActionResult, error)
}

type RecruitmentHandlerConfig struct {
	DryRunDefault bool
}

type RecruitmentHandler struct {
	store         *store.RecruitmentStore
	adapter       RecruitmentAdapter
	audit         recruitment.AuditLogger
	dryRunDefault bool
}

func NewRecruitmentHandler(s *store.RecruitmentStore, adapter RecruitmentAdapter, audit recruitment.AuditLogger, config RecruitmentHandlerConfig) *RecruitmentHandler {
	if s == nil {
		s = store.DefaultRecruitmentStore()
	}
	if audit == nil {
		audit = recruitment.SlogAuditLogger{}
	}
	return &RecruitmentHandler{
		store:         s,
		adapter:       adapter,
		audit:         audit,
		dryRunDefault: config.DryRunDefault,
	}
}

func (h *RecruitmentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/recruitment/candidates", h.ListCandidates)
	mux.HandleFunc("POST /api/v1/recruitment/reports", h.CreateReport)
	mux.HandleFunc("POST /api/v1/recruitment/candidates/{candidate_id}/actions", h.RunCandidateAction)
}

func (h *RecruitmentHandler) ListCandidates(w http.ResponseWriter, r *http.Request) {
	if operator := operatorFromRequest(r); operator == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "operator required"})
		return
	}
	writeJSON(w, http.StatusOK, h.store.ListCandidates())
}

func (h *RecruitmentHandler) CreateReport(w http.ResponseWriter, r *http.Request) {
	operator := operatorFromRequest(r)
	if operator == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "operator required"})
		return
	}
	var req domain.RecruitmentReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := validateReportRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	result, err := h.adapter.CreateReport(r.Context(), req)
	now := time.Now().UTC()
	status := domain.RecruitmentReportStatusCreated
	if req.IsDryRun(h.dryRunDefault) {
		status = domain.RecruitmentReportStatusDryRun
	}
	response := domain.RecruitmentReportResult{
		ReportID:  newID("rpt", now),
		Status:    status,
		Markdown:  result.Markdown,
		Metrics:   result.Metrics,
		CreatedAt: now,
	}
	if err != nil {
		h.logAudit(domain.RecruitmentAuditEntry{
			ActionID:  response.ReportID,
			Action:    domain.RecruitmentActionCreateReport,
			Operator:  operator,
			Status:    domain.RecruitmentActionStatusFailed,
			Message:   publicAdapterError(err),
			CreatedAt: now,
		})
		writeJSON(w, adapterErrorStatus(err), map[string]string{"error": publicAdapterError(err)})
		return
	}
	h.logAudit(domain.RecruitmentAuditEntry{
		ActionID:  response.ReportID,
		Action:    domain.RecruitmentActionCreateReport,
		Operator:  operator,
		Status:    status,
		Message:   "report created",
		CreatedAt: now,
	})
	writeJSON(w, http.StatusCreated, response)
}

func (h *RecruitmentHandler) RunCandidateAction(w http.ResponseWriter, r *http.Request) {
	operator := operatorFromRequest(r)
	if operator == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "operator required"})
		return
	}
	if !canWriteRecruitment(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "recruitment write permission required"})
		return
	}
	candidateID := r.PathValue("candidate_id")
	candidate, ok := h.store.GetCandidate(candidateID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "candidate not found"})
		return
	}
	if err := validateCandidate(candidate); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var req domain.RecruitmentActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	if err := validateActionRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	adapterResult, err := h.adapter.RunAction(r.Context(), candidate, req)
	now := time.Now().UTC()
	status := adapterResult.Status
	if status == "" {
		status = domain.RecruitmentActionStatusSent
	}
	message := adapterResult.Message
	if message == "" {
		message = actionSuccessMessage(req.Action, status)
	}
	response := domain.RecruitmentActionResult{
		ActionID:          newID("act", now),
		CandidateID:       candidate.ID,
		Action:            req.Action,
		Status:            status,
		Message:           message,
		ExternalMessageID: adapterResult.ExternalMessageID,
		CreatedAt:         now,
	}
	if err != nil {
		response.Status = domain.RecruitmentActionStatusFailed
		response.Message = publicAdapterError(err)
		h.logAudit(auditEntry(response, operator))
		writeJSON(w, adapterErrorStatus(err), map[string]string{"error": publicAdapterError(err)})
		return
	}

	if !req.IsDryRun(h.dryRunDefault) {
		h.store.RecordAction(candidate.ID, req.Action, stageAfterAction(req.Action, status))
	}
	h.logAudit(auditEntry(response, operator))
	writeJSON(w, http.StatusCreated, response)
}

func validateReportRequest(req domain.RecruitmentReportRequest) error {
	if req.Days != nil && (*req.Days < 1 || *req.Days > 366) {
		return errors.New("days must be between 1 and 366")
	}
	if req.Start != nil && !validISODate(*req.Start) {
		return errors.New("start must use YYYY-MM-DD")
	}
	if req.End != nil && !validISODate(*req.End) {
		return errors.New("end must use YYYY-MM-DD")
	}
	if (req.Start == nil) != (req.End == nil) {
		return errors.New("start and end must be provided together")
	}
	if req.Days != nil && (req.Start != nil || req.End != nil) {
		return errors.New("days cannot be combined with start/end")
	}
	if req.Start != nil && req.End != nil && *req.Start > *req.End {
		return errors.New("start must be before or equal to end")
	}
	return nil
}

func validateActionRequest(req domain.RecruitmentActionRequest) error {
	allowed := allowedActionParams(req.Action)
	if allowed == nil {
		return errors.New("unsupported recruitment action")
	}
	for key := range req.Params {
		if !allowed[key] {
			return fmt.Errorf("unsupported parameter: %s", key)
		}
	}
	switch req.Action {
	case domain.RecruitmentActionSendSurvey:
		return validateOptionalURL(req.Params, "link", "survey link")
	case domain.RecruitmentActionSendTrainingInvite:
		return validateOptionalResource(req.Params, "qr", "qr")
	case domain.RecruitmentActionSendExam:
		return nil
	case domain.RecruitmentActionCreateInterviewNotice:
		if err := requireString(req.Params, "position", 80); err != nil {
			return err
		}
		return requireString(req.Params, "time", 80)
	default:
		return errors.New("unsupported recruitment action")
	}
}

func allowedActionParams(action string) map[string]bool {
	switch action {
	case domain.RecruitmentActionSendSurvey:
		return map[string]bool{"link": true}
	case domain.RecruitmentActionSendTrainingInvite:
		return map[string]bool{"qr": true}
	case domain.RecruitmentActionSendExam:
		return map[string]bool{}
	case domain.RecruitmentActionCreateInterviewNotice:
		return map[string]bool{"position": true, "time": true}
	default:
		return nil
	}
}

func validateCandidate(candidate domain.RecruitmentCandidate) error {
	if candidate.ID == "" || candidate.Name == "" || candidate.Email == "" {
		return errors.New("candidate id, name, and email are required")
	}
	if len(candidate.Name) > 80 {
		return errors.New("candidate name is too long")
	}
	if _, err := mail.ParseAddress(candidate.Email); err != nil || len(candidate.Email) > 254 {
		return errors.New("candidate email is invalid")
	}
	return nil
}

func requireString(params map[string]any, key string, max int) error {
	value, ok := params[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", key)
	}
	if len([]rune(value)) > max {
		return fmt.Errorf("%s is too long", key)
	}
	return nil
}

func validateOptionalURL(params map[string]any, key string, label string) error {
	value, ok := params[key]
	if !ok {
		return nil
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return fmt.Errorf("%s must be a URL", label)
	}
	parsed, err := url.ParseRequestURI(text)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid http(s) URL", label)
	}
	if len(text) > 2048 {
		return fmt.Errorf("%s is too long", label)
	}
	return nil
}

func validateOptionalResource(params map[string]any, key string, label string) error {
	value, ok := params[key]
	if !ok {
		return nil
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return fmt.Errorf("%s must be a resource path", label)
	}
	if strings.ContainsAny(text, "\x00\r\n") || len(text) > 2048 {
		return fmt.Errorf("%s is invalid", label)
	}
	parsed, err := url.Parse(text)
	if err == nil && parsed.Scheme != "" {
		if parsed.Scheme != "oss" && parsed.Scheme != "https" {
			return fmt.Errorf("%s must use oss or https", label)
		}
	}
	return nil
}

func validISODate(value string) bool {
	if !isoDatePattern.MatchString(value) {
		return false
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

func operatorFromRequest(r *http.Request) string {
	operator := strings.TrimSpace(r.Header.Get("X-Operator"))
	if operator == "" {
		return ""
	}
	if len([]rune(operator)) > 80 {
		return operator[:80]
	}
	return operator
}

func canWriteRecruitment(r *http.Request) bool {
	permission := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Recruitment-Permission")))
	return permission == "write" || permission == "admin"
}

func auditEntry(result domain.RecruitmentActionResult, operator string) domain.RecruitmentAuditEntry {
	return domain.RecruitmentAuditEntry{
		ActionID:          result.ActionID,
		CandidateID:       result.CandidateID,
		Action:            result.Action,
		Operator:          operator,
		Status:            result.Status,
		Message:           result.Message,
		ExternalMessageID: result.ExternalMessageID,
		CreatedAt:         result.CreatedAt,
	}
}

func (h *RecruitmentHandler) logAudit(entry domain.RecruitmentAuditEntry) {
	if err := h.audit.Log(entry); err != nil {
		slog.Error("write recruitment audit log", "error", err)
	}
}

func stageAfterAction(action string, status string) string {
	if status == domain.RecruitmentActionStatusDryRun || status == domain.RecruitmentActionStatusFailed {
		return ""
	}
	switch action {
	case domain.RecruitmentActionSendSurvey:
		return "survey_sent"
	case domain.RecruitmentActionSendTrainingInvite:
		return "invite_sent"
	case domain.RecruitmentActionSendExam:
		return "exam_sent"
	case domain.RecruitmentActionCreateInterviewNotice:
		return "interview_draft"
	default:
		return ""
	}
}

func actionSuccessMessage(action string, status string) string {
	if status == domain.RecruitmentActionStatusDryRun {
		return "已完成 dry_run 预览，未发送邮件"
	}
	switch action {
	case domain.RecruitmentActionSendSurvey:
		return "问卷邮件已发送"
	case domain.RecruitmentActionSendTrainingInvite:
		return "实训邀约已发送"
	case domain.RecruitmentActionSendExam:
		return "笔试邀请已发送"
	case domain.RecruitmentActionCreateInterviewNotice:
		return "面试通知草稿已生成"
	default:
		return "招聘动作已完成"
	}
}

func publicAdapterError(err error) string {
	if errors.Is(err, recruitment.ErrAdapterTimeout) {
		return "招聘服务执行超时，请稍后重试"
	}
	return "招聘服务暂不可用，请稍后重试"
}

func adapterErrorStatus(err error) int {
	if errors.Is(err, recruitment.ErrAdapterTimeout) {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}

func newID(prefix string, now time.Time) string {
	return prefix + "_" + now.Format("20060102_150405_000000000")
}
