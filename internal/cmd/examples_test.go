package cmd

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// readOnlyVerbs are the queryable verbs whose documented examples must run green
// verbatim against a real, executing spec. Mutating/lifecycle examples (init,
// new, approve, verify→complete, brain, submit…) are exercised end-to-end by
// TestLifecycleE2E; this test guards the read surface — the examples an operator
// is most likely to copy-paste — against verb/flag drift.
var readOnlyVerbs = map[string]bool{
	"help": true, "version": true, "status": true, "check": true,
	"report": true, "context": true, "next": true, "handshake": true,
}

// TestDocumentedExamplesRun covers SPEC-07 T-07-01: every concrete (placeholder-
// free) example declared in the command palette (the SPEC-02 feature-map SOT)
// dispatches and succeeds against a fresh `specd init`'d, executing project. A
// stale example that names a removed flag or verb fails here.
func TestDocumentedExamplesRun(t *testing.T) {
	root := newDemoSpec(t)
	gitInitRepo(t, root)
	advanceToExecuting(t, root)
	// Give report/status a completed task so telemetry/coverage examples render.
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("verify T1: %v", err)
	}
	if err := Run(root, "complete-task", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("complete T1: %v", err)
	}

	ran := 0
	for _, cmd := range core.Commands {
		if !readOnlyVerbs[cmd.Name] {
			continue
		}
		for _, ex := range cmd.Examples {
			if !isConcreteExample(ex) {
				continue // a usage synopsis, not a runnable invocation
			}
			parsed, err := cli.ParseArgs(strings.Fields(ex)[1:]) // drop "specd", parse verbatim
			if err != nil {
				t.Errorf("documented example %q does not parse: %v", ex, err)
				continue
			}
			if parsed.Command != cmd.Name {
				continue // an alt-form line under another verb's entry
			}
			args := normaliseOperands(parsed.Pos)
			if _, err := captureStdout(t, func() error { return Run(root, parsed.Command, args, parsed.Flags) }); err != nil {
				t.Errorf("documented example %q failed: %v", ex, err)
			}
			ran++
		}
	}
	if ran == 0 {
		t.Fatal("no documented examples were exercised — parser or filter is broken")
	}
}

// isConcreteExample keeps only copy-pasteable invocations: no placeholder
// (`<>` / `[]`) and no pipe alternation.
func isConcreteExample(ex string) bool {
	if !strings.HasPrefix(ex, "specd ") {
		return false
	}
	return !strings.ContainsAny(ex, "<>[]|")
}

// normaliseOperands rewrites the docs' canonical slug/task operands onto the
// demo spec this test drives, so the example runs against real state.
func normaliseOperands(pos []string) []string {
	out := make([]string, 0, len(pos))
	for _, tok := range pos {
		switch tok {
		case "payments":
			out = append(out, "demo")
		case "T3":
			out = append(out, "T1")
		default:
			out = append(out, tok)
		}
	}
	return out
}
