package core

import (
	"strings"
	"testing"
)

// TestParseDeployPlanValid accepts a well-formed plan and normalizes the env.
func TestParseDeployPlanValid(t *testing.T) {
	plan, err := ParseDeployPlan("staging", []byte(`{
		"requiresGates": ["eval", "review"],
		"approvalRequired": true,
		"steps": [
			{"name": "push", "command": "kubectl apply -f -", "rollbackCommand": "kubectl rollout undo", "timeoutSeconds": 60}
		]
	}`))
	if err != nil {
		t.Fatalf("valid plan rejected: %v", err)
	}
	if plan.Env != "staging" || len(plan.Steps) != 1 || !plan.ApprovalRequired {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

// TestParseDeployPlanHostile rejects the adversarial-config matrix (P4.4 same-PR
// adversarial coverage): unknown fields, missing/non-positive/oversize timeouts,
// empty steps, duplicate names, unknown gates, and env mismatch.
func TestParseDeployPlanHostile(t *testing.T) {
	cases := map[string]string{
		"unknown field":    `{"steps":[{"name":"a","command":"x","timeoutSeconds":1}],"evil":true}`,
		"missing timeout":  `{"steps":[{"name":"a","command":"x"}]}`,
		"zero timeout":     `{"steps":[{"name":"a","command":"x","timeoutSeconds":0}]}`,
		"negative timeout": `{"steps":[{"name":"a","command":"x","timeoutSeconds":-5}]}`,
		"oversize timeout": `{"steps":[{"name":"a","command":"x","timeoutSeconds":99999}]}`,
		"empty steps":      `{"steps":[]}`,
		"empty command":    `{"steps":[{"name":"a","command":"  ","timeoutSeconds":1}]}`,
		"empty name":       `{"steps":[{"name":"","command":"x","timeoutSeconds":1}]}`,
		"duplicate name":   `{"steps":[{"name":"a","command":"x","timeoutSeconds":1},{"name":"a","command":"y","timeoutSeconds":1}]}`,
		"unknown gate":     `{"requiresGates":["deploy"],"steps":[{"name":"a","command":"x","timeoutSeconds":1}]}`,
		"env mismatch":     `{"env":"prod","steps":[{"name":"a","command":"x","timeoutSeconds":1}]}`,
		"not json":         `not json`,
	}
	for name, body := range cases {
		if _, err := ParseDeployPlan("staging", []byte(body)); err == nil {
			t.Errorf("%s: expected rejection, got nil", name)
		}
	}
}

// TestValidateEnvTraversal rejects env names that would escape .specd/deploy/.
func TestValidateEnvTraversal(t *testing.T) {
	for _, bad := range []string{"../prod", "a/b", "..", "PROD", "prod space", ""} {
		if err := ValidateEnv(bad); err == nil {
			t.Errorf("ValidateEnv(%q) = nil, want error", bad)
		}
	}
	if err := ValidateEnv("production"); err != nil {
		t.Errorf("ValidateEnv(production) = %v, want nil", err)
	}
}

// TestDeployPreconditions covers the gate/approval refusal matrix.
func TestDeployPreconditions(t *testing.T) {
	plan := DeployPlan{Env: "production", RequiresGates: []string{"eval", "security", "review"}, Steps: []DeployStep{{Name: "s", Command: "x", TimeoutSeconds: 1}}}

	incomplete := &State{Spec: "x", Status: StatusExecuting}
	if got := DeployPreconditions(incomplete, plan, true); len(got) == 0 {
		t.Error("incomplete spec should be blocked")
	}

	// Complete but no gates/approval → multiple problems including approval.
	complete := &State{Spec: "x", Status: StatusComplete}
	probs := DeployPreconditions(complete, plan, true)
	joined := strings.Join(probs, "|")
	for _, want := range []string{"approval", "eval", "security", "review"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected problem mentioning %q, got %v", want, probs)
		}
	}

	// All green + approval → clear.
	green := &State{
		Spec: "x", Status: StatusComplete,
		DeployApproval: &DeployApproval{Env: "production"},
		Evals:          map[string]EvalSummary{"default": {Pass: true}},
		Security:       &SecurityScan{Blocking: 0},
		Review:         &ReviewRecord{Verdict: string(ReviewApprove)},
	}
	if got := DeployPreconditions(green, plan, true); len(got) != 0 {
		t.Errorf("green spec should be clear, got %v", got)
	}

	// Blocking security finding re-blocks.
	green.Security = &SecurityScan{Blocking: 2}
	if got := DeployPreconditions(green, plan, true); len(got) == 0 {
		t.Error("blocking security finding should re-block")
	}
}

// TestRollbackChain reconstructs the inverse of the last run's successful steps
// in reverse order, ignoring failed steps and steps without a rollback command.
func TestRollbackChain(t *testing.T) {
	entries := []DeployLedgerEntry{
		{Seq: 1, Kind: "step", Step: "a", RollbackCommand: "undo-a", Success: true},
		{Seq: 2, Kind: "step", Step: "b", RollbackCommand: "", Success: true}, // no rollback → skipped
		{Seq: 3, Kind: "step", Step: "c", RollbackCommand: "undo-c", Success: true},
		{Seq: 4, Kind: "step", Step: "d", RollbackCommand: "undo-d", Success: false}, // failed → skipped
	}
	chain := RollbackChain(entries)
	if len(chain) != 2 || chain[0].Step != "c" || chain[1].Step != "a" {
		t.Fatalf("chain = %+v, want [c a]", chain)
	}
}

// TestRollbackChainScopedToLastRun stops at the prior rollback marker so a
// re-deploy after a rollback does not unwind the earlier run.
func TestRollbackChainScopedToLastRun(t *testing.T) {
	entries := []DeployLedgerEntry{
		{Kind: "step", Step: "old", RollbackCommand: "undo-old", Success: true},
		{Kind: "rollback", Step: "old", Success: true},
		{Kind: "step", Step: "new", RollbackCommand: "undo-new", Success: true},
	}
	chain := RollbackChain(entries)
	if len(chain) != 1 || chain[0].Step != "new" {
		t.Fatalf("chain = %+v, want [new] only (scoped past rollback marker)", chain)
	}
}

// TestDeployLedgerRoundTrip appends and reads back entries with monotonic seqs.
func TestDeployLedgerRoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := AppendDeployEntry(root, "svc", DeployLedgerEntry{Env: "staging", Kind: "step", Step: "a", Success: true}); err != nil {
		t.Fatal(err)
	}
	if err := AppendDeployEntry(root, "svc", DeployLedgerEntry{Env: "staging", Kind: "step", Step: "b", Success: true}); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadDeployLedger(root, "svc")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Seq != 1 || entries[1].Seq != 2 {
		t.Fatalf("entries = %+v, want 2 with seq 1,2", entries)
	}
}
