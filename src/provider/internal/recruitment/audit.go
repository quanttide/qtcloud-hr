package recruitment

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
)

type AuditLogger interface {
	Log(entry domain.RecruitmentAuditEntry) error
}

type FileAuditLogger struct {
	dir string
	mu  sync.Mutex
}

func NewFileAuditLogger(dir string) *FileAuditLogger {
	return &FileAuditLogger{dir: dir}
}

func (l *FileAuditLogger) Log(entry domain.RecruitmentAuditEntry) error {
	if l == nil || l.dir == "" {
		return nil
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	path := filepath.Join(l.dir, "recruitment-actions.jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

type SlogAuditLogger struct{}

func (SlogAuditLogger) Log(entry domain.RecruitmentAuditEntry) error {
	slog.Info(
		"recruitment action",
		"action_id", entry.ActionID,
		"candidate_id", entry.CandidateID,
		"action", entry.Action,
		"operator", entry.Operator,
		"status", entry.Status,
	)
	return nil
}
