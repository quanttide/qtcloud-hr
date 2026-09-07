package recruitment

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/quanttide/qtcloud-human/src/provider/internal/domain"
)

func TestFileAuditLoggerUsesPrivateDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix directory permissions")
	}

	dir := filepath.Join(t.TempDir(), "audit")
	logger := NewFileAuditLogger(dir)
	if err := logger.Log(domain.RecruitmentAuditEntry{ActionID: "action_001"}); err != nil {
		t.Fatalf("write audit entry: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat audit directory: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("audit directory permissions = %o, want 700", got)
	}
}
