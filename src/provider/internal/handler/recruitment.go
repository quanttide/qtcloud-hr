package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/quanttide/qtcloud-human/src/provider/internal/auth"
	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
	"github.com/quanttide/qtcloud-human/src/provider/internal/recruitment"
	"github.com/quanttide/qtcloud-human/src/provider/internal/store"
)

const defaultDryRun = true
const resumeViewTTL = 5 * time.Minute
const candidateNotFoundMessage = "候选人不存在或数据已过期，请重新拉取新邮件"
const resumeAttachmentNotFoundMessage = "简历附件不存在或数据已过期，请重新拉取新邮件"

var isoDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type RecruitmentAdapter interface {
	CreateReport(ctx context.Context, req domain.RecruitmentReportRequest) (domain.RecruitmentAdapterReportResult, error)
	SyncInbox(ctx context.Context, req domain.RecruitmentInboxSyncRequest) (domain.RecruitmentAdapterInboxSyncResult, error)
	FetchResume(ctx context.Context, candidate domain.RecruitmentCandidate, attachment domain.RecruitmentResumeAttachment) (domain.RecruitmentAdapterResumeResult, error)
	RunAction(ctx context.Context, candidate domain.RecruitmentCandidate, req domain.RecruitmentActionRequest) (domain.RecruitmentAdapterActionResult, error)
	CheckProviderStatus(ctx context.Context) domain.RecruitmentProviderStatus
}

type resumeView struct {
	Path        string    `json:"path"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	Operator    string    `json:"operator"`
	CandidateID string    `json:"candidate_id"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type RecruitmentHandlerConfig struct {
	DryRunDefault       bool
	AllowRealActions    bool
	ResumeCacheRoot     string
	ResumeViewStatePath string
	Authenticator       auth.UserInfoAuthorizer
	RecruitmentWriters  []string
	GatewaySecret       string
}

type RecruitmentHandler struct {
	store               *store.RecruitmentStore
	adapter             RecruitmentAdapter
	audit               recruitment.AuditLogger
	dryRunDefault       bool
	allowRealActions    bool
	resumeCacheRoot     string
	resumeViewStatePath string
	authenticator       auth.UserInfoAuthorizer
	recruitmentWriters  map[string]bool
	gatewaySecret       string
	resumeViews         map[string]resumeView
	resumeViewMu        sync.Mutex
}

func NewRecruitmentHandler(s *store.RecruitmentStore, adapter RecruitmentAdapter, audit recruitment.AuditLogger, config RecruitmentHandlerConfig) *RecruitmentHandler {
	if s == nil {
		s = store.DefaultRecruitmentStore()
	}
	if audit == nil {
		audit = recruitment.SlogAuditLogger{}
	}
	writers := make(map[string]bool, len(config.RecruitmentWriters))
	for _, writer := range config.RecruitmentWriters {
		writer = strings.TrimSpace(writer)
		if writer != "" {
			writers[writer] = true
		}
	}
	return &RecruitmentHandler{
		store:               s,
		adapter:             adapter,
		audit:               audit,
		dryRunDefault:       config.DryRunDefault,
		allowRealActions:    config.AllowRealActions,
		resumeCacheRoot:     cleanOptionalAbsPath(config.ResumeCacheRoot),
		resumeViewStatePath: cleanOptionalAbsPath(config.ResumeViewStatePath),
		authenticator:       config.Authenticator,
		recruitmentWriters:  writers,
		gatewaySecret:       strings.TrimSpace(config.GatewaySecret),
		resumeViews:         map[string]resumeView{},
	}
}

func (h *RecruitmentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/recruitment/provider/status", h.ProviderStatus)
	mux.HandleFunc("GET /api/v1/recruitment/candidates", h.ListCandidates)
	mux.HandleFunc("POST /api/v1/recruitment/inbox/sync", h.SyncInbox)
	mux.HandleFunc("POST /api/v1/recruitment/reports", h.CreateReport)
	mux.HandleFunc("PATCH /api/v1/recruitment/candidates/{candidate_id}", h.UpdateCandidateStatus)
	mux.HandleFunc("POST /api/v1/recruitment/candidates/{candidate_id}/actions", h.RunCandidateAction)
	mux.HandleFunc("POST /api/v1/recruitment/candidates/{candidate_id}/resume/{attachment_index}/view", h.CreateResumeView)
	mux.HandleFunc("GET /api/v1/recruitment/resume-view/{token}", h.ServeResumeView)
}

func (h *RecruitmentHandler) ProviderStatus(w http.ResponseWriter, r *http.Request) {
	operator, ok := h.authenticate(w, r, false)
	if !ok {
		return
	}
	status := h.adapter.CheckProviderStatus(r.Context())
	status.Operator = operator
	writeJSON(w, http.StatusOK, status)
}

func (h *RecruitmentHandler) ListCandidates(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authenticate(w, r, false); !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.store.ListCandidates())
}

