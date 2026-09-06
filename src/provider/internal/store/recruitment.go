package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
)

type RecruitmentStore struct {
	mu              sync.RWMutex
	candidates      map[string]*domain.RecruitmentCandidate
	order           []string
	persistencePath string
}

func NewRecruitmentStore(initial []domain.RecruitmentCandidate) *RecruitmentStore {
	items := make(map[string]*domain.RecruitmentCandidate, len(initial))
	order := make([]string, 0, len(initial))
	for _, candidate := range initial {
		clone := candidate
		if clone.ID == "" {
			continue
		}
		if clone.UpdatedAt.IsZero() {
			clone.UpdatedAt = time.Now().UTC()
		}
		if clone.ReceivedAt.IsZero() {
			clone.ReceivedAt = clone.UpdatedAt
		}
		if _, exists := items[clone.ID]; !exists {
			order = append(order, clone.ID)
		}
		items[clone.ID] = &clone
	}
	return &RecruitmentStore{candidates: items, order: order}
}

func NewPersistentRecruitmentStore(path string) (*RecruitmentStore, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return NewRecruitmentStore(nil), nil
	}

	store := NewRecruitmentStore(nil)
	store.persistencePath = filepath.Clean(cleanPath)
	data, err := os.ReadFile(store.persistencePath)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, err
	}

	var candidates []domain.RecruitmentCandidate
	if err := json.Unmarshal(data, &candidates); err != nil {
		return nil, err
	}
	loaded := NewRecruitmentStore(candidates)
	loaded.persistencePath = store.persistencePath
	return loaded, nil
}

func DefaultRecruitmentStore() *RecruitmentStore {
	return NewRecruitmentStore(nil)
}

func (s *RecruitmentStore) ListCandidates() []domain.RecruitmentCandidate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.RecruitmentCandidate, 0, len(s.candidates))
	seen := make(map[string]bool, len(s.candidates))
	for _, id := range s.order {
		candidate, ok := s.candidates[id]
		if !ok {
			continue
		}
		out = append(out, *candidate)
		seen[id] = true
	}
	for id, candidate := range s.candidates {
		if seen[id] {
			continue
		}
		out = append(out, *candidate)
	}
	sort.SliceStable(out, func(i, j int) bool {
		iReceivedAt := candidateReceivedAt(out[i])
		jReceivedAt := candidateReceivedAt(out[j])
		return iReceivedAt.After(jReceivedAt)
	})
	return out
}

func (s *RecruitmentStore) UpsertCandidates(candidates []domain.RecruitmentCandidate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, candidate := range candidates {
		if candidate.ID == "" {
			continue
		}
		clone := candidate
		if clone.UpdatedAt.IsZero() {
			clone.UpdatedAt = time.Now().UTC()
		}
		if clone.ReceivedAt.IsZero() {
			clone.ReceivedAt = clone.UpdatedAt
		}
		if _, exists := s.candidates[clone.ID]; !exists {
			s.order = append(s.order, clone.ID)
		}
		s.candidates[clone.ID] = &clone
	}
	s.persistLocked()
}

func (s *RecruitmentStore) ReplaceCandidates(candidates []domain.RecruitmentCandidate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make(map[string]*domain.RecruitmentCandidate, len(candidates))
	order := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ID == "" {
			continue
		}
		clone := candidate
		if clone.UpdatedAt.IsZero() {
			clone.UpdatedAt = time.Now().UTC()
		}
		if clone.ReceivedAt.IsZero() {
			clone.ReceivedAt = clone.UpdatedAt
		}
		if _, exists := next[clone.ID]; !exists {
			order = append(order, clone.ID)
		}
		next[clone.ID] = &clone
	}
	s.candidates = next
	s.order = order
	s.persistLocked()
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

func (s *RecruitmentStore) UpdateCandidateStatus(candidateID string, status string) (domain.RecruitmentCandidate, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	candidate, ok := s.candidates[candidateID]
	if !ok {
		return domain.RecruitmentCandidate{}, false
	}
	candidate.Status = status
	if candidate.ReceivedAt.IsZero() {
		candidate.ReceivedAt = candidate.UpdatedAt
	}
	candidate.UpdatedAt = time.Now().UTC()
	s.persistLocked()
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
	if candidate.ReceivedAt.IsZero() {
		candidate.ReceivedAt = candidate.UpdatedAt
	}
	candidate.UpdatedAt = time.Now().UTC()
	s.persistLocked()
}

func (s *RecruitmentStore) persistLocked() {
	if s.persistencePath == "" {
		return
	}
	candidates := make([]domain.RecruitmentCandidate, 0, len(s.candidates))
	seen := make(map[string]bool, len(s.candidates))
	for _, id := range s.order {
		candidate, ok := s.candidates[id]
		if !ok {
			continue
		}
		candidates = append(candidates, *candidate)
		seen[id] = true
	}
	for id, candidate := range s.candidates {
		if seen[id] {
			continue
		}
		candidates = append(candidates, *candidate)
	}
	data, err := json.Marshal(candidates)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.persistencePath), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(s.persistencePath, data, 0o600)
}

func candidateReceivedAt(candidate domain.RecruitmentCandidate) time.Time {
	if !candidate.ReceivedAt.IsZero() {
		return candidate.ReceivedAt
	}
	return candidate.UpdatedAt
}
