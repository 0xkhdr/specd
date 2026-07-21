package gates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestContextBudgetGate(t *testing.T) {
	findings := contextBudget(CheckCtx{
		Slug: "demo",
		Tasks: []core.TaskRow{
			{ID: "T1", Role: "craftsman"},
		},
		MaxContextTokens: 1,
	})
	if len(findings) != 1 || findings[0].Severity != Error || !strings.Contains(findings[0].Message, "exceeds budget") {
		t.Fatalf("findings = %+v", findings)
	}
}

// TestContextBudgetRequiredFailsClosed (R3.2): when the required context alone
// exceeds the budget, the gate reports one error naming the task and a concise
// remediation (decompose / narrow declared files) — the required set is never
// silently budgeted down to fit.
func TestContextBudgetRequiredFailsClosed(t *testing.T) {
	root := t.TempDir()
	design := filepath.Join(root, ".specd", "specs", "demo", "design.md")
	if err := os.MkdirAll(filepath.Dir(design), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(design, make([]byte, 8000), 0o644); err != nil { // ~2000 required tokens
		t.Fatal(err)
	}
	findings := contextBudget(CheckCtx{
		Root:             root,
		Slug:             "demo",
		Tasks:            []core.TaskRow{{ID: "T1", Role: "craftsman"}},
		MaxContextTokens: 100,
	})
	if len(findings) != 1 || findings[0].Severity != Error {
		t.Fatalf("findings = %+v", findings)
	}
	for _, want := range []string{"T1", "exceeds budget", "decompose", "narrow declared files"} {
		if !strings.Contains(findings[0].Message, want) {
			t.Fatalf("finding missing %q: %s", want, findings[0].Message)
		}
	}
}

func TestContextBudgetIgnoresCompletedTasks(t *testing.T) {
	findings := contextBudget(CheckCtx{
		Slug:             "demo",
		Tasks:            []core.TaskRow{{ID: "T1", Role: "craftsman"}},
		Status:           map[string]core.TaskRunStatus{"T1": core.TaskComplete},
		MaxContextTokens: 1,
	})
	if len(findings) != 0 {
		t.Fatalf("completed task findings = %+v", findings)
	}
}
