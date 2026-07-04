package gates

import (
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