func (h *RecruitmentHandler) CreateReport(w http.ResponseWriter, r *http.Request) {
	operator, ok := h.authenticate(w, r, false)
	if !ok {
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
	req = h.normalizeReportRequest(req)
	if !req.IsDryRun(h.dryRunDefault) && !h.allowRealActions {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "real recruitment actions are disabled"})
		return
	}
	if !req.IsDryRun(h.dryRunDefault) && !h.providerReady(r.Context()) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "招聘服务环境未就绪，请稍后重试"})
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

func (h *RecruitmentHandler) SyncInbox(w http.ResponseWriter, r *http.Request) {
	operator, ok := h.authenticate(w, r, true)
	if !ok {
		return
	}
	var req domain.RecruitmentInboxSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := validateInboxSyncRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	req = h.normalizeInboxSyncRequest(req)
	if !req.IsDryRun(h.dryRunDefault) && !h.allowRealActions {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "real recruitment actions are disabled"})
		return
	}
	if !req.IsDryRun(h.dryRunDefault) && !h.providerReady(r.Context()) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "招聘服务环境未就绪，请稍后重试"})
		return
	}

	adapterResult, err := h.adapter.SyncInbox(r.Context(), req)
	now := time.Now().UTC()
	status := adapterResult.Status
	if status == "" {
		status = domain.RecruitmentInboxStatusSynced
	}
	response := domain.RecruitmentInboxSyncResult{
		SyncID:     newID("sync", now),
		Status:     status,
		Mailbox:    adapterResult.Mailbox,
		Folder:     adapterResult.Folder,
		Scanned:    adapterResult.Scanned,
		Imported:   adapterResult.Imported,
		Candidates: adapterResult.Candidates,
		CreatedAt:  now,
	}
	if err != nil {
		response.Status = domain.RecruitmentInboxStatusFailed
		h.logAudit(domain.RecruitmentAuditEntry{
			ActionID:  response.SyncID,
			Action:    domain.RecruitmentActionSyncInbox,
			Operator:  operator,
			Status:    domain.RecruitmentActionStatusFailed,
			Message:   publicAdapterError(err),
			CreatedAt: now,
		})
		writeJSON(w, adapterErrorStatus(err), map[string]string{"error": publicAdapterError(err)})
		return
	}
	if !req.IsDryRun(h.dryRunDefault) {
		h.store.ReplaceCandidates(adapterResult.Candidates)
	}
	h.logAudit(domain.RecruitmentAuditEntry{
		ActionID:  response.SyncID,
		Action:    domain.RecruitmentActionSyncInbox,
		Operator:  operator,
		Status:    response.Status,
		Message:   fmt.Sprintf("inbox sync completed: scanned=%d imported=%d", response.Scanned, response.Imported),
		CreatedAt: now,
	})
	writeJSON(w, http.StatusOK, response)
}

func (h *RecruitmentHandler) UpdateCandidateStatus(w http.ResponseWriter, r *http.Request) {
	operator, ok := h.authenticate(w, r, true)
	if !ok {
		return
	}
	var req domain.RecruitmentCandidateStatusUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	status := normalizeCandidateStatus(req.Status)
	if status == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be passed or rejected"})
		return
	}
	candidate, ok := h.store.UpdateCandidateStatus(r.PathValue("candidate_id"), status)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": candidateNotFoundMessage})
		return
	}
	h.logAudit(domain.RecruitmentAuditEntry{
		ActionID:    newID("status", candidate.UpdatedAt),
		CandidateID: candidate.ID,
		Action:      domain.RecruitmentActionUpdateCandidateStatus,
		Operator:    operator,
		Status:      candidate.Status,
		Message:     "candidate status updated",
		CreatedAt:   candidate.UpdatedAt,
	})
	writeJSON(w, http.StatusOK, candidate)
}

