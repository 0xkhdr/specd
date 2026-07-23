package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

func TestMachineExitSeverityParity(t *testing.T) {
	t.Run("check_text_and_json", func(t *testing.T) {
		root := newDemoSpec(t)
		path := core.StatePath(root, "demo")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		raw = []byte(strings.Replace(string(raw), "\n}", ",\n  \"unexpected\": true\n}", 1))
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}

		for _, flags := range []map[string]string{
			{"schema-only": "true"},
			{"schema-only": "true", "json": "true"},
		} {
			output, err := captureStdout(t, func() error {
				return Run(root, "check", []string{"demo"}, flags)
			})
			if err == nil {
				t.Fatalf("check flags %v returned success for error finding: %s", flags, output)
			}
			if !strings.Contains(output, "schema") || !strings.Contains(output, "error") {
				t.Fatalf("check flags %v did not render the error finding: %s", flags, output)
			}
		}
		if err := diagnosticCheckFailure([]gates.Finding{{Severity: gates.Warn}}); err != nil {
			t.Fatalf("warning-only findings returned nonzero: %v", err)
		}
	})

	t.Run("controller_halt_and_dispatch", func(t *testing.T) {
		halted := newBrainTestRoot(t, "orchestrated", brainEnabledConfig+"routing:\n  allow_unknown_telemetry: false\n")
		writeBrainSingleTask(t, halted)
		if err := runBrain(halted, []string{"start", "demo"}, nil); err != nil {
			t.Fatal(err)
		}
		err := runBrain(halted, []string{"run", "demo"}, map[string]string{"authority": ""})
		if !errors.Is(err, ErrControllerHalt) || !errors.Is(err, ErrUsage) {
			t.Fatalf("zero-dispatch halt classification = %v, want controller halt/exit 2", err)
		}
		if refusal, ok := core.AsRefusal(err); !ok || refusal.StateChanged || !strings.Contains(refusal.Error(), "halt") {
			t.Fatalf("zero-dispatch refusal = %+v, found=%v", refusal, ok)
		}

		dispatched := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
		writeBrainSingleTask(t, dispatched)
		if err := runBrain(dispatched, []string{"start", "demo"}, nil); err != nil {
			t.Fatal(err)
		}
		if err := runBrain(dispatched, []string{"run", "demo"}, map[string]string{"authority": ""}); err != nil {
			t.Fatalf("successful dispatch changed: %v", err)
		}
		if missions := loadBrainSession(t, dispatched).PendingMissions; len(missions) != 1 {
			t.Fatalf("successful run dispatched %d missions, want 1", len(missions))
		}
	})
}

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
