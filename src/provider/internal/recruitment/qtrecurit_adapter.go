package recruitment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
)

const (
	defaultRecruitmentMailbox    = "hr@quanttide.com"
	defaultProviderStatusTimeout = 5 * time.Second
)

var (
	ErrAdapterFailed  = errors.New("recruitment adapter failed")
	ErrAdapterTimeout = errors.New("recruitment adapter timed out")
)

type CommandRunner interface {
	Run(ctx context.Context, binary string, args []string) (stdout string, stderr string, err error)
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, binary string, args []string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

type CLIAdapter struct {
	Binary        string
	ArgsPrefix    []string
	Timeout       time.Duration
	StatusTimeout time.Duration
	Runner        CommandRunner
}

type qtrecuritInboxSyncResult struct {
	Status     string                    `json:"status"`
	Mailbox    string                    `json:"mailbox"`
	Folder     string                    `json:"folder"`
	Scanned    int                       `json:"scanned"`
	Imported   int                       `json:"imported"`
	Candidates []qtrecuritInboxCandidate `json:"candidates"`
}

type qtrecuritInboxCandidate struct {
	ID                string                      `json:"id"`
	Name              string                      `json:"name"`
	Email             string                      `json:"email"`
	MessageID         string                      `json:"message_id"`
	Subject           string                      `json:"subject"`
	Body              string                      `json:"body"`
	Position          string                      `json:"position"`
	Stage             string                      `json:"stage"`
	Status            string                      `json:"status"`
	HasResume         bool                        `json:"has_resume"`
	HasCoverLetter    bool                        `json:"has_cover_letter"`
	ResumeAttachments []qtrecuritResumeAttachment `json:"resume_attachments"`
	Attachments       []qtrecuritResumeAttachment `json:"attachments"`
	Message           qtrecuritInboxMessage       `json:"message"`
	Raw               qtrecuritInboxMessage       `json:"raw"`
	SourceMessageID   string                      `json:"source_message_id"`
	ReceivedAt        string                      `json:"received_at"`
	UpdatedAt         string                      `json:"updated_at"`
}

type qtrecuritInboxMessage struct {
	Attachments []qtrecuritResumeAttachment `json:"attachments"`
}

type qtrecuritResumeAttachment struct {
	ID           string `json:"id"`
	AttachmentID string `json:"attachment_id"`
	FileName     string `json:"file_name"`
	Name         string `json:"name"`
	Filename     string `json:"filename"`
	ContentType  string `json:"content_type"`
	MIMEType     string `json:"mime_type"`
	URL          string `json:"url"`
	DownloadURL  string `json:"download_url"`
	PreviewURL   string `json:"preview_url"`
	SizeBytes    int64  `json:"size_bytes"`
	Size         int64  `json:"size"`
}

type qtrecuritResumeResult struct {
	Status      string `json:"status"`
	Path        string `json:"path"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

func NewCLIAdapter(binary string, timeout time.Duration) *CLIAdapter {
	if binary == "" {
		binary = "qtrecurit"
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &CLIAdapter{
		Binary:        binary,
		Timeout:       timeout,
		StatusTimeout: defaultProviderStatusTimeout,
		Runner:        ExecCommandRunner{},
	}
}

func (a *CLIAdapter) CreateReport(ctx context.Context, req domain.RecruitmentReportRequest) (domain.RecruitmentAdapterReportResult, error) {
	if req.IsDryRun(false) {
		return domain.RecruitmentAdapterReportResult{
			Markdown: "# 招聘统计报告\n\n> dry_run：未读取 HR 邮箱。\n",
		}, nil
	}
	args := BuildReportArgs(req)
	stdout, stderr, err := a.run(ctx, args)
	if err != nil {
		return domain.RecruitmentAdapterReportResult{}, err
	}
	if strings.TrimSpace(stdout) == "" {
		if strings.TrimSpace(stderr) != "" {
			return domain.RecruitmentAdapterReportResult{}, fmt.Errorf("%w: empty report output", ErrAdapterFailed)
		}
		return domain.RecruitmentAdapterReportResult{}, fmt.Errorf("%w: empty report output", ErrAdapterFailed)
	}
	return domain.RecruitmentAdapterReportResult{Markdown: stdout}, nil
}

func (a *CLIAdapter) RunAction(ctx context.Context, candidate domain.RecruitmentCandidate, req domain.RecruitmentActionRequest) (domain.RecruitmentAdapterActionResult, error) {
	args := BuildActionArgs(candidate, req)
	stdout, stderr, err := a.run(ctx, args)
	if err != nil {
		return domain.RecruitmentAdapterActionResult{}, err
	}
	status := actionStatus(req.Action, req.IsDryRun(false), stdout, stderr)
	return domain.RecruitmentAdapterActionResult{
		Status:  status,
		Message: actionMessage(req.Action, status),
	}, nil
}

func (a *CLIAdapter) FetchResume(ctx context.Context, candidate domain.RecruitmentCandidate, attachment domain.RecruitmentResumeAttachment) (domain.RecruitmentAdapterResumeResult, error) {
	args := BuildResumeArgs(candidate, attachment)
	stdout, _, err := a.run(ctx, args)
	if err != nil {
		return domain.RecruitmentAdapterResumeResult{}, err
	}
	var decoded qtrecuritResumeResult
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		return domain.RecruitmentAdapterResumeResult{}, fmt.Errorf("%w: invalid resume output", ErrAdapterFailed)
	}
	if strings.TrimSpace(decoded.Path) == "" || strings.TrimSpace(decoded.FileName) == "" {
		return domain.RecruitmentAdapterResumeResult{}, fmt.Errorf("%w: invalid resume output", ErrAdapterFailed)
	}
	return domain.RecruitmentAdapterResumeResult{
		Path:        decoded.Path,
		FileName:    decoded.FileName,
		ContentType: decoded.ContentType,
		SizeBytes:   decoded.SizeBytes,
	}, nil
}

func (a *CLIAdapter) CheckProviderStatus(ctx context.Context) domain.RecruitmentProviderStatus {
	statusTimeout := a.StatusTimeout
	if statusTimeout <= 0 {
		statusTimeout = defaultProviderStatusTimeout
	}
	statusCtx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()

	componentResults := make(chan domain.RecruitmentProviderStatusComponent, 2)
	go func() {
		componentResults <- a.checkQtrecurit(statusCtx)
	}()
	go func() {
		componentResults <- a.checkMailbox(statusCtx)
	}()
	components := make([]domain.RecruitmentProviderStatusComponent, 2)
	for range components {
		component := <-componentResults
		if component.Name == "qtrecurit" {
			components[0] = component
		} else {
			components[1] = component
		}
	}
	ready := true
	for _, component := range components {
		if component.Status != "ok" {
			ready = false
			break
		}
	}
	status := "ready"
	message := "provider 可执行 qtrecurit，并可访问 HR 邮箱"
	if !ready {
		status = "blocked"
		message = "provider 环境未就绪，请检查 qtrecurit 二进制和 HR 邮箱认证态"
	}
	return domain.RecruitmentProviderStatus{
		Status:     status,
		Ready:      ready,
		Mailbox:    defaultRecruitmentMailbox,
		Message:    message,
		Components: components,
		CheckedAt:  time.Now().UTC(),
	}
}

func (a *CLIAdapter) checkQtrecurit(ctx context.Context) domain.RecruitmentProviderStatusComponent {
	stdout, _, err := a.runRaw(ctx, a.Binary, []string{"--version"})
	if err != nil {
		return domain.RecruitmentProviderStatusComponent{
			Name:    "qtrecurit",
			Status:  "failed",
			Message: "qtrecurit 不可执行，请检查 provider 镜像是否包含二进制文件",
		}
	}
	return domain.RecruitmentProviderStatusComponent{
		Name:    "qtrecurit",
		Status:  "ok",
		Message: "qtrecurit 可执行",
		Version: safeStatusLine(stdout),
	}
}

func (a *CLIAdapter) checkMailbox(ctx context.Context) domain.RecruitmentProviderStatusComponent {
	_, _, err := a.runRaw(ctx, "lark-cli", []string{
		"mail", "user_mailbox.folders", "list",
		"--user-mailbox-id", defaultRecruitmentMailbox,
		"--folder-type", "1",
		"--as", "user",
		"--format", "json",
	})
	if err != nil {
		return domain.RecruitmentProviderStatusComponent{
			Name:    "hr_mailbox",
			Status:  "failed",
			Message: "HR 邮箱认证态不可用，请在 provider 运行环境配置 lark-cli 登录状态",
		}
	}
	return domain.RecruitmentProviderStatusComponent{
		Name:    "hr_mailbox",
		Status:  "ok",
		Message: "HR 邮箱可访问",
	}
}

func (a *CLIAdapter) SyncInbox(ctx context.Context, req domain.RecruitmentInboxSyncRequest) (domain.RecruitmentAdapterInboxSyncResult, error) {
	if req.IsDryRun(false) {
		return domain.RecruitmentAdapterInboxSyncResult{
			Status:     domain.RecruitmentInboxStatusDryRun,
			Mailbox:    inboxMailbox(req),
			Folder:     inboxFolder(req),
			Candidates: []domain.RecruitmentCandidate{},
		}, nil
	}
	args := BuildInboxSyncArgs(req)
	stdout, _, err := a.run(ctx, args)
	if err != nil {
		return domain.RecruitmentAdapterInboxSyncResult{}, err
	}
	var decoded qtrecuritInboxSyncResult
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		return domain.RecruitmentAdapterInboxSyncResult{}, fmt.Errorf("%w: invalid inbox sync output", ErrAdapterFailed)
	}
	result := domain.RecruitmentAdapterInboxSyncResult{
		Status:     decoded.Status,
		Mailbox:    decoded.Mailbox,
		Folder:     decoded.Folder,
		Scanned:    decoded.Scanned,
		Imported:   decoded.Imported,
		Candidates: make([]domain.RecruitmentCandidate, 0, len(decoded.Candidates)),
	}
	for _, candidate := range decoded.Candidates {
		converted, err := candidate.toDomain()
		if err != nil {
			return domain.RecruitmentAdapterInboxSyncResult{}, fmt.Errorf("%w: invalid inbox candidate", ErrAdapterFailed)
		}
		result.Candidates = append(result.Candidates, converted)
	}
	if result.Status == "" {
		result.Status = domain.RecruitmentInboxStatusSynced
	}
	if result.Mailbox == "" {
		result.Mailbox = inboxMailbox(req)
	}
	if result.Folder == "" {
		result.Folder = inboxFolder(req)
	}
	if result.Candidates == nil {
		result.Candidates = []domain.RecruitmentCandidate{}
	}
	return result, nil
}

func (c qtrecuritInboxCandidate) toDomain() (domain.RecruitmentCandidate, error) {
	receivedAt, err := parseQtrecuritTime(c.ReceivedAt)
	if err != nil {
		return domain.RecruitmentCandidate{}, err
	}
	updatedAt, err := parseQtrecuritTime(c.UpdatedAt)
	if err != nil {
		return domain.RecruitmentCandidate{}, err
	}
	if receivedAt.IsZero() {
		receivedAt = updatedAt
	}
	name := displayName(c.Name, c.Subject, c.Email)
	subject := safeDisplayText(c.Subject)
	body := safeDisplayText(c.Body)
	resumeAttachments := resumeAttachmentsToDomain(firstAttachmentList(c.ResumeAttachments, c.Attachments, c.Message.Attachments, c.Raw.Attachments))
	return domain.RecruitmentCandidate{
		ID:                c.ID,
		Name:              name,
		Email:             c.Email,
		Subject:           subject,
		Body:              body,
		Position:          c.Position,
		Stage:             c.Stage,
		Status:            c.Status,
		HasResume:         c.HasResume || len(resumeAttachments) > 0,
		HasCoverLetter:    c.HasCoverLetter,
		ResumeAttachments: resumeAttachments,
		SourceMessageID:   strings.TrimSpace(firstNonEmpty(c.SourceMessageID, c.MessageID)),
		ReceivedAt:        receivedAt,
		UpdatedAt:         updatedAt,
	}, nil
}

func firstAttachmentList(values ...[]qtrecuritResumeAttachment) []qtrecuritResumeAttachment {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func resumeAttachmentsToDomain(attachments []qtrecuritResumeAttachment) []domain.RecruitmentResumeAttachment {
	if len(attachments) == 0 {
		return nil
	}
	result := make([]domain.RecruitmentResumeAttachment, 0, len(attachments))
	for _, attachment := range attachments {
		fileName := safeDisplayText(firstNonEmpty(attachment.FileName, attachment.Name, attachment.Filename))
		if fileName == "" || !isResumeFileName(fileName) {
			continue
		}
		result = append(result, domain.RecruitmentResumeAttachment{
			FileName:    fileName,
			ContentType: safeDisplayText(firstNonEmpty(attachment.ContentType, attachment.MIMEType)),
			URL:         strings.TrimSpace(firstNonEmpty(attachment.URL, attachment.DownloadURL, attachment.PreviewURL)),
			SizeBytes:   firstPositive(attachment.SizeBytes, attachment.Size),
			SourceID:    strings.TrimSpace(firstNonEmpty(attachment.AttachmentID, attachment.ID)),
		})
	}
	return result
}

func isResumeFileName(fileName string) bool {
	lower := strings.ToLower(strings.TrimSpace(fileName))
	return strings.HasSuffix(lower, ".pdf") || strings.HasSuffix(lower, ".doc") || strings.HasSuffix(lower, ".docx")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstPositive(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func displayName(name string, subject string, email string) string {
	trimmed := strings.TrimSpace(safeDisplayText(name))
	if trimmed != "" {
		return trimmed
	}
	if fromSubject := nameFromBracketedSubject(subject); fromSubject != "" {
		return fromSubject
	}
	return email
}

func safeDisplayText(value string) string {
	if containsReplacementCharacter(value) {
		return ""
	}
	return strings.TrimSpace(value)
}

func containsReplacementCharacter(value string) bool {
	return strings.ContainsRune(value, '\uFFFD')
}

func nameFromBracketedSubject(subject string) string {
	matches := regexp.MustCompile(`\[([^\]]+)\]`).FindAllStringSubmatch(subject, -1)
	if len(matches) < 2 || len(matches[1]) < 2 {
		return ""
	}
	return strings.TrimSpace(safeDisplayText(matches[1][1]))
}

func parseQtrecuritTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse("2006-01-02 15:04", value); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("invalid time: %s", value)
}

func (a *CLIAdapter) run(parent context.Context, args []string) (string, string, error) {
	commandArgs := append([]string{}, a.ArgsPrefix...)
	commandArgs = append(commandArgs, args...)
	return a.runRaw(parent, a.Binary, commandArgs)
}

func (a *CLIAdapter) runRaw(parent context.Context, binary string, args []string) (string, string, error) {
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	runner := a.Runner
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	stdout, stderr, err := runner.Run(ctx, binary, args)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", "", ErrAdapterTimeout
	}
	if err != nil {
		return stdout, stderr, fmt.Errorf("%w: %v", ErrAdapterFailed, err)
	}
	return stdout, stderr, nil
}

func BuildReportArgs(req domain.RecruitmentReportRequest) []string {
	args := []string{"report"}
	if req.Days != nil {
		args = append(args, "--days", strconv.Itoa(*req.Days))
	}
	if req.Start != nil {
		args = append(args, "--start", *req.Start)
	}
	if req.End != nil {
		args = append(args, "--end", *req.End)
	}
	return args
}

func BuildInboxSyncArgs(req domain.RecruitmentInboxSyncRequest) []string {
	args := []string{"inbox", "sync", "--mailbox", inboxMailbox(req), "--folder", inboxFolder(req), "--format", "json"}
	if req.PageSize != nil {
		args = append(args, "--page-size", strconv.Itoa(*req.PageSize))
	}
	if req.IsDryRun(false) {
		args = append(args, "--dry-run")
	}
	return args
}

func BuildResumeArgs(candidate domain.RecruitmentCandidate, attachment domain.RecruitmentResumeAttachment) []string {
	return []string{
		"inbox", "resume",
		"--mailbox", defaultRecruitmentMailbox,
		"--message-id", candidate.SourceMessageID,
		"--attachment-id", attachment.SourceID,
		"--file-name", attachment.FileName,
		"--format", "json",
	}
}

func BuildActionArgs(candidate domain.RecruitmentCandidate, req domain.RecruitmentActionRequest) []string {
	args := []string{"access"}
	switch req.Action {
	case domain.RecruitmentActionSendSurvey:
		args = append(args, "survey", "--to", candidate.Email, "--name", candidate.Name)
		if link, ok := req.Params["link"].(string); ok && link != "" {
			args = append(args, "--link", link)
		}
	case domain.RecruitmentActionSendTrainingInvite:
		args = append(args, "invite", "--to", candidate.Email, "--name", candidate.Name)
		if qr, ok := req.Params["qr"].(string); ok && qr != "" {
			args = append(args, "--qr", qr)
		}
	case domain.RecruitmentActionSendExam:
		args = append(args, "exam", "--to", candidate.Email, "--name", candidate.Name)
	case domain.RecruitmentActionCreateInterviewNotice:
		args = append(
			args,
			"interview",
			"--to", candidate.Email,
			"--name", candidate.Name,
			"--position", stringParam(req.Params, "position"),
			"--time", stringParam(req.Params, "time"),
		)
	}
	if req.IsDryRun(false) {
		args = append(args, "--dry-run")
	}
	return args
}

func stringParam(params map[string]any, key string) string {
	value, _ := params[key].(string)
	return value
}

func inboxMailbox(req domain.RecruitmentInboxSyncRequest) string {
	if strings.TrimSpace(req.Mailbox) == "" {
		return defaultRecruitmentMailbox
	}
	return req.Mailbox
}

func inboxFolder(req domain.RecruitmentInboxSyncRequest) string {
	if strings.TrimSpace(req.Folder) == "" {
		return "INBOX"
	}
	return req.Folder
}

func actionStatus(action string, dryRun bool, stdout string, stderr string) string {
	if dryRun || strings.Contains(strings.ToLower(stdout), "dry-run") {
		return domain.RecruitmentActionStatusDryRun
	}
	if action == domain.RecruitmentActionCreateInterviewNotice || strings.Contains(stdout, "草稿") {
		return domain.RecruitmentActionStatusDraft
	}
	if strings.Contains(stdout, "已发送") || strings.Contains(strings.ToLower(stdout), "sent") {
		return domain.RecruitmentActionStatusSent
	}
	if strings.Contains(stderr, "草稿") {
		return domain.RecruitmentActionStatusDraft
	}
	return domain.RecruitmentActionStatusSent
}

func actionMessage(action string, status string) string {
	switch status {
	case domain.RecruitmentActionStatusDryRun:
		return "已完成 dry_run 预览，未发送邮件"
	case domain.RecruitmentActionStatusDraft:
		if action == domain.RecruitmentActionCreateInterviewNotice {
			return "面试通知草稿已生成"
		}
		return "邮件草稿已生成"
	case domain.RecruitmentActionStatusSent:
		switch action {
		case domain.RecruitmentActionSendSurvey:
			return "问卷邮件已发送"
		case domain.RecruitmentActionSendTrainingInvite:
			return "实训邀约已发送"
		case domain.RecruitmentActionSendExam:
			return "笔试邀请已发送"
		}
		return "招聘动作已完成"
	default:
		return "招聘动作未完成"
	}
}

func DefaultBinary() string {
	if binary := os.Getenv("QTRECURIT_BIN"); binary != "" {
		return binary
	}
	if os.Getenv("QTRECURIT_MANIFEST_PATH") != "" {
		return "cargo"
	}
	return "qtrecurit"
}

func DefaultArgsPrefix() []string {
	manifest := strings.TrimSpace(os.Getenv("QTRECURIT_MANIFEST_PATH"))
	if manifest == "" {
		return nil
	}
	return []string{"run", "--quiet", "--manifest-path", manifest, "--"}
}

func DefaultTimeout() time.Duration {
	if value := os.Getenv("QTRECURIT_TIMEOUT_SECONDS"); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return 60 * time.Second
}

func safeStatusLine(value string) string {
	line := strings.TrimSpace(strings.Split(value, "\n")[0])
	if len([]rune(line)) > 80 {
		return string([]rune(line)[:80])
	}
	return line
}