func (h *RecruitmentHandler) RunCandidateAction(w http.ResponseWriter, r *http.Request) {
	operator, ok := h.authenticate(w, r, true)
	if !ok {
		return
	}
	candidateID := r.PathValue("candidate_id")
	candidate, ok := h.store.GetCandidate(candidateID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": candidateNotFoundMessage})
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
	req = h.normalizeActionRequest(req)
	if !req.IsDryRun(h.dryRunDefault) && !h.allowRealActions {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "real recruitment actions are disabled"})
		return
	}
	if !req.IsDryRun(h.dryRunDefault) && !h.providerReady(r.Context()) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "招聘服务环境未就绪，请稍后重试"})
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

func (h *RecruitmentHandler) CreateResumeView(w http.ResponseWriter, r *http.Request) {
	operator, ok := h.authenticate(w, r, true)
	if !ok {
		return
	}
	if !h.allowRealActions {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "real recruitment actions are disabled"})
		return
	}
	if !h.providerReady(r.Context()) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "招聘服务环境未就绪，请稍后重试"})
		return
	}
	candidate, attachment, ok := h.resumeAttachmentFromRequest(w, r)
	if !ok {
		return
	}
	if attachment.SourceID == "" || candidate.SourceMessageID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "resume attachment is not downloadable"})
		return
	}
	if !isAllowedResumeFileName(attachment.FileName) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported resume file type"})
		return
	}
	adapterResult, err := h.adapter.FetchResume(r.Context(), candidate, attachment)
	if err != nil {
		writeJSON(w, adapterErrorStatus(err), map[string]string{"error": publicAdapterError(err)})
		return
	}
	resolved, err := filepath.Abs(adapterResult.Path)
	if err != nil || !isAllowedResumeFileName(adapterResult.FileName) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "招聘服务暂不可用，请稍后重试"})
		return
	}
	if !pathIsUnderRoot(resolved, h.resumeCacheRoot) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "招聘服务暂不可用，请稍后重试"})
		return
	}
	if info, err := os.Stat(resolved); err != nil || info.IsDir() {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "招聘服务暂不可用，请稍后重试"})
		return
	}

	token, err := newResumeToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "招聘服务暂不可用，请稍后重试"})
		return
	}
	expiresAt := time.Now().UTC().Add(resumeViewTTL)
	h.resumeViewMu.Lock()
	h.loadResumeViewsLocked()
	h.pruneResumeViewsLocked(time.Now().UTC())
	h.resumeViews[token] = resumeView{
		Path:        resolved,
		FileName:    adapterResult.FileName,
		ContentType: firstNonEmpty(adapterResult.ContentType, mime.TypeByExtension(filepath.Ext(adapterResult.FileName))),
		Operator:    operator,
		CandidateID: candidate.ID,
		ExpiresAt:   expiresAt,
	}
	h.persistResumeViewsLocked()
	h.resumeViewMu.Unlock()

	writeJSON(w, http.StatusCreated, domain.RecruitmentResumeViewResult{
		URL:       "/api/v1/recruitment/resume-view/" + token,
		ExpiresAt: expiresAt,
	})
}

