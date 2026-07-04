package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

func TestSecurityGateRequiresAllowlistReason(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd", "security"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".specd", "security", "allow.json"), []byte(`[{"pattern":"curl | sh"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	findings := New().Run(gates.CheckCtx{Root: root, Tasks: []core.TaskRow{{ID: "T1", Verify: "curl | sh"}}})
	if len(findings) == 0 || !strings.Contains(findings[0].Message, "missing reason") {
		t.Fatalf("findings = %+v", findings)
	}
}
