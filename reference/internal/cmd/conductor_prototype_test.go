package cmd_test

import (
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// buildVerifyingSpec seeds a minimal gate-valid spec parked at `verifying` (all
// work done, awaiting the completion accept) so the completion gates can be
// exercised directly.
func buildVerifyingSpec(h *th.Harness, slug string) {
	h.Spec(slug).
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "Implement", Verify: "true", Requirements: []int{1}, Status: core.TaskComplete}).
		Status(core.StatusVerifying).
		Build()
}

// writePassingRubric writes an eval rubric whose single regex check matches the
// seeded requirements.md, so `specd eval` records a passing run.
func writePassingRubric(t *testing.T, h *th.Harness, slug string) {
	t.Helper()
	path := core.DefaultEvalRubricPath(h.Root, slug)
	rubric := `{"minScore":0.5,"checks":[{"id":"has-req","kind":"regex","path":"requirements.md","pattern":"SYSTEM","weight":1}]}`
	if err := os.WriteFile(path, []byte(rubric), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestNewPrototypeFlag proves `new --prototype` records a pending prototype and
// says so, while a plain `new` does not.
func TestNewPrototypeFlag(t *testing.T) {
	h := th.New(t)
	res := h.RunExpect(core.ExitOK, "new", "spike", "--prototype")
	if !strings.Contains(res.Stdout, "prototype") {
		t.Errorf("expected prototype notice, got %q", res.Stdout)
	}
	proto := h.State("spike").Raw().Prototype
	if proto == nil || proto.Status != core.PrototypePending {
		t.Fatalf("prototype = %+v, want status pending", proto)
	}

	h.RunExpect(core.ExitOK, "new", "regular")
	if p := h.State("regular").Raw().Prototype; p != nil {
		t.Errorf("plain new set a prototype: %+v", p)
	}
}

// TestPrototypeCannotComplete proves a prototype spec is hard-blocked from
// reaching complete until it is promoted (V5 acceptance; invariant 5).
func TestPrototypeCannotComplete(t *testing.T) {
	h := th.New(t)
	buildVerifyingSpec(h, "auth")
	// Mark it a prototype after building the verifying spec.
	st, err := core.LoadState(h.Root, "auth")
	if err != nil {
		t.Fatal(err)
	}
	st.Prototype = &core.PrototypeState{Status: core.PrototypePending}
	if err := core.SaveState(h.Root, "auth", st); err != nil {
		t.Fatal(err)
	}

	res := h.RunExpect(core.ExitGate, "approve", "auth")
	if !strings.Contains(res.Out(), "promote") {
		t.Errorf("block message should point at promote, got %q", res.Out())
	}
	h.State("auth").Status(core.StatusVerifying) // unchanged

	// Promote (passing eval + evidence) then completion is allowed.
	writePassingRubric(t, h, "auth")
	h.RunExpect(core.ExitOK, "eval", "auth")
	h.RunExpect(core.ExitOK, "promote", "auth", "--evidence", "manual review ok")
	if p := h.State("auth").Raw().Prototype; p == nil || p.Status != core.PrototypePromoted {
		t.Fatalf("prototype not promoted: %+v", p)
	}
	h.RunExpect(core.ExitOK, "approve", "auth")
	h.State("auth").Status(core.StatusComplete)
}

// TestEvalGateBlocksComplete proves the opt-in eval gate blocks completion until
// a passing eval run is recorded, and is off by default.
func TestEvalGateBlocksComplete(t *testing.T) {
	t.Run("off_by_default", func(t *testing.T) {
		h := th.New(t)
		buildVerifyingSpec(h, "auth")
		h.RunExpect(core.ExitOK, "approve", "auth")
		h.State("auth").Status(core.StatusComplete)
	})

	t.Run("required_blocks_then_allows", func(t *testing.T) {
		t.Setenv("SPECD_GATES_EVAL", "required")
		h := th.New(t)
		buildVerifyingSpec(h, "auth")

		res := h.RunExpect(core.ExitGate, "approve", "auth")
		if !strings.Contains(res.Out(), "eval gate") {
			t.Errorf("expected eval-gate block message, got %q", res.Out())
		}
		h.State("auth").Status(core.StatusVerifying)

		writePassingRubric(t, h, "auth")
		h.RunExpect(core.ExitOK, "eval", "auth")
		h.RunExpect(core.ExitOK, "approve", "auth")
		h.State("auth").Status(core.StatusComplete)
	})
}

// TestContextHUD proves `context --hud` renders deterministically and reports
// the load files with measured cost.
func TestContextHUD(t *testing.T) {
	h := th.New(t)
	h.Spec("auth").
		Req("Login", "story", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()

	a := h.RunExpect(core.ExitOK, "context", "auth", "--hud", "--json")
	b := h.RunExpect(core.ExitOK, "context", "auth", "--hud", "--json")
	if a.Stdout != b.Stdout {
		t.Fatalf("HUD output not stable:\n%s\n---\n%s", a.Stdout, b.Stdout)
	}
	for _, want := range []string{"\"files\"", "\"approxTokens\"", "\"mode\""} {
		if !strings.Contains(a.Stdout, want) {
			t.Errorf("HUD json missing %q:\n%s", want, a.Stdout)
		}
	}

	txt := h.RunExpect(core.ExitOK, "context", "auth", "--hud")
	if !strings.Contains(txt.Stdout, "HUD: auth") {
		t.Errorf("text HUD missing header:\n%s", txt.Stdout)
	}
}

// TestReportConductor proves `report --conductor` clusters the ledger's
// rejection reasons deterministically.
func TestReportConductor(t *testing.T) {
	h := th.New(t)
	h.Spec("auth").
		Req("Login", "story", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()

	for _, reason := range []string{"flaky", "wrong file", "flaky"} {
		if err := core.AppendConductorEvent(h.Root, "auth", core.ConductorEvent{Action: "reject", Reason: reason}); err != nil {
			t.Fatal(err)
		}
	}
	res := h.RunExpect(core.ExitOK, "report", "auth", "--conductor")
	// "flaky" (x2) must be listed before "wrong file" (x1).
	fi := strings.Index(res.Stdout, "flaky")
	wi := strings.Index(res.Stdout, "wrong file")
	if fi < 0 || wi < 0 || fi > wi {
		t.Errorf("expected flaky (x2) before wrong file (x1):\n%s", res.Stdout)
	}
}
