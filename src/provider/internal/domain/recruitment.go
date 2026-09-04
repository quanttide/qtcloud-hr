package domain

import "time"

const (
	RecruitmentActionCreateReport          = "create_report"
	RecruitmentActionSendSurvey            = "send_survey"
	RecruitmentActionSendTrainingInvite    = "send_training_invite"
	RecruitmentActionSendExam              = "send_exam"
	RecruitmentActionCreateInterviewNotice = "create_interview_notice"
	RecruitmentActionStatusDraft           = "draft"
	RecruitmentActionStatusSent            = "sent"
	RecruitmentActionStatusFailed          = "failed"
	RecruitmentActionStatusDryRun          = "dry_run"
	RecruitmentReportStatusCreated         = "created"
	RecruitmentReportStatusDryRun          = "dry_run"
	RecruitmentReportStatusFailed          = "failed"
)

type RecruitmentCandidate struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Email           string    `json:"email"`
	Position        string    `json:"position,omitempty"`
	Stage           string    `json:"stage"`
	Status          string    `json:"status"`
	HasResume       bool      `json:"has_resume"`
	HasCoverLetter  bool      `json:"has_cover_letter"`
	SourceMessageID string    `json:"source_message_id,omitempty"`
	LastAction      string    `json:"last_action,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type RecruitmentReportRequest struct {
	Days   *int    `json:"days,omitempty"`
	Start  *string `json:"start,omitempty"`
	End    *string `json:"end,omitempty"`
	DryRun *bool   `json:"dry_run,omitempty"`
}

func (r RecruitmentReportRequest) IsDryRun(defaultValue bool) bool {
	if r.DryRun == nil {
		return defaultValue
	}
	return *r.DryRun
}

type RecruitmentReportResult struct {
	ReportID  string         `json:"report_id"`
	Status    string         `json:"status"`
	Markdown  string         `json:"markdown"`
	Metrics   map[string]int `json:"metrics,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type RecruitmentActionRequest struct {
	Action string         `json:"action"`
	DryRun *bool          `json:"dry_run,omitempty"`
	Params map[string]any `json:"params"`
}

func (r RecruitmentActionRequest) IsDryRun(defaultValue bool) bool {
	if r.DryRun == nil {
		return defaultValue
	}
	return *r.DryRun
}

type RecruitmentActionResult struct {
	ActionID          string    `json:"action_id"`
	CandidateID       string    `json:"candidate_id"`
	Action            string    `json:"action"`
	Status            string    `json:"status"`
	Message           string    `json:"message"`
	ExternalMessageID string    `json:"external_message_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type RecruitmentAdapterReportResult struct {
	Markdown string
	Metrics  map[string]int
}

type RecruitmentAdapterActionResult struct {
	Status            string
	Message           string
	ExternalMessageID string
}

type RecruitmentAdapterActionCall struct {
	Candidate RecruitmentCandidate
	Request   RecruitmentActionRequest
}

type RecruitmentAuditEntry struct {
	ActionID          string    `json:"action_id"`
	CandidateID       string    `json:"candidate_id,omitempty"`
	Action            string    `json:"action"`
	Operator          string    `json:"operator"`
	Status            string    `json:"status"`
	Message           string    `json:"message"`
	ExternalMessageID string    `json:"external_message_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}
