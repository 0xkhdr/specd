package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestWorkflowCoherenceBaselineAgentCompletionGap(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, nil, map[string]string{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "what marks a task complete") {
		t.Fatalf("W0 baseline no longer claims verify completes tasks:\n%s", body)
	}
	if !core.ForbiddenTool("task") {
		t.Fatal("W0 baseline unexpectedly exposes task completion through tool contracts")
	}
}
