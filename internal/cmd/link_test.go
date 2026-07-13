package cmd

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// newSpecInProject scaffolds an additional spec (with a trivially-passing task)
// inside an already-initialized project root.
func newSpecInProject(t *testing.T, root, slug string) {
	t.Helper()
	if err := Run(root, "new", []string{slug}, nil); err != nil {
		t.Fatalf("new %s: %v", slug, err)
	}
	writeTasks(t, root, slug, "| T1 | scout | spec.md | - | true | ok |")
	authorDemoSpec(t, root, slug)
}

// completeSpec drives a spec all the way to complete: through the approval
// gates, a passing verify, and task completion.
func completeSpec(t *testing.T, root, slug string) {
	t.Helper()
	for _, gate := range []string{"requirements", "design", "executing"} {
		if err := Run(root, "approve", []string{slug, gate}, nil); err != nil {
			t.Fatalf("approve %s %s: %v", slug, gate, err)
		}
	}
	if err := Run(root, "verify", []string{slug, "T1"}, nil); err != nil {
		t.Fatalf("verify %s: %v", slug, err)
	}
	if err := Run(root, "task", []string{"complete", slug, "T1"}, nil); err != nil {
		t.Fatalf("complete %s: %v", slug, err)
	}
}

func TestLinkCycleRefusedAndEnforced(t *testing.T) {
	root := newDemoSpec(t) // inits the project and a "demo" spec (unused here)
	gitInitRepo(t, root)
	for _, slug := range []string{"a", "b", "c"} {
		newSpecInProject(t, root, slug)
	}

	// R1: link a→b, b→c.
	if err := Run(root, "link", []string{"a", "b"}, nil); err != nil {
		t.Fatalf("link a b: %v", err)
	}
	if err := Run(root, "link", []string{"b", "c"}, nil); err != nil {
		t.Fatalf("link b c: %v", err)
	}

	// R1: linking to a nonexistent slug fails closed (exit 2).
	if err := Run(root, "link", []string{"a", "ghost"}, nil); err == nil {
		t.Fatal("link to unknown slug must fail")
	}

	// R2: c→a would close the cycle a→b→c→a; refused with the path printed.
	err := Run(root, "link", []string{"c", "a"}, nil)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle link should be refused with a cycle path, got %v", err)
	}
	if !strings.Contains(err.Error(), "c → a → b → c") {
		t.Fatalf("cycle path not printed: %v", err)
	}

	// R4: with nothing complete, only c (no deps) is on the program frontier.
	out, err := captureStdout(t, func() error { return Run(root, "status", nil, map[string]string{"program": ""}) })
	if err != nil {
		t.Fatalf("status --program: %v", err)
	}
	if !strings.Contains(out, "program frontier (actionable now): c") {
		t.Fatalf("frontier should be {c}:\n%s", out)
	}

	// R5: a cannot advance into execution while b is incomplete.
	err = Run(root, "approve", []string{"a", "executing"}, nil)
	if err == nil || !strings.Contains(err.Error(), "b") {
		t.Fatalf("approve a executing should be blocked naming b, got %v", err)
	}

	// Completing the dependency chain unblocks a.
	completeSpec(t, root, "c")
	completeSpec(t, root, "b")
	if err := Run(root, "approve", []string{"a", "executing"}, nil); err != nil {
		t.Fatalf("approve a executing after deps complete: %v", err)
	}
}

func TestLinkKind(t *testing.T) {
	root := newDemoSpec(t)
	for _, slug := range []string{"original", "successor"} {
		newSpecInProject(t, root, slug)
	}
	flags := map[string]string{"kind": "supersedes", "reason": "replace obsolete contract"}
	if err := Run(root, "link", []string{"successor", "original"}, flags); err != nil {
		t.Fatalf("typed link: %v", err)
	}
	program, err := core.LoadProgram(core.ProgramPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Links) != 1 {
		t.Fatalf("links = %+v", program.Links)
	}
	link := program.Links[0]
	if link.From != "successor" || link.To != "original" || link.Kind != core.LinkKindSupersedes || link.Reason != flags["reason"] {
		t.Fatalf("typed source trace lost: %+v", link)
	}
	if err := Run(root, "link", []string{"original", "successor"}, map[string]string{"kind": "invalid"}); err == nil {
		t.Fatal("unknown link kind must fail closed")
	}
}

func TestUnlinkRemovesEnforcement(t *testing.T) {
	root := newDemoSpec(t)
	gitInitRepo(t, root)
	newSpecInProject(t, root, "a")
	newSpecInProject(t, root, "b")

	if err := Run(root, "link", []string{"a", "b"}, nil); err != nil {
		t.Fatalf("link: %v", err)
	}
	// Blocked while b is incomplete.
	if err := Run(root, "approve", []string{"a", "executing"}, nil); err == nil {
		t.Fatal("a should be blocked by b")
	}
	// R3: unlink removes the enforcement; a may now execute.
	if err := Run(root, "unlink", []string{"a", "b"}, nil); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if err := Run(root, "approve", []string{"a", "executing"}, nil); err != nil {
		t.Fatalf("approve after unlink: %v", err)
	}
	// R3: removing a nonexistent link fails closed.
	if err := Run(root, "unlink", []string{"a", "b"}, nil); err == nil {
		t.Fatal("removing a nonexistent link must fail")
	}
	// Sanity: program.json is a real file, not tucked into a spec's state.
	if _, err := core.LoadProgram(core.ProgramPath(root)); err != nil {
		t.Fatalf("load program: %v", err)
	}
}
