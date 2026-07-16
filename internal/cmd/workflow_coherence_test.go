package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestWorkflowCoherenceAgentCompletionGuidance(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, nil, map[string]string{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "verify alone does not complete task") || !strings.Contains(string(body), "specd complete-task <slug> <task>") {
		t.Fatalf("completion guidance is not truthful/executable:\n%s", body)
	}
	if !core.ForbiddenTool("task") {
		t.Fatal("broad task command unexpectedly exposed through tool contracts")
	}
}
