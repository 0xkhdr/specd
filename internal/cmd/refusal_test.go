package cmd

import (
	"errors"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestTypedRefusalUnknownCommandFailsClosed(t *testing.T) {
	err := RefuseUnknownCommand("nope")

	// The dispatcher classifies on this sentinel to exit 2. Adopting the typed
	// shape must not change that.
	if !errors.Is(err, ErrUnknownCommand) {
		t.Fatal("unknown-command refusal no longer matches ErrUnknownCommand")
	}

	refusal, ok := core.AsRefusal(err)
	if !ok {
		t.Fatalf("refusal is untyped: %v", err)
	}
	if refusal.Code != "UNKNOWN_COMMAND" {
		t.Fatalf("code=%q", refusal.Code)
	}
	if refusal.AuthorityConsumed {
		t.Fatal("refusal before authority issue reports authority_consumed true")
	}
	if refusal.RecoveryCommand == "" || refusal.ActorRequired != core.RefusalActorAgent {
		t.Fatalf("refusal leaves recovery unstated: %#v", refusal)
	}
}

// TestTypedRefusalDispatchSitesAreTyped enumerates the refusals reachable
// through dispatch and asserts each returns the one structured shape. R4.2 is
// about coverage: a single untyped path is where an agent starts improvising.
func TestTypedRefusalDispatchSitesAreTyped(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		code string
		verb string
		args []string
		flag map[string]string
	}{
		{name: "unknown-command", code: "UNKNOWN_COMMAND", verb: "definitely-not-a-verb"},
		{name: "traversal-slug", code: "SPEC_INVALID", verb: "check", args: []string{"../../escape"}},
		{name: "flag-enum", code: "FLAG_VALUE_INVALID", verb: "link", args: []string{"a", "b"}, flag: map[string]string{"kind": "not-a-kind"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(root, tc.verb, tc.args, tc.flag)
			refusal, ok := core.AsRefusal(err)
			if !ok {
				t.Fatalf("untyped refusal: %v", err)
			}
			if refusal.Code != tc.code {
				t.Fatalf("code=%q want %q", refusal.Code, tc.code)
			}
			if refusal.Blocker == "" || refusal.ActorRequired == "" || refusal.RecoveryCommand == "" {
				t.Fatalf("incomplete refusal: %#v", refusal)
			}
			// None of these ran an operation, so none burned a packet.
			if refusal.AuthorityConsumed {
				t.Fatalf("%s reports authority_consumed before authority issue", tc.name)
			}
		})
	}
}

func TestTypedRefusalReachesRunUnchanged(t *testing.T) {
	err := Run(t.TempDir(), "definitely-not-a-verb", nil, nil)
	if err == nil {
		t.Fatal("unknown verb did not fail closed")
	}
	if !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("Run error lost the sentinel: %v", err)
	}
	if _, ok := core.AsRefusal(err); !ok {
		t.Fatalf("Run returned an untyped refusal: %v", err)
	}
}
