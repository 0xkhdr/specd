package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestActionableContextBudgetRefusal(t *testing.T) {
	root := newDemoSpec(t)
	t.Setenv("SPECD_CONTEXT_MAX_TOKENS", "1")
	writeTasks(t, root, "demo", "| T1 | craftsman | z.go,a.go | - | go test ./... | R1.1 |")
	for path, body := range map[string]string{"a.go": "aaaa", "z.go": "zzzzzzzz"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	checkOutput, err := captureStdout(t, func() error { return Run(root, "check", []string{"demo"}, nil) })
	if err == nil {
		t.Fatal("check unexpectedly accepted an over-budget task")
	}
	assertActionableBudgetRefusal(t, checkOutput)

	guideOutput, err := captureStdout(t, func() error {
		return Run(root, "status", []string{"demo"}, map[string]string{"guide": "", "json": ""})
	})
	if err != nil {
		t.Fatalf("status --guide --json: %v", err)
	}
	var guide core.Guidance
	if err := json.Unmarshal([]byte(guideOutput), &guide); err != nil {
		t.Fatalf("decode guidance: %v (out=%q)", err, guideOutput)
	}
	assertActionableBudgetRefusal(t, strings.Join(guide.Blockers, "\n"))
}

func assertActionableBudgetRefusal(t *testing.T, output string) {
	t.Helper()
	if a, z := strings.Index(output, "a.go="), strings.Index(output, "z.go="); a < 0 || z < 0 || a >= z {
		t.Fatalf("contributions are not source ordered: %s", output)
	}
	for _, want := range []string{
		"required source contributions:",
		"1. the tasks.md owner edits .specd/specs/demo/tasks.md",
		"2. an agent runs `specd check demo`",
		"3. a human runs `specd approve demo`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("refusal missing %q: %s", want, output)
		}
	}
	if strings.Count(output, "recovery: 1.") != 1 {
		t.Fatalf("refusal must contain one recovery sequence: %s", output)
	}
}
