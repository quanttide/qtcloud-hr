package store

import (
	"sync"
	"time"

	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
)

type RecruitmentStore struct {
	mu         sync.RWMutex
	candidates map[string]*domain.RecruitmentCandidate
}

func NewRecruitmentStore(initial []domain.RecruitmentCandidate) *RecruitmentStore {
	items := make(map[string]*domain.RecruitmentCandidate, len(initial))
	for _, candidate := range initial {
		clone := candidate
		if clone.UpdatedAt.IsZero() {
			clone.UpdatedAt = time.Now().UTC()
		}
		items[clone.ID] = &clone
	}
	return &RecruitmentStore{candidates: items}
}

func DefaultRecruitmentStore() *RecruitmentStore {
	return NewRecruitmentStore([]domain.RecruitmentCandidate{
		{
			ID:             "cand_001",
			Name:           "张明",
			Email:          "zhangming@gmail.com",
			Position:       "前端开发",
			Stage:          "new",
			Status:         "pending",
			HasResume:      true,
			HasCoverLetter: false,
		},
		{
			ID:             "cand_002",
			Name:           "王芳",
			Email:          "wangfang@qq.com",
			Position:       "产品经理",
			Stage:          "new",
			Status:         "pending",
			HasResume:      true,
			HasCoverLetter: true,
		},
	})
}

func (s *RecruitmentStore) ListCandidates() []domain.RecruitmentCandidate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.RecruitmentCandidate, 0, len(s.candidates))
	for _, candidate := range s.candidates {
		out = append(out, *candidate)
	}
	return out
}

func (s *RecruitmentStore) GetCandidate(id string) (domain.RecruitmentCandidate, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	candidate, ok := s.candidates[id]
	if !ok {
		return domain.RecruitmentCandidate{}, false
	}
	return *candidate, true
}

func (s *RecruitmentStore) RecordAction(candidateID string, action string, stage string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	candidate, ok := s.candidates[candidateID]
	if !ok {
		return
	}
	candidate.LastAction = action
	if stage != "" {
		candidate.Stage = stage
	}
	candidate.UpdatedAt = time.Now().UTC()
}