func (h *RecruitmentHandler) ServeResumeView(w http.ResponseWriter, r *http.Request) {
	if !h.gatewaySecretMatches(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "gateway required"})
		return
	}
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "resume view not found"})
		return
	}
	now := time.Now().UTC()
	h.resumeViewMu.Lock()
	h.pruneResumeViewsLocked(now)
	view, ok := h.resumeViews[token]
	if !ok {
		h.loadResumeViewsLocked()
		h.pruneResumeViewsLocked(now)
		view, ok = h.resumeViews[token]
	}
	h.resumeViewMu.Unlock()
	if !ok || now.After(view.ExpiresAt) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "resume view not found"})
		return
	}
	if info, err := os.Stat(view.Path); err != nil || info.IsDir() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "resume view not found"})
		return
	}
	if !pathIsUnderRoot(view.Path, h.resumeCacheRoot) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "resume view not found"})
		return
	}
	contentType := firstNonEmpty(view.ContentType, mime.TypeByExtension(filepath.Ext(view.FileName)), "application/octet-stream")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", contentDisposition(view.FileName, contentType, r.URL.Query().Get("download") == "1"))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	action := "view_resume"
	status := domain.RecruitmentActionStatusViewed
	if r.URL.Query().Get("download") == "1" {
		action = "download_resume"
		status = domain.RecruitmentActionStatusDownloaded
	}
	h.logAudit(domain.RecruitmentAuditEntry{
		ActionID:    newID("resume", now),
		CandidateID: view.CandidateID,
		Action:      action,
		Operator:    view.Operator,
		Status:      status,
		Message:     action,
		CreatedAt:   now,
	})
	http.ServeFile(w, r, view.Path)
}

func (h *RecruitmentHandler) providerReady(ctx context.Context) bool {
	status := h.adapter.CheckProviderStatus(ctx)
	return status.Ready && status.Status == "ready"
}

func (h *RecruitmentHandler) authenticate(w http.ResponseWriter, r *http.Request, requireWrite bool) (string, bool) {
	if !h.gatewaySecretMatches(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "gateway required"})
		return "", false
	}

	operator := ""
	if h.authenticator != nil {
		principal, err := h.authenticator.Authorize(r.Context(), r.Header.Get("Authorization"))
		if err != nil || strings.TrimSpace(principal.Subject) == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return "", false
		}
		operator = principal.Subject
		if requireWrite && !h.recruitmentWriters[operator] {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "recruitment write permission required"})
			return "", false
		}
		return operator, true
	}

	operator = operatorFromRequest(r)
	if operator == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "operator required"})
		return "", false
	}
	if requireWrite && !canWriteRecruitment(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "recruitment write permission required"})
		return "", false
	}
	return operator, true
}

func (h *RecruitmentHandler) gatewaySecretMatches(r *http.Request) bool {
	if h.gatewaySecret == "" {
		return true
	}
	actual := []byte(r.Header.Get("X-Qtcloud-Gateway-Secret"))
	expected := []byte(h.gatewaySecret)
	return len(actual) == len(expected) && subtle.ConstantTimeCompare(actual, expected) == 1
}

func (h *RecruitmentHandler) resumeAttachmentFromRequest(w http.ResponseWriter, r *http.Request) (domain.RecruitmentCandidate, domain.RecruitmentResumeAttachment, bool) {
	candidateID := strings.TrimSpace(r.PathValue("candidate_id"))
	candidate, ok := h.store.GetCandidate(candidateID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": candidateNotFoundMessage})
		return domain.RecruitmentCandidate{}, domain.RecruitmentResumeAttachment{}, false
	}
	index, err := strconv.Atoi(r.PathValue("attachment_index"))
	if err != nil || index < 0 || index >= len(candidate.ResumeAttachments) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": resumeAttachmentNotFoundMessage})
		return domain.RecruitmentCandidate{}, domain.RecruitmentResumeAttachment{}, false
	}
	return candidate, candidate.ResumeAttachments[index], true
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

func (h *RecruitmentHandler) normalizeReportRequest(req domain.RecruitmentReportRequest) domain.RecruitmentReportRequest {
	if req.DryRun == nil {
		dryRun := h.dryRunDefault
		req.DryRun = &dryRun
	}
	return req
}

func (h *RecruitmentHandler) normalizeActionRequest(req domain.RecruitmentActionRequest) domain.RecruitmentActionRequest {
	if req.DryRun == nil {
		dryRun := h.dryRunDefault
		req.DryRun = &dryRun
	}
	return req
}

func (h *RecruitmentHandler) normalizeInboxSyncRequest(req domain.RecruitmentInboxSyncRequest) domain.RecruitmentInboxSyncRequest {
	if strings.TrimSpace(req.Mailbox) == "" {
		req.Mailbox = "hr@quanttide.com"
	}
	if strings.TrimSpace(req.Folder) == "" {
		req.Folder = "INBOX"
	}
	if req.PageSize == nil {
		pageSize := 50
		req.PageSize = &pageSize
	}
	if req.DryRun == nil {
		dryRun := h.dryRunDefault
		req.DryRun = &dryRun
	}
	return req
}

