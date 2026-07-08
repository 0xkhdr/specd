package cmd

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestBuildBrief(t *testing.T) {
	mk := func(status core.SpecStatus) *core.State {
		s := core.InitialState("auth", "Auth")
		s.Status = status
		return &s
	}

	cases := []struct {
		name       string
		state      *core.State
		wantLabel  string
		wantInNext string
	}{
		{"requirements", mk(core.StatusRequirements), "ANALYZE", "approve"},
		{"design", mk(core.StatusDesign), "PLAN (design)", "approve"},
		{"tasks", mk(core.StatusTasks), "PLAN (tasks)", "approve"},
		{"executing", mk(core.StatusExecuting), "EXECUTE", "specd next"},
		{"verifying", mk(core.StatusVerifying), "VERIFY", "approve"},
		{"complete", mk(core.StatusComplete), "REFLECT", "memory"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := buildBrief(c.state, "auth", "make test")
			if b.phaseLabel != c.wantLabel {
				t.Errorf("phaseLabel = %q, want %q", b.phaseLabel, c.wantLabel)
			}
			if !strings.Contains(b.next, c.wantInNext) {
				t.Errorf("next = %q, want it to contain %q", b.next, c.wantInNext)
			}
			if b.purpose == "" || b.focus == "" || len(b.load) == 0 {
				t.Errorf("brief has empty fields: %+v", b)
			}
		})
	}
}

func TestBuildBrief_BlockedSubConditional(t *testing.T) {
	st := core.InitialState("auth", "Auth")
	st.Status = core.StatusBlocked

	// No blockers recorded yet: generic stuck message.
	b := buildBrief(&st, "auth", "")
	if b.phaseLabel != "EXECUTE (blocked)" {
		t.Fatalf("phaseLabel = %q, want EXECUTE (blocked)", b.phaseLabel)
	}
	if !strings.Contains(b.focus, "All remaining tasks blocked") {
		t.Errorf("focus = %q, want generic blocked message", b.focus)
	}

	// With a recorded blocker: point the agent at SIGNALS.
	st.Blockers = []core.Blocker{{Task: "T1", Reason: "missing creds", Since: "now"}}
	b = buildBrief(&st, "auth", "")
	if !strings.Contains(b.focus, "Resolve the blockers") {
		t.Errorf("focus = %q, want blocker-resolution message", b.focus)
	}
}

func TestBuildBrief_UnknownStatusDefault(t *testing.T) {
	st := core.InitialState("auth", "Auth")
	st.Status = core.SpecStatus("bogus")
	b := buildBrief(&st, "auth", "")
	if b.phaseLabel != "" || b.purpose != "" || b.focus != "" || b.next != "" || len(b.load) != 0 {
		t.Errorf("unknown status should yield zero brief, got %+v", b)
	}
}
