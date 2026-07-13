package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLintBeforeBuild(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".specd", "steering")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "# Memory\n\n## Rule\n**Pattern:** must use atomic writes\n**Source:** evidence:a\n**Criticality:** critical\n**Owner:** platform\n**Applies-To:** phase=executing\n**Status:** active\n\n##  rule  \n**Pattern:** must not use atomic writes\n**Source:** review:b\n**Criticality:** critical\n**Owner:** platform\n**Applies-To:** phase=executing\n**Status:** active\n"
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	items, findings, err := SelectMemory(root, "demo", SelectionContext{Phase: "executing", AsOf: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), MemoryLintRequired: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("conflicted memory reached context: %+v", items)
	}
	if len(findings) == 0 || !strings.Contains(findings[0].Reason, "memory lint") {
		t.Fatalf("missing pre-build lint finding: %+v", findings)
	}
}
