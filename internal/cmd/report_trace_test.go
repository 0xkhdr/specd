package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

// TestReportTraceResolvesParentsUniqueAndByteStable pins spec 07 R6.2 end to end:
// `report --trace` projects metadata-only spans whose ids are unique, whose
// parent references all resolve within the export, and whose two exports over the
// same tree are byte-identical. A seeded ACP dispatch for T1 gives the verify and
// completion spans a real parent to resolve.
func TestReportTraceResolvesParentsUniqueAndByteStable(t *testing.T) {
	root := newHistoryDemo(t)

	// Seed a dispatch so activity spans have a parent to resolve against (R6.2).
	acp := filepath.Join(core.SpecdDir(root), "specs", "demo", "acp.jsonl")
	if err := orchestration.AppendACP(acp, orchestration.ACPEvent{
		Kind:   orchestration.ACPKindDispatch,
		TaskID: "T1",
		Time:   time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("seed dispatch: %v", err)
	}

	first, err := captureStdout(t, func() error {
		return Run(root, "report", []string{"demo"}, map[string]string{"trace": ""})
	})
	if err != nil {
		t.Fatalf("report --trace: %v", err)
	}

	ids := map[string]bool{}
	var spans []core.RunSpan
	sawDispatchParent := false
	for _, line := range strings.Split(strings.TrimRight(first, "\n"), "\n") {
		if line == "" {
			continue
		}
		var s core.RunSpan
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Fatalf("trace line %q not JSON: %v", line, err)
		}
		if s.SpanID == "" {
			t.Fatalf("span missing span_id: %q", line)
		}
		if ids[s.SpanID] {
			t.Fatalf("duplicate span_id %q", s.SpanID)
		}
		ids[s.SpanID] = true
		spans = append(spans, s)
	}
	for _, s := range spans {
		if s.ParentSpanID == "" {
			continue
		}
		if !ids[s.ParentSpanID] {
			t.Fatalf("span %q references unknown parent %q", s.SpanID, s.ParentSpanID)
		}
		if s.Kind == core.SpanVerify && s.TaskID == "T1" {
			sawDispatchParent = true
		}
	}
	if !sawDispatchParent {
		t.Fatal("expected a T1 verify span parented to its dispatch")
	}

	// R6.2: a second export over the same tree is byte-identical.
	second, err := captureStdout(t, func() error {
		return Run(root, "report", []string{"demo"}, map[string]string{"trace": ""})
	})
	if err != nil {
		t.Fatalf("second report --trace: %v", err)
	}
	if first != second {
		t.Fatalf("trace not byte-identical:\n--- 1 ---\n%s\n--- 2 ---\n%s", first, second)
	}
}
