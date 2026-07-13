package gates

import (
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func TestMemoryConflictLint(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	blocks := []core.MemBlock{
		{Key: "Atomic Writes", Pattern: "must use atomic writes", Criticality: "critical", Status: "active", Owner: "platform", Source: "evidence:a", Provenance: "build:a"},
		{Key: " atomic   writes ", Pattern: "must not use atomic writes", Criticality: "critical", Status: "active", Owner: "platform", Source: "review:b", Provenance: "review:b"},
		{Key: "future", Pattern: "must not use atomic writes", Criticality: "critical", Status: "active", Owner: "platform", ExpiresAt: "2027-01-01"},
		{Key: "old", Pattern: "must not use atomic writes", Criticality: "critical", Status: "active", Owner: "platform", ExpiresAt: "2026-01-01"},
		{Key: "unknown-confidence", Pattern: "atomic writes", Criticality: "critical", Status: "active", Confidence: "unknown"},
		{Key: "forced", Raw: "**Promoted:** x [mode=forced authority=lead provenance=review:r1]", Status: "active"},
	}
	findings := memoryConflictLint(CheckCtx{MemoryLintRequired: true, MemoryBlocks: blocks, MemoryAsOf: now})
	if len(findings) != 4 {
		t.Fatalf("findings=%+v, want duplicate, two contradictions, unowned force", findings)
	}
	var messages []string
	for _, finding := range findings {
		messages = append(messages, finding.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{"duplicate normalized memory key", "contradictory active critical memory", "forced memory promotion"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("findings missing %q: %s", want, joined)
		}
	}
	if got := memoryConflictLint(CheckCtx{MemoryBlocks: blocks, MemoryAsOf: now}); len(got) != 0 {
		t.Fatalf("default profile findings=%+v", got)
	}
}
