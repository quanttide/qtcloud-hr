package recruitment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
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
	Binary  string
	Timeout time.Duration
	Runner  CommandRunner
}

func NewCLIAdapter(binary string, timeout time.Duration) *CLIAdapter {
	if binary == "" {
		binary = "qtrecurit"
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &CLIAdapter{
		Binary:  binary,
		Timeout: timeout,
		Runner:  ExecCommandRunner{},
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

func (a *CLIAdapter) run(parent context.Context, args []string) (string, string, error) {
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
	stdout, stderr, err := runner.Run(ctx, a.Binary, args)
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
	return "qtrecurit"
}

func DefaultTimeout() time.Duration {
	if value := os.Getenv("QTRECURIT_TIMEOUT_SECONDS"); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return 60 * time.Second
}