func validateInboxSyncRequest(req domain.RecruitmentInboxSyncRequest) error {
	mailbox := strings.TrimSpace(req.Mailbox)
	if mailbox != "" {
		if _, err := mail.ParseAddress(mailbox); err != nil || len(mailbox) > 254 {
			return errors.New("mailbox is invalid")
		}
	}
	folder := strings.TrimSpace(req.Folder)
	if folder != "" && (len([]rune(folder)) > 80 || strings.ContainsAny(folder, "\x00\r\n")) {
		return errors.New("folder is invalid")
	}
	if req.PageSize != nil && (*req.PageSize < 1 || *req.PageSize > 100) {
		return errors.New("page_size must be between 1 and 100")
	}
	return nil
}

func normalizeCandidateStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "passed":
		return "passed"
	case "rejected":
		return "rejected"
	default:
		return ""
	}
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

func isAllowedResumeFileName(fileName string) bool {
	lower := strings.ToLower(strings.TrimSpace(fileName))
	return strings.HasSuffix(lower, ".pdf") || strings.HasSuffix(lower, ".doc") || strings.HasSuffix(lower, ".docx")
}

func newResumeToken() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func (h *RecruitmentHandler) pruneResumeViewsLocked(now time.Time) {
	for token, view := range h.resumeViews {
		if now.After(view.ExpiresAt) {
			delete(h.resumeViews, token)
		}
	}
}

func contentDisposition(fileName string, contentType string, download bool) string {
	disposition := "attachment"
	if contentType == "application/pdf" && !download {
		disposition = "inline"
	}
	return fmt.Sprintf("%s; filename=%q", disposition, filepath.Base(fileName))
}

func (h *RecruitmentHandler) loadResumeViewsLocked() {
	if h.resumeViewStatePath == "" {
		return
	}
	data, err := os.ReadFile(h.resumeViewStatePath)
	if err != nil {
		return
	}
	var views map[string]resumeView
	if err := json.Unmarshal(data, &views); err != nil {
		return
	}
	for token, view := range views {
		h.resumeViews[token] = view
	}
}

func (h *RecruitmentHandler) persistResumeViewsLocked() {
	if h.resumeViewStatePath == "" {
		return
	}
	data, err := json.Marshal(h.resumeViews)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(h.resumeViewStatePath), 0o700); err != nil {
		slog.Error("create resume view state directory", "path", h.resumeViewStatePath, "error", err)
		return
	}
	temp, err := os.CreateTemp(filepath.Dir(h.resumeViewStatePath), ".resume-views-*.tmp")
	if err != nil {
		slog.Error("create resume view state temp file", "path", h.resumeViewStatePath, "error", err)
		return
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		slog.Error("protect resume view state temp file", "path", h.resumeViewStatePath, "error", err)
		_ = temp.Close()
		return
	}
	if _, err := temp.Write(data); err != nil {
		slog.Error("write resume view state", "path", h.resumeViewStatePath, "error", err)
		_ = temp.Close()
		return
	}
	if err := temp.Sync(); err != nil {
		slog.Error("sync resume view state", "path", h.resumeViewStatePath, "error", err)
		_ = temp.Close()
		return
	}
	if err := temp.Close(); err != nil {
		slog.Error("close resume view state", "path", h.resumeViewStatePath, "error", err)
		return
	}
	if err := os.Rename(tempPath, h.resumeViewStatePath); err != nil {
		if removeErr := os.Remove(h.resumeViewStatePath); removeErr == nil {
			err = os.Rename(tempPath, h.resumeViewStatePath)
		}
	}
	if err != nil {
		slog.Error("replace resume view state", "path", h.resumeViewStatePath, "error", err)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cleanOptionalAbsPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return filepath.Clean(resolved)
}

func pathIsUnderRoot(path string, root string) bool {
	if root == "" {
		return true
	}
	resolved := filepath.Clean(path)
	relative, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	return relative == "." || (relative != "" && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && relative != "..")
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
