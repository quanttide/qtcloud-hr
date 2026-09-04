package recruitment

import (
	"reflect"
	"testing"

	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
)

func TestBuildReportArgsUsesWhitelistedStructuredArgs(t *testing.T) {
	days := 30
	dryRun := true
	args := BuildReportArgs(domain.RecruitmentReportRequest{Days: &days, DryRun: &dryRun})
	want := []string{"report", "--days", "30"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
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
