package recruitment

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
)

type fakeCommandRunner struct {
	stdout  string
	stderr  string
	err     error
	args    []string
	outputs map[string]fakeCommandOutput
}

type fakeCommandOutput struct {
	stdout string
	stderr string
	err    error
}

func (f *fakeCommandRunner) Run(ctx context.Context, binary string, args []string) (string, string, error) {
	f.args = args
	if f.outputs != nil {
		key := binary + " " + strings.Join(args, " ")
		if output, ok := f.outputs[key]; ok {
			return output.stdout, output.stderr, output.err
		}
	}
	return f.stdout, f.stderr, f.err
}

func TestBuildReportArgsUsesWhitelistedStructuredArgs(t *testing.T) {
	days := 30
	dryRun := true
	args := BuildReportArgs(domain.RecruitmentReportRequest{Days: &days, DryRun: &dryRun})
	want := []string{"report", "--days", "30"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestBuildInboxSyncArgsUsesWhitelistedStructuredArgs(t *testing.T) {
	pageSize := 25
	dryRun := true
	args := BuildInboxSyncArgs(domain.RecruitmentInboxSyncRequest{
		Mailbox:  "hr@quanttide.com",
		Folder:   "INBOX",
		PageSize: &pageSize,
		DryRun:   &dryRun,
	})
	want := []string{"inbox", "sync", "--mailbox", "hr@quanttide.com", "--folder", "INBOX", "--format", "json", "--page-size", "25", "--dry-run"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestBuildResumeArgsUsesStructuredCandidateAttachmentIDs(t *testing.T) {
	candidate := domain.RecruitmentCandidate{
		SourceMessageID: "message_001",
	}
	attachment := domain.RecruitmentResumeAttachment{
		FileName: "张三-后端开发简历.pdf",
		SourceID: "attachment_001",
	}

	got := BuildResumeArgs(candidate, attachment)
	want := []string{
		"inbox", "resume",
		"--mailbox", "hr@quanttide.com",
		"--message-id", "message_001",
		"--attachment-id", "attachment_001",
		"--file-name", "张三-后端开发简历.pdf",
		"--format", "json",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestSyncInboxParsesCLIJSONWithDateOnlyUpdatedAt(t *testing.T) {
	runner := &fakeCommandRunner{
		stdout: `{"status":"synced","mailbox":"hr@quanttide.com","folder":"INBOX","scanned":1,"imported":1,"candidates":[{"id":"cand_1","name":"张三","email":"zhangsan@example.com","position":"数据工程师","stage":"new","status":"pending","has_resume":true,"has_cover_letter":true,"updated_at":"2026-09-04"}]}`,
	}
	adapter := NewCLIAdapter("qtrecurit", 0)
	adapter.Runner = runner

	result, err := adapter.SyncInbox(context.Background(), domain.RecruitmentInboxSyncRequest{})

	if err != nil {
		t.Fatalf("SyncInbox returned error: %v", err)
	}
	if result.Imported != 1 || len(result.Candidates) != 1 {
		t.Fatalf("unexpected inbox result: %+v", result)
	}
	if got := result.Candidates[0].UpdatedAt.Format("2006-01-02"); got != "2026-09-04" {
		t.Fatalf("updated_at = %s", got)
	}
}

func TestSyncInboxParsesCLIJSONWithMinuteUpdatedAt(t *testing.T) {
	runner := &fakeCommandRunner{
		stdout: `{"status":"synced","mailbox":"hr@quanttide.com","folder":"INBOX","scanned":1,"imported":1,"candidates":[{"id":"cand_1","name":"张三","email":"zhangsan@example.com","position":"产品经理","stage":"new","status":"pending","has_resume":true,"has_cover_letter":true,"updated_at":"2026-09-04 16:26"}]}`,
	}
	adapter := NewCLIAdapter("qtrecurit", 0)
	adapter.Runner = runner

	result, err := adapter.SyncInbox(context.Background(), domain.RecruitmentInboxSyncRequest{})

	if err != nil {
		t.Fatalf("SyncInbox returned error: %v", err)
	}
	if got := result.Candidates[0].UpdatedAt.Format("2006-01-02 15:04"); got != "2026-09-04 16:26" {
		t.Fatalf("updated_at = %s", got)
	}
}

func TestSyncInboxParsesCLIJSONWithReceivedAt(t *testing.T) {
	runner := &fakeCommandRunner{
		stdout: `{"status":"synced","mailbox":"hr@quanttide.com","folder":"INBOX","scanned":1,"imported":1,"candidates":[{"id":"cand_1","name":"张三","email":"zhangsan@example.com","position":"产品经理","stage":"new","status":"pending","has_resume":true,"has_cover_letter":true,"received_at":"2026-09-04 16:26","updated_at":"2026-09-05 10:00"}]}`,
	}
	adapter := NewCLIAdapter("qtrecurit", 0)
	adapter.Runner = runner

	result, err := adapter.SyncInbox(context.Background(), domain.RecruitmentInboxSyncRequest{})

	if err != nil {
		t.Fatalf("SyncInbox returned error: %v", err)
	}
	if got := result.Candidates[0].ReceivedAt.Format("2006-01-02 15:04"); got != "2026-09-04 16:26" {
		t.Fatalf("received_at = %s", got)
	}
	if got := result.Candidates[0].UpdatedAt.Format("2006-01-02 15:04"); got != "2026-09-05 10:00" {
		t.Fatalf("updated_at = %s", got)
	}
}

func TestSyncInboxParsesCLIJSONWithSubjectAndBody(t *testing.T) {
	runner := &fakeCommandRunner{
		stdout: `{"status":"synced","mailbox":"hr@quanttide.com","folder":"INBOX","scanned":1,"imported":1,"candidates":[{"id":"cand_1","name":"张三","email":"zhangsan@example.com","subject":"应聘后端开发","body":"HR 您好，我想投递后端开发岗位，附件是我的简历。","position":"后端开发","stage":"new","status":"pending","has_resume":true,"has_cover_letter":true,"updated_at":"2026-09-04"}]}`,
	}
	adapter := NewCLIAdapter("qtrecurit", 0)
	adapter.Runner = runner

	result, err := adapter.SyncInbox(context.Background(), domain.RecruitmentInboxSyncRequest{})

	if err != nil {
		t.Fatalf("SyncInbox returned error: %v", err)
	}
	if got := result.Candidates[0].Subject; got != "应聘后端开发" {
		t.Fatalf("subject = %q", got)
	}
	if got := result.Candidates[0].Body; got != "HR 您好，我想投递后端开发岗位，附件是我的简历。" {
		t.Fatalf("body = %q", got)
	}
}

func TestSyncInboxParsesResumeAttachments(t *testing.T) {
	runner := &fakeCommandRunner{
		stdout: `{"status":"synced","mailbox":"hr@quanttide.com","folder":"INBOX","scanned":1,"imported":1,"candidates":[{"id":"cand_1","name":"张三","email":"zhangsan@example.com","subject":"应聘后端开发","position":"后端开发","stage":"new","status":"pending","has_resume":true,"has_cover_letter":true,"resume_attachments":[{"file_name":"张三-后端开发简历.pdf","content_type":"application/pdf","url":"https://files.example.test/resumes/cand_1.pdf","size_bytes":245760}],"updated_at":"2026-09-04"}]}`,
	}
	adapter := NewCLIAdapter("qtrecurit", 0)
	adapter.Runner = runner

	result, err := adapter.SyncInbox(context.Background(), domain.RecruitmentInboxSyncRequest{})

	if err != nil {
		t.Fatalf("SyncInbox returned error: %v", err)
	}
	attachments := result.Candidates[0].ResumeAttachments
	if len(attachments) != 1 {
		t.Fatalf("attachments = %+v, want one resume attachment", attachments)
	}
	if got := attachments[0].FileName; got != "张三-后端开发简历.pdf" {
		t.Fatalf("file_name = %q", got)
	}
	if got := attachments[0].URL; got != "https://files.example.test/resumes/cand_1.pdf" {
		t.Fatalf("url = %q", got)
	}
}

func TestSyncInboxParsesProviderAttachmentAliases(t *testing.T) {
	runner := &fakeCommandRunner{
		stdout: `{"status":"synced","mailbox":"hr@quanttide.com","folder":"INBOX","scanned":1,"imported":1,"candidates":[{"id":"cand_1","name":"王五","email":"wangwu@example.com","subject":"应聘产品经理","position":"产品经理","stage":"new","status":"pending","has_resume":true,"has_cover_letter":true,"message_id":"message_alias","attachments":[{"id":"att_resume_001","filename":"王五-产品经理简历.docx","mime":"application/vnd.openxmlformats-officedocument.wordprocessingml.document","download_url":"https://files.example.test/resumes/cand_1.docx","size":245760}],"updated_at":"2026-09-04"}]}`,
	}
	adapter := NewCLIAdapter("qtrecurit", 0)
	adapter.Runner = runner

	result, err := adapter.SyncInbox(context.Background(), domain.RecruitmentInboxSyncRequest{})

	if err != nil {
		t.Fatalf("SyncInbox returned error: %v", err)
	}
	candidate := result.Candidates[0]
	if got := candidate.SourceMessageID; got != "message_alias" {
		t.Fatalf("source_message_id = %q", got)
	}
	attachments := candidate.ResumeAttachments
	if len(attachments) != 1 {
		t.Fatalf("attachments = %+v, want one resume attachment", attachments)
	}
	if got := attachments[0].FileName; got != "王五-产品经理简历.docx" {
		t.Fatalf("file_name = %q", got)
	}
	if got := attachments[0].SourceID; got != "att_resume_001" {
		t.Fatalf("source id = %q", got)
	}
}

func TestSyncInboxParsesNestedRawMessageAttachments(t *testing.T) {
	runner := &fakeCommandRunner{
		stdout: `{"status":"synced","mailbox":"hr@quanttide.com","folder":"INBOX","scanned":1,"imported":1,"candidates":[{"id":"cand_1","name":"赵六","email":"zhaoliu@example.com","subject":"应聘前端开发","position":"前端开发","stage":"new","status":"pending","has_resume":true,"has_cover_letter":true,"source_message_id":"message_nested","message":{"attachments":[{"attachment_id":"att_nested_001","file_name":"赵六-前端开发简历.pdf","content_type":"application/pdf","size_bytes":123456}]},"updated_at":"2026-09-04"}]}`,
	}
	adapter := NewCLIAdapter("qtrecurit", 0)
	adapter.Runner = runner

	result, err := adapter.SyncInbox(context.Background(), domain.RecruitmentInboxSyncRequest{})

	if err != nil {
		t.Fatalf("SyncInbox returned error: %v", err)
	}
	attachments := result.Candidates[0].ResumeAttachments
	if len(attachments) != 1 {
		t.Fatalf("attachments = %+v, want one resume attachment", attachments)
	}
	if got := attachments[0].FileName; got != "赵六-前端开发简历.pdf" {
		t.Fatalf("file_name = %q", got)
	}
	if got := attachments[0].SourceID; got != "att_nested_001" {
		t.Fatalf("source id = %q", got)
	}
}

func TestSyncInboxSanitizesGarbledDisplayFields(t *testing.T) {
	runner := &fakeCommandRunner{
		stdout: `{"status":"synced","mailbox":"hr@quanttide.com","folder":"INBOX","scanned":1,"imported":1,"candidates":[{"id":"cand_1","name":"����","email":"candidate@example.com","subject":"[产品经理] - [王五] - [浙江越秀外国语学院] - [3个月以上]","body":"您好，希望投递产品经理岗位。","stage":"new","status":"pending","has_resume":true,"has_cover_letter":true,"updated_at":"2026-09-04"}]}`,
	}
	adapter := NewCLIAdapter("qtrecurit", 0)
	adapter.Runner = runner

	result, err := adapter.SyncInbox(context.Background(), domain.RecruitmentInboxSyncRequest{})

	if err != nil {
		t.Fatalf("SyncInbox returned error: %v", err)
	}
	if got := result.Candidates[0].Name; got != "王五" {
		t.Fatalf("name = %q, want subject fallback", got)
	}
	if strings.Contains(result.Candidates[0].Name, "�") {
		t.Fatalf("name should not expose replacement characters: %+v", result.Candidates[0])
	}
}

func TestCLIAdapterPrependsConfiguredArgsWithoutShellJoin(t *testing.T) {
	runner := &fakeCommandRunner{
		stdout: `{"status":"synced","mailbox":"hr@quanttide.com","folder":"INBOX","scanned":0,"imported":0,"candidates":[]}`,
	}
	adapter := NewCLIAdapter("cargo", 0)
	adapter.Runner = runner
	adapter.ArgsPrefix = []string{"run", "--quiet", "--manifest-path", "D:/qtrecurit/Cargo.toml", "--"}

	_, err := adapter.SyncInbox(context.Background(), domain.RecruitmentInboxSyncRequest{})

	if err != nil {
		t.Fatalf("SyncInbox returned error: %v", err)
	}
	wantPrefix := []string{"run", "--quiet", "--manifest-path", "D:/qtrecurit/Cargo.toml", "--", "inbox", "sync"}
	if len(runner.args) < len(wantPrefix) || !reflect.DeepEqual(runner.args[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("args = %#v, want prefix %#v", runner.args, wantPrefix)
	}
}

func TestBuildActionArgsUsesWhitelistedStructuredArgs(t *testing.T) {
	dryRun := true
	candidate := domain.RecruitmentCandidate{
		Email: "zhangsan@example.com",
		Name:  "张三",
	}
	cases := []struct {
		name string
		req  domain.RecruitmentActionRequest
		want []string
	}{
		{
			name: "survey",
			req: domain.RecruitmentActionRequest{
				Action: domain.RecruitmentActionSendSurvey,
				DryRun: &dryRun,
				Params: map[string]any{"link": "https://example.com/survey"},
			},
			want: []string{"access", "survey", "--to", "zhangsan@example.com", "--name", "张三", "--link", "https://example.com/survey", "--dry-run"},
		},
		{
			name: "invite",
			req: domain.RecruitmentActionRequest{
				Action: domain.RecruitmentActionSendTrainingInvite,
				DryRun: &dryRun,
				Params: map[string]any{"qr": "oss://qtcloud-human-studio/assets/training-group.png", "ignored": "--danger"},
			},
			want: []string{"access", "invite", "--to", "zhangsan@example.com", "--name", "张三", "--qr", "oss://qtcloud-human-studio/assets/training-group.png", "--dry-run"},
		},
		{
			name: "exam",
			req: domain.RecruitmentActionRequest{
				Action: domain.RecruitmentActionSendExam,
				DryRun: &dryRun,
				Params: map[string]any{},
			},
			want: []string{"access", "exam", "--to", "zhangsan@example.com", "--name", "张三", "--dry-run"},
		},
		{
			name: "interview",
			req: domain.RecruitmentActionRequest{
				Action: domain.RecruitmentActionCreateInterviewNotice,
				DryRun: &dryRun,
				Params: map[string]any{"position": "数据工程师", "time": "2026-09-10 10:00"},
			},
			want: []string{"access", "interview", "--to", "zhangsan@example.com", "--name", "张三", "--position", "数据工程师", "--time", "2026-09-10 10:00", "--dry-run"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildActionArgs(candidate, tc.req)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("args = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestCheckProviderStatusReportsReadyWhenCLIAndMailboxWork(t *testing.T) {
	runner := &fakeCommandRunner{outputs: map[string]fakeCommandOutput{
		"qtrecurit --version": {
			stdout: "qtrecurit 0.1.0\n",
		},
		"lark-cli mail user_mailboxes profile --mailbox hr@quanttide.com --format json": {
			stdout: `{"data":{"email":"hr@quanttide.com"}}`,
		},
	}}
	adapter := NewCLIAdapter("qtrecurit", 0)
	adapter.Runner = runner

	status := adapter.CheckProviderStatus(context.Background())

	if !status.Ready || status.Status != "ready" || status.Mailbox != "hr@quanttide.com" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if len(status.Components) != 2 || status.Components[0].Status != "ok" || status.Components[1].Status != "ok" {
		t.Fatalf("unexpected components: %+v", status.Components)
	}
}

func TestCheckProviderStatusReportsBlockedWhenMailboxFailsWithoutLeakingDetails(t *testing.T) {
	runner := &fakeCommandRunner{outputs: map[string]fakeCommandOutput{
		"qtrecurit --version": {
			stdout: "qtrecurit 0.1.0\n",
		},
		"lark-cli mail user_mailboxes profile --mailbox hr@quanttide.com --format json": {
			stderr: "token=secret mail body stack trace",
			err:    ErrAdapterFailed,
		},
	}}
	adapter := NewCLIAdapter("qtrecurit", 0)
	adapter.Runner = runner

	status := adapter.CheckProviderStatus(context.Background())

	if status.Ready || status.Status != "blocked" {
		t.Fatalf("unexpected status: %+v", status)
	}
	serialized := strings.ToLower(status.Message)
	for _, component := range status.Components {
		serialized += " " + strings.ToLower(component.Message)
	}
	for _, leak := range []string{"token=secret", "mail body", "stack trace"} {
		if strings.Contains(serialized, leak) {
			t.Fatalf("status leaked %q: %+v", leak, status)
		}
	}
}
