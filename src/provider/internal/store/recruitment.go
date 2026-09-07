package store

import (
	"encoding/json"
	"errors"
	"log/slog"
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

type persistedRecruitmentState struct {
	Candidates                []domain.RecruitmentCandidate `json:"candidates"`
	ResumeAttachmentSourceIDs map[string][]string           `json:"resume_attachment_source_ids,omitempty"`
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
	if err := store.loadPersistedLocked(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return store, nil
}

func DefaultRecruitmentStore() *RecruitmentStore {
	return NewRecruitmentStore(nil)
}

func (s *RecruitmentStore) ListCandidates() []domain.RecruitmentCandidate {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshPersistedLocked()
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
	s.refreshPersistedLocked()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshPersistedLocked()
	candidate, ok := s.candidates[id]
	if !ok {
		return domain.RecruitmentCandidate{}, false
	}
	return *candidate, true
}

func (s *RecruitmentStore) UpdateCandidateStatus(candidateID string, status string) (domain.RecruitmentCandidate, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshPersistedLocked()
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
	s.refreshPersistedLocked()
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
	sourceIDs := make(map[string][]string)
	for _, candidate := range candidates {
		ids := make([]string, len(candidate.ResumeAttachments))
		hasSourceID := false
		for index, attachment := range candidate.ResumeAttachments {
			ids[index] = attachment.SourceID
			hasSourceID = hasSourceID || attachment.SourceID != ""
		}
		if hasSourceID {
			sourceIDs[candidate.ID] = ids
		}
	}
	data, err := json.Marshal(persistedRecruitmentState{
		Candidates:                candidates,
		ResumeAttachmentSourceIDs: sourceIDs,
	})
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.persistencePath), 0o700); err != nil {
		slog.Error("create recruitment state directory", "path", s.persistencePath, "error", err)
		return
	}
	temp, err := os.CreateTemp(filepath.Dir(s.persistencePath), ".recruitment-candidates-*.tmp")
	if err != nil {
		slog.Error("create recruitment state temp file", "path", s.persistencePath, "error", err)
		return
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		slog.Error("protect recruitment state temp file", "path", s.persistencePath, "error", err)
		_ = temp.Close()
		return
	}
	if _, err := temp.Write(data); err != nil {
		slog.Error("write recruitment state", "path", s.persistencePath, "error", err)
		_ = temp.Close()
		return
	}
	if err := temp.Sync(); err != nil {
		slog.Error("sync recruitment state", "path", s.persistencePath, "error", err)
		_ = temp.Close()
		return
	}
	if err := temp.Close(); err != nil {
		slog.Error("close recruitment state", "path", s.persistencePath, "error", err)
		return
	}
	if err := os.Rename(tempPath, s.persistencePath); err != nil {
		if removeErr := os.Remove(s.persistencePath); removeErr == nil {
			err = os.Rename(tempPath, s.persistencePath)
		}
	}
	if err != nil {
		slog.Error("replace recruitment state", "path", s.persistencePath, "error", err)
	}
}

func (s *RecruitmentStore) refreshPersistedLocked() {
	if s.persistencePath == "" {
		return
	}
	if err := s.loadPersistedLocked(); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("refresh recruitment state", "path", s.persistencePath, "error", err)
	}
}

func (s *RecruitmentStore) loadPersistedLocked() error {
	if s.persistencePath == "" {
		return nil
	}
	data, err := os.ReadFile(s.persistencePath)
	if err != nil {
		return err
	}
	var state persistedRecruitmentState
	if err := json.Unmarshal(data, &state); err != nil || state.Candidates == nil {
		var legacyCandidates []domain.RecruitmentCandidate
		if legacyErr := json.Unmarshal(data, &legacyCandidates); legacyErr != nil {
			return legacyErr
		}
		state.Candidates = legacyCandidates
	}
	for _, candidate := range state.Candidates {
		ids := state.ResumeAttachmentSourceIDs[candidate.ID]
		for index := range candidate.ResumeAttachments {
			if index < len(ids) && ids[index] != "" {
				candidate.ResumeAttachments[index].SourceID = ids[index]
			}
		}
	}
	loaded := NewRecruitmentStore(state.Candidates)
	s.candidates = loaded.candidates
	s.order = loaded.order
	return nil
}

func candidateReceivedAt(candidate domain.RecruitmentCandidate) time.Time {
	if !candidate.ReceivedAt.IsZero() {
		return candidate.ReceivedAt
	}
	return candidate.UpdatedAt
}
