package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// completeSpec builds a gate-valid spec parked at complete for deploy tests.
func completeSpec(h *th.Harness, slug string) {
	h.Spec(slug).
		Req("Ship", "As an operator, I want to deploy", "THE SYSTEM SHALL deploy the service.").
		FullDesign().
		Status(core.StatusComplete).
		Build()
}

// writeDeployPlan writes a deploy config for env under the harness root.
func writeDeployPlan(t *testing.T, h *th.Harness, env, body string) {
	t.Helper()
	dir := filepath.Join(h.Root, ".specd", "deploy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, env+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDeploySuccessChain runs all steps and records a succeeded outcome.
func TestDeploySuccessChain(t *testing.T) {
	h := th.New(t)
	completeSpec(h, "svc")
	marker := filepath.Join(h.Root, "deployed")
	writeDeployPlan(t, h, "staging", `{
		"steps": [
			{"name": "one", "command": "printf a >> `+marker+`", "rollbackCommand": "true", "timeoutSeconds": 10},
			{"name": "two", "command": "printf b >> `+marker+`", "rollbackCommand": "true", "timeoutSeconds": 10}
		]
	}`)

	h.RunExpect(core.ExitOK, "deploy", "svc", "--env", "staging")

	got, err := os.ReadFile(marker)
	if err != nil || string(got) != "ab" {
		t.Fatalf("steps did not run in order: %q err=%v", got, err)
	}
	if st := h.State("svc").Raw(); st.Deploy == nil || st.Deploy.Outcome != "succeeded" || st.Deploy.Steps != 2 {
		t.Fatalf("deploy record = %+v, want succeeded/2", st.Deploy)
	}
}

// TestDeployMidChainFailureThenRollback: a failing step halts, exit gate, and
// rollback unwinds only the recorded successful step.
func TestDeployMidChainFailureThenRollback(t *testing.T) {
	h := th.New(t)
	completeSpec(h, "svc")
	rb := filepath.Join(h.Root, "rolledback")
	writeDeployPlan(t, h, "staging", `{
		"steps": [
			{"name": "one", "command": "true", "rollbackCommand": "printf 1 >> `+rb+`", "timeoutSeconds": 10},
			{"name": "two", "command": "exit 1", "rollbackCommand": "printf 2 >> `+rb+`", "timeoutSeconds": 10}
		]
	}`)

	h.RunExpect(core.ExitGate, "deploy", "svc", "--env", "staging")

	// Rollback unwinds step one only (step two failed → not in the chain).
	h.RunExpect(core.ExitOK, "deploy", "rollback", "svc", "--env", "staging")
	got, err := os.ReadFile(rb)
	if err != nil || string(got) != "1" {
		t.Fatalf("rollback chain = %q err=%v, want only step-one rollback \"1\"", got, err)
	}
}

// TestDeployGateRefusal: a plan requiring a gate with no recorded evidence is
// blocked before any step runs.
func TestDeployGateRefusal(t *testing.T) {
	h := th.New(t)
	completeSpec(h, "svc")
	ran := filepath.Join(h.Root, "ran")
	writeDeployPlan(t, h, "staging", `{
		"requiresGates": ["security"],
		"steps": [{"name": "one", "command": "printf x >> `+ran+`", "timeoutSeconds": 10}]
	}`)

	res := h.RunExpect(core.ExitGate, "deploy", "svc", "--env", "staging")
	if _, err := os.Stat(ran); !os.IsNotExist(err) {
		t.Fatal("step ran despite gate refusal")
	}
	if !strings.Contains(res.Out(), "security") {
		t.Errorf("expected security-gate refusal, got %q", res.Out())
	}
}

// TestDeployProductionRequiresApproval: production deploy is impossible without a
// recorded human approval; once approved it proceeds.
func TestDeployProductionRequiresApproval(t *testing.T) {
	h := th.New(t)
	completeSpec(h, "svc")
	marker := filepath.Join(h.Root, "prod")
	writeDeployPlan(t, h, "production", `{
		"steps": [{"name": "one", "command": "printf x >> `+marker+`", "timeoutSeconds": 10}]
	}`)

	// Without approval → blocked, nothing ran.
	res := h.RunExpect(core.ExitGate, "deploy", "svc", "--env", "production")
	if !strings.Contains(res.Out(), "approval") {
		t.Errorf("expected approval refusal, got %q", res.Out())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("production step ran without approval")
	}

	// Record the human deploy approval, then deploy proceeds.
	h.RunExpect(core.ExitOK, "approve", "svc", "--deploy", "--env", "production")
	h.RunExpect(core.ExitOK, "deploy", "svc", "--env", "production")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("approved production deploy did not run: %v", err)
	}
}

// TestDeployDryRun shows the plan without executing.
func TestDeployDryRun(t *testing.T) {
	h := th.New(t)
	completeSpec(h, "svc")
	marker := filepath.Join(h.Root, "dry")
	writeDeployPlan(t, h, "staging", `{
		"steps": [{"name": "one", "command": "printf x >> `+marker+`", "timeoutSeconds": 10}]
	}`)
	h.RunExpect(core.ExitOK, "deploy", "svc", "--env", "staging", "--dry-run")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("dry-run executed a step")
	}
}
