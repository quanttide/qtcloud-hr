package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
)

func TestDefaultRecruitmentStoreStartsEmpty(t *testing.T) {
	store := DefaultRecruitmentStore()

	if got := store.ListCandidates(); len(got) != 0 {
		t.Fatalf("expected no seeded recruitment mock candidates, got %+v", got)
	}
}

func TestListCandidatesSortsByLatestReceivedAt(t *testing.T) {
	store := NewRecruitmentStore([]domain.RecruitmentCandidate{
		{
			ID:         "cand_old",
			Name:       "旧候选人",
			Email:      "old@example.com",
			Stage:      "new",
			Status:     "pending",
			ReceivedAt: time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:         "cand_new",
			Name:       "新候选人",
			Email:      "new@example.com",
			Stage:      "new",
			Status:     "pending",
			ReceivedAt: time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC),
		},
	})

	candidates := store.ListCandidates()

	if got := candidates[0].ID; got != "cand_new" {
		t.Fatalf("first candidate = %s, want cand_new", got)
	}
}

func TestListCandidatesFallsBackToLatestUpdate(t *testing.T) {
	store := NewRecruitmentStore([]domain.RecruitmentCandidate{
		{ID: "cand_old", Name: "旧候选人", Email: "old@example.com", Stage: "new", Status: "pending", UpdatedAt: time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC)},
		{ID: "cand_new", Name: "新候选人", Email: "new@example.com", Stage: "new", Status: "pending", UpdatedAt: time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)},
	})

	candidates := store.ListCandidates()

	if got := candidates[0].ID; got != "cand_new" {
		t.Fatalf("first candidate = %s, want cand_new", got)
	}
}

func TestListCandidatesKeepsInputOrderWhenReceivedAtMatches(t *testing.T) {
	receivedAt := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	store := NewRecruitmentStore([]domain.RecruitmentCandidate{
		{ID: "cand_first", Name: "第一封", Email: "first@example.com", Stage: "new", Status: "pending", ReceivedAt: receivedAt, UpdatedAt: receivedAt},
		{ID: "cand_second", Name: "第二封", Email: "second@example.com", Stage: "new", Status: "pending", ReceivedAt: receivedAt, UpdatedAt: receivedAt},
	})

	candidates := store.ListCandidates()

	if got := candidates[0].ID; got != "cand_first" {
		t.Fatalf("first candidate = %s, want cand_first", got)
	}
}

func TestPersistentRecruitmentStoreLoadsCandidatesAcrossInstances(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state", "candidates.json")
	first, err := NewPersistentRecruitmentStore(statePath)
	if err != nil {
		t.Fatalf("create first store: %v", err)
	}
	first.ReplaceCandidates([]domain.RecruitmentCandidate{{
		ID:     "cand_persisted",
		Name:   "李四",
		Email:  "lisi@example.com",
		Stage:  "new",
		Status: "pending",
	}})

	second, err := NewPersistentRecruitmentStore(statePath)
	if err != nil {
		t.Fatalf("create second store: %v", err)
	}
	got, ok := second.GetCandidate("cand_persisted")
	if !ok {
		t.Fatal("persisted candidate was not loaded")
	}
	if got.Email != "lisi@example.com" {
		t.Fatalf("candidate email = %q, want lisi@example.com", got.Email)
	}
}

func TestPersistentRecruitmentStoreKeepsResumeAttachmentSourceIDs(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state", "candidates.json")
	first, err := NewPersistentRecruitmentStore(statePath)
	if err != nil {
		t.Fatalf("create first store: %v", err)
	}
	first.ReplaceCandidates([]domain.RecruitmentCandidate{{
		ID:              "cand_resume",
		Name:            "简历候选人",
		Email:           "resume@example.com",
		Stage:           "new",
		Status:          "pending",
		SourceMessageID: "message_001",
		ResumeAttachments: []domain.RecruitmentResumeAttachment{{
			FileName:    "resume.pdf",
			ContentType: "application/pdf",
			SourceID:    "attachment_001",
		}},
	}})

	second, err := NewPersistentRecruitmentStore(statePath)
	if err != nil {
		t.Fatalf("create second store: %v", err)
	}
	got, ok := second.GetCandidate("cand_resume")
	if !ok {
		t.Fatal("persisted resume candidate was not loaded")
	}
	if len(got.ResumeAttachments) != 1 {
		t.Fatalf("resume attachments = %+v, want one attachment", got.ResumeAttachments)
	}
	if got.ResumeAttachments[0].SourceID != "attachment_001" {
		t.Fatalf(
			"resume attachment source id = %q, want attachment_001",
			got.ResumeAttachments[0].SourceID,
		)
	}
}

