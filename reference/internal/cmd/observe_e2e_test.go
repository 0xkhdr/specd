package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestObserveCorrelateWritesMidreq: a production error whose stack frame matches
// a task's files contract is attributed to that spec, appends an evidenced
// mid-requirements entry, and gates high impact for approval (V9/P5.2 + the
// success metric: every observed error → an evidenced midreq entry).
func TestObserveCorrelateWritesMidreq(t *testing.T) {
	h := th.New(t)
	h.Spec("billing").
		Req("Charge", "As a user, I want to be charged", "THE SYSTEM SHALL charge the card.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "Charge", Files: "internal/svc/*.go", Requirements: []int{1}, Status: core.TaskRunning}).
		Status(core.StatusExecuting).
		Build()

	payload := filepath.Join(h.Root, "err.json")
	if err := os.WriteFile(payload, []byte(`{
		"service": "billing", "environment": "production", "severity": "error",
		"message": "nil pointer in Charge",
		"frames": [{"file": "internal/svc/charge.go", "line": 88}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	res := h.RunExpect(core.ExitOK, "observe", "correlate", payload)
	if !strings.Contains(res.Out(), "billing") {
		t.Errorf("expected correlation to billing, got %q", res.Out())
	}

	body, err := os.ReadFile(core.ArtifactPath(h.Root, "billing", "mid-requirements.md"))
	if err != nil {
		t.Fatalf("mid-requirements.md not written: %v", err)
	}
	for _, want := range []string{"production error (observe)", "nil pointer in Charge", "internal/svc/charge.go", "confidence"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("midreq entry missing %q:\n%s", want, body)
		}
	}
	// High impact (severity error) → awaiting-approval gate, turn bumped.
	h.State("billing").Gate(core.GateAwaitingApproval).Turn(1)
}

// TestObserveCorrelateNoMatchRequiresSpec: an unattributable payload is rejected
// with guidance to force a spec, rather than silently dropping the error.
func TestObserveCorrelateNoMatchRequiresSpec(t *testing.T) {
	h := th.New(t)
	h.Spec("other").Req("X", "As a user, I want X", "THE SYSTEM SHALL do X.").FullDesign().Status(core.StatusExecuting).Build()

	payload := filepath.Join(h.Root, "err.json")
	_ = os.WriteFile(payload, []byte(`{"severity":"warning","message":"unmatched","environment":"staging"}`), 0o644)

	res := h.RunExpect(core.ExitGate, "observe", "correlate", payload)
	if !strings.Contains(res.Out(), "--spec") {
		t.Errorf("expected --spec guidance, got %q", res.Out())
	}

	// Forcing the spec attributes it (low confidence) and writes the entry.
	h.RunExpect(core.ExitOK, "observe", "correlate", payload, "--spec", "other")
	if _, err := os.Stat(core.ArtifactPath(h.Root, "other", "mid-requirements.md")); err != nil {
		t.Fatalf("forced attribution did not write midreq: %v", err)
	}
}

// TestObserveRejectsHostilePayload: a traversing frame path is rejected.
func TestObserveRejectsHostilePayload(t *testing.T) {
	h := th.New(t)
	h.Spec("svc").Req("X", "As a user, I want X", "THE SYSTEM SHALL do X.").FullDesign().Status(core.StatusExecuting).Build()
	payload := filepath.Join(h.Root, "evil.json")
	_ = os.WriteFile(payload, []byte(`{"severity":"error","message":"x","frames":[{"file":"../../etc/passwd"}]}`), 0o644)
	h.RunExpect(core.ExitGate, "observe", "correlate", payload)
}
