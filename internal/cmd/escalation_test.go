package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// failVerify appends n failing verify records for taskID so the task escalates
// under the default ratchet (3).
func failVerify(t *testing.T, root, slug, taskID string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		rec := core.EvidenceRecord{TaskID: taskID, Command: "false", ExitCode: 1, GitHead: "abc"}
		if err := core.AppendEvidence(core.EvidencePath(root, slug), rec); err != nil {
			t.Fatalf("append evidence: %v", err)
		}
	}
}

// advancePastRequirements approves requirements and design so execution verbs
// like `verify` pass both lifecycle and approval gates.
func advancePastRequirements(t *testing.T, root, slug string) {
	t.Helper()
	for range 2 {
		if err := Run(root, "approve", []string{slug}, nil); err != nil {
			t.Fatalf("approve next: %v", err)
		}
	}
}

func TestTaskOverride(t *testing.T) {
	root := newDemoSpec(t)
	advancePastRequirements(t, root, "demo")
	failVerify(t, root, "demo", "T1", 3)

	t.Run("override_without_reason_is_usage_error", func(t *testing.T) {
		err := Run(root, "task", []string{"T1"}, map[string]string{"override": ""})
		if err == nil || !strings.Contains(err.Error(), "usage") {
			t.Fatalf("want usage error, got %v", err)
		}
	})

	t.Run("verify_blocked_while_escalated", func(t *testing.T) {
		err := Run(root, "verify", []string{"demo", "T1"}, nil)
		if err == nil || !strings.Contains(err.Error(), "escalated") {
			t.Fatalf("want escalation block, got %v", err)
		}
	})

	t.Run("override_with_reason_clears_ratchet", func(t *testing.T) {
		if err := Run(root, "task", []string{"T1"}, map[string]string{"override": "", "reason": "flaky infra, verified manually"}); err != nil {
			t.Fatalf("override: %v", err)
		}
		count, err := taskFailCount(root, "demo", "T1")
		if err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("fail count after override = %d, want 0", count)
		}
		overrides, err := core.LoadOverrides(core.OverridePath(root, "demo"))
		if err != nil || len(overrides) != 1 || overrides[0].PriorFailCount != 3 {
			t.Fatalf("override ledger = %+v (err %v)", overrides, err)
		}
	})

	t.Run("verify_runs_after_override", func(t *testing.T) {
		// The demo task's verify is `true` (exit 0); after the override the ratchet
		// no longer blocks it, so verify succeeds and records passing evidence.
		if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
			t.Fatalf("verify after override: %v", err)
		}
	})

	t.Run("override_on_non_escalated_task_refused", func(t *testing.T) {
		// After the passing verify above the task is no longer escalated.
		err := Run(root, "task", []string{"T1"}, map[string]string{"override": "", "reason": "again"})
		if err == nil || !strings.Contains(err.Error(), "not escalated") {
			t.Fatalf("want not-escalated refusal, got %v", err)
		}
	})
}

func TestStatusEscalated(t *testing.T) {
	root := newDemoSpec(t)
	failVerify(t, root, "demo", "T1", 3)

	t.Run("text_status_surfaces_escalation", func(t *testing.T) {
		out, err := captureStdout(t, func() error { return Run(root, "status", []string{"demo"}, nil) })
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if !strings.Contains(out, "Escalated") || !strings.Contains(out, "T1") {
			t.Fatalf("status output missing escalation: %q", out)
		}
	})

	t.Run("json_status_carries_escalated_map", func(t *testing.T) {
		out, err := captureStdout(t, func() error { return Run(root, "status", []string{"demo"}, map[string]string{"json": ""}) })
		if err != nil {
			t.Fatalf("status --json: %v", err)
		}
		var payload struct {
			Escalated map[string]int `json:"escalated"`
		}
		if err := json.Unmarshal([]byte(out), &payload); err != nil {
			t.Fatalf("decode: %v\n%s", err, out)
		}
		if payload.Escalated["T1"] != 3 {
			t.Fatalf("escalated map = %+v, want T1:3", payload.Escalated)
		}
	})
}
