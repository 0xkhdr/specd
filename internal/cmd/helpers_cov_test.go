package cmd

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// helpers_cov_test.go covers the pure command-layer helpers — status mapping,
// frontier messaging, remediation fallback, spec-node lookup, the confirm
// prompt, and the pinky arg parsers — that the command integration tests reach
// only incidentally.

func TestStatusWord(t *testing.T) {
	cases := map[string]string{
		"":     "changed",
		"A":    "added",
		"M":    "modified",
		"D":    "deleted",
		"R100": "renamed",
		"C075": "copied",
		"??":   "??",
		"xyz":  "xyz",
	}
	for code, want := range cases {
		if got := statusWord(code); got != want {
			t.Errorf("statusWord(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestFrontierStuckReason(t *testing.T) {
	if got := frontierStuckReason(core.NextResult{Kind: core.NextAllComplete}, "all done"); got != "all done" {
		t.Errorf("complete = %q", got)
	}
	if got := frontierStuckReason(core.NextResult{Kind: core.NextAllBlocked, Blocked: []string{"T1"}}, ""); !strings.Contains(got, "blocked") {
		t.Errorf("blocked = %q", got)
	}
	if got := frontierStuckReason(core.NextResult{Kind: core.NextWaiting, Blocking: []string{"T2"}}, ""); !strings.Contains(got, "waiting") {
		t.Errorf("waiting = %q", got)
	}
	if got := frontierStuckReason(core.NextResult{Kind: core.NextTask}, ""); got != "" {
		t.Errorf("task kind should be empty, got %q", got)
	}
}

func TestFirstRemediation(t *testing.T) {
	if got := firstRemediation(nil); !strings.Contains(got, "doctor") {
		t.Errorf("empty remediation fallback = %q", got)
	}
	if got := firstRemediation([]string{"fix this", "then that"}); got != "fix this" {
		t.Errorf("first remediation = %q", got)
	}
}

func TestFindSpecNode(t *testing.T) {
	specs := []core.SpecNode{{Slug: "a"}, {Slug: "b"}}
	if got := findSpecNode(specs, "b"); got == nil || got.Slug != "b" {
		t.Errorf("findSpecNode(b) = %#v", got)
	}
	if got := findSpecNode(specs, "missing"); got != nil {
		t.Errorf("findSpecNode(missing) = %#v, want nil", got)
	}
}

func TestConfirm(t *testing.T) {
	for _, in := range []string{"y\n", "yes\n", "  YES \n"} {
		if !confirm(strings.NewReader(in), "ok? ") {
			t.Errorf("confirm(%q) should be true", in)
		}
	}
	for _, in := range []string{"n\n", "no\n", "", "maybe\n"} {
		if confirm(strings.NewReader(in), "ok? ") {
			t.Errorf("confirm(%q) should be false", in)
		}
	}
}

func TestPinkyArgParsers(t *testing.T) {
	// Lease: needs session + worker + positive attempt.
	if _, _, _, ok := pinkyLeaseArgs(cli.ParseArgs([]string{"--session", "s", "--worker", "w", "--attempt", "1"})); !ok {
		t.Error("valid lease args should parse")
	}
	if _, _, _, ok := pinkyLeaseArgs(cli.ParseArgs([]string{"--session", "s"})); ok {
		t.Error("lease args missing worker/attempt should fail")
	}

	// Progress: full set + bounded percent.
	full := []string{"--session", "s", "--worker", "w", "--spec", "sp", "--task", "T1", "--message", "m", "--attempt", "1", "--percent", "50"}
	if rep, ok := pinkyProgressArgs(cli.ParseArgs(full)); !ok || rep.Percent != 50 {
		t.Errorf("valid progress args = %#v / %v", rep, ok)
	}
	if _, ok := pinkyProgressArgs(cli.ParseArgs([]string{"--session", "s"})); ok {
		t.Error("incomplete progress args should fail")
	}

	// Block: full set required.
	blk := []string{"--session", "s", "--worker", "w", "--spec", "sp", "--task", "T1", "--reason", "r", "--attempt", "1"}
	if rep, ok := pinkyBlockArgs(cli.ParseArgs(blk)); !ok || rep.Reason != "r" {
		t.Errorf("valid block args = %#v / %v", rep, ok)
	}
	if _, ok := pinkyBlockArgs(cli.ParseArgs([]string{"--worker", "w"})); ok {
		t.Error("incomplete block args should fail")
	}

	// Query: full set required.
	qry := []string{"--session", "s", "--worker", "w", "--spec", "sp", "--task", "T1", "--text", "q", "--attempt", "1"}
	if rep, ok := pinkyQueryArgs(cli.ParseArgs(qry)); !ok || rep.Text != "q" {
		t.Errorf("valid query args = %#v / %v", rep, ok)
	}
	if _, ok := pinkyQueryArgs(cli.ParseArgs([]string{"--text", "q"})); ok {
		t.Error("incomplete query args should fail")
	}
}