func TestPersistentRecruitmentStoreRefreshesExternalChanges(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state", "candidates.json")
	first, err := NewPersistentRecruitmentStore(statePath)
	if err != nil {
		t.Fatalf("create first store: %v", err)
	}
	second, err := NewPersistentRecruitmentStore(statePath)
	if err != nil {
		t.Fatalf("create second store: %v", err)
	}

	first.ReplaceCandidates([]domain.RecruitmentCandidate{{
		ID:     "cand_new",
		Name:   "王五",
		Email:  "wangwu@example.com",
		Stage:  "new",
		Status: "pending",
	}})

	got, ok := second.GetCandidate("cand_new")
	if !ok {
		t.Fatal("second store did not refresh the latest candidate snapshot")
	}
	if got.Email != "wangwu@example.com" {
		t.Fatalf("candidate email = %q, want wangwu@example.com", got.Email)
	}
}

func TestReplaceCandidatesDropsStaleItems(t *testing.T) {
	store := NewRecruitmentStore([]domain.RecruitmentCandidate{
		{ID: "cand_old", Name: "旧候选人", Email: "old@example.com", Stage: "new", Status: "pending"},
	})

	store.ReplaceCandidates([]domain.RecruitmentCandidate{
		{ID: "cand_new", Name: "新候选人", Email: "new@example.com", Stage: "new", Status: "pending"},
	})

	if _, ok := store.GetCandidate("cand_old"); ok {
		t.Fatalf("stale candidate should be removed after inbox sync")
	}
	if candidate, ok := store.GetCandidate("cand_new"); !ok || candidate.Name != "新候选人" {
		t.Fatalf("new candidate missing after replace: %+v", candidate)
	}
}

func TestReplaceCandidatesPreservesWorkflowStateForExistingCandidates(t *testing.T) {
	store := NewRecruitmentStore([]domain.RecruitmentCandidate{
		{
			ID:         "cand_existing",
			Name:       "旧姓名",
			Email:      "old@example.com",
			Stage:      "survey_sent",
			Status:     "passed",
			LastAction: domain.RecruitmentActionSendSurvey,
		},
	})

	store.ReplaceCandidates([]domain.RecruitmentCandidate{
		{
			ID:      "cand_existing",
			Name:    "新姓名",
			Email:   "new@example.com",
			Subject: "更新后的主题",
			Stage:   "new",
			Status:  "pending",
		},
	})

	candidate, ok := store.GetCandidate("cand_existing")
	if !ok {
		t.Fatal("existing candidate missing after replacement")
	}
	if candidate.Name != "新姓名" || candidate.Subject != "更新后的主题" {
		t.Fatalf("latest source fields were not applied: %+v", candidate)
	}
	if candidate.Status != "passed" {
		t.Fatalf("manual status = %q, want passed", candidate.Status)
	}
	if candidate.Stage != "survey_sent" {
		t.Fatalf("workflow stage = %q, want survey_sent", candidate.Stage)
	}
	if candidate.LastAction != domain.RecruitmentActionSendSurvey {
		t.Fatalf("last action = %q, want %q", candidate.LastAction, domain.RecruitmentActionSendSurvey)
	}
}

func TestRecordActionDoesNotMoveLegacyCandidateAheadOfNewerMail(t *testing.T) {
	store := NewRecruitmentStore([]domain.RecruitmentCandidate{
		{ID: "cand_old", Name: "旧候选人", Email: "old@example.com", Stage: "new", Status: "pending", UpdatedAt: time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC)},
		{ID: "cand_new", Name: "新候选人", Email: "new@example.com", Stage: "new", Status: "pending", UpdatedAt: time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)},
	})

	store.RecordAction("cand_old", domain.RecruitmentActionSendSurvey, "survey_sent")
	candidates := store.ListCandidates()

	if got := candidates[0].ID; got != "cand_new" {
		t.Fatalf("first candidate = %s, want cand_new", got)
	}
}
