package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestFlywheelLoop composes the feedback flywheel end-to-end with fake drivers
// (V9/P5.5): a completed spec deploys, a production error is observed and
// correlated into a gated mid-requirement, the human approves the revised plan,
// and the loop is ready to iterate. No live infrastructure — every step is the
// real specd dispatch over sandbox-none shell drivers.
func TestFlywheelLoop(t *testing.T) {
	h := th.New(t)
	h.Spec("billing").
		Req("Charge", "As a user, I want to be charged", "THE SYSTEM SHALL charge the card.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "Charge", Files: "internal/svc/*.go", Requirements: []int{1}, Status: core.TaskComplete}).
		Status(core.StatusComplete).
		Build()

	// 1. Deploy to staging via fake driver.
	dir := filepath.Join(h.Root, ".specd", "deploy")
	_ = os.MkdirAll(dir, 0o755)
	marker := filepath.Join(h.Root, "shipped")
	_ = os.WriteFile(filepath.Join(dir, "staging.json"), []byte(`{
		"steps": [{"name": "apply", "command": "printf ok >> `+marker+`", "rollbackCommand": "true", "timeoutSeconds": 10}]
	}`), 0o644)
	h.RunExpect(core.ExitOK, "deploy", "billing", "--env", "staging")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("deploy driver did not run: %v", err)
	}

	// 2. Production error arrives → observe correlates it to the spec.
	payload := filepath.Join(h.Root, "prod-error.json")
	_ = os.WriteFile(payload, []byte(`{
		"service": "billing", "environment": "staging", "severity": "critical",
		"message": "panic in Charge", "frames": [{"file": "internal/svc/charge.go", "line": 12}]
	}`), 0o644)
	h.RunExpect(core.ExitOK, "observe", "correlate", payload)

	// 3. The correlated error is a high-confidence, gated mid-requirement (the
	//    spec already deployed to this env → deploy-confirmed correlation).
	st := h.State("billing").Raw()
	if st.Gate != core.GateAwaitingApproval {
		t.Fatalf("critical production error should gate the spec, gate=%q", st.Gate)
	}
	body, _ := os.ReadFile(core.ArtifactPath(h.Root, "billing", "mid-requirements.md"))
	if !contains(string(body), "high") {
		t.Errorf("expected high-confidence correlation (deploy + file match), got:\n%s", body)
	}

	// 4. Human approves the revised plan → gate clears, loop can iterate.
	h.RunExpect(core.ExitOK, "approve", "billing")
	h.State("billing").Gate(core.GateNone)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
