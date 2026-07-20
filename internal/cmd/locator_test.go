package cmd

import (
	"encoding/json"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestMachineLocator pins R5.1: a machine-readable status answers "where am I
// and what may I do" without a second round trip, and does so additively — the
// fields a pre-locator consumer parsed are all still where they were.
func TestMachineLocator(t *testing.T) {
	root := newDemoSpec(t)
	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}

	for _, surface := range []struct {
		name  string
		flags map[string]string
	}{
		{"status --json", map[string]string{"json": "true"}},
		{"status --guide --json", map[string]string{"guide": "true", "json": "true"}},
	} {
		t.Run(surface.name, func(t *testing.T) {
			out, err := captureStdout(t, func() error {
				return Run(root, "status", []string{"demo"}, surface.flags)
			})
			if err != nil {
				t.Fatalf("status: %v", err)
			}

			var payload struct {
				Locator core.Locator `json:"locator"`
			}
			if err := json.Unmarshal([]byte(out), &payload); err != nil {
				t.Fatalf("unmarshal: %v\n%s", err, out)
			}
			locator := payload.Locator

			if locator.Spec != "demo" {
				t.Errorf("spec = %q, want demo", locator.Spec)
			}
			if locator.Phase == "" || locator.Status == "" {
				t.Errorf("phase/status not stated: %+v", locator)
			}
			if locator.Revision != state.Revision {
				t.Errorf("revision = %d, want %d", locator.Revision, state.Revision)
			}
			if locator.ActorClass != core.ActorAgent {
				t.Errorf("actor_class = %q, want agent", locator.ActorClass)
			}
			if locator.Authority != core.AuthorityNone {
				t.Errorf("authority = %q, want none — no token was presented", locator.Authority)
			}
			if len(locator.LegalOperations) == 0 {
				t.Error("no legal operations stated, so the agent must still guess")
			}
			if len(locator.HumanOnly) == 0 {
				t.Error("human-only boundary not stated")
			}
			// No sandbox was declared, so the session must not read as governed.
			if locator.Assurance != core.AssuranceAdvisory {
				t.Errorf("assurance = %q, want advisory on an unsandboxed host", locator.Assurance)
			}
			if !locator.Authoritative {
				t.Error("a locator built from loaded state.json must report authoritative")
			}
			// The locator reports, it never widens: nothing legal here may be human-only.
			for _, op := range locator.LegalOperations {
				for _, human := range locator.HumanOnly {
					if op == human {
						t.Errorf("%q listed as both legal and human-only", op)
					}
				}
			}

			// Additive: pre-locator keys unchanged.
			var legacy map[string]json.RawMessage
			if err := json.Unmarshal([]byte(out), &legacy); err != nil {
				t.Fatalf("legacy unmarshal: %v", err)
			}
			want := "slug"
			if surface.flags["guide"] == "true" {
				want = "legal_commands"
			}
			if _, ok := legacy[want]; !ok {
				t.Errorf("legacy key %q missing; locator replaced the payload instead of extending it", want)
			}
		})
	}
}
