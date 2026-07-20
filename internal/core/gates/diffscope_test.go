package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core/scope"
)

func change(kind, path string) scope.Change { return scope.Change{Kind: kind, Path: path} }

// baseInput is a clean, in-scope mission: every rule below is checked by
// perturbing exactly one thing about it.
func baseInput() DiffScopeInput {
	return DiffScopeInput{
		TaskID:             "T1",
		Baseline:           "abc123",
		Changes:            []scope.Change{change("M", "internal/core/thing.go")},
		DeclaredPaths:      []string{"internal/core/thing.go"},
		BaselineIsAncestor: true,
		BaselineResolvable: true,
	}
}

func TestDiffScopeAcceptsDeclaredChange(t *testing.T) {
	if findings := CheckDiffScope(baseInput()); len(findings) != 0 {
		t.Fatalf("declared change refused: %+v", findings)
	}
}

// R4.1: undeclared modify, create, delete, and rename all refuse.
func TestDiffScopeRejectsUndeclaredChanges(t *testing.T) {
	cases := []struct {
		name   string
		change scope.Change
		want   string
	}{
		{"undeclared modify", change("M", "internal/core/other.go"), "modified"},
		{"undeclared create", change("A", "internal/core/new.go"), "created"},
		{"undeclared delete", change("D", "internal/core/gone.go"), "deleted"},
		{"undeclared untracked", change("untracked", "scratch.txt"), "created, untracked"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := baseInput()
			input.Changes = []scope.Change{tc.change}
			findings := CheckDiffScope(input)
			if len(findings) != 1 {
				t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
			}
			if !strings.Contains(findings[0].Message, tc.change.Path) {
				t.Errorf("finding does not name the offending path: %q", findings[0].Message)
			}
			if !strings.Contains(findings[0].Message, tc.want) {
				t.Errorf("finding does not name the change kind %q: %q", tc.want, findings[0].Message)
			}
			if findings[0].Severity != Error {
				t.Errorf("severity = %q, want error", findings[0].Severity)
			}
		})
	}
}

// R4.1: a rename is an undeclared write at whichever end is undeclared, so both
// ends are checked. Renaming a declared file out to an undeclared path is the
// case a one-ended check would miss.
func TestDiffScopeRejectsRenameAtEitherEnd(t *testing.T) {
	t.Run("rename_to_undeclared_path", func(t *testing.T) {
		input := baseInput()
		input.Changes = []scope.Change{{Kind: "R", Path: "internal/core/moved.go", PreviousPath: "internal/core/thing.go"}}
		findings := CheckDiffScope(input)
		if len(findings) == 0 {
			t.Fatal("rename to an undeclared path was accepted")
		}
	})
	t.Run("rename_from_undeclared_path", func(t *testing.T) {
		input := baseInput()
		input.Changes = []scope.Change{{Kind: "R", Path: "internal/core/thing.go", PreviousPath: "internal/core/sneaky.go"}}
		findings := CheckDiffScope(input)
		if len(findings) != 1 {
			t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "sneaky.go") {
			t.Errorf("finding does not name the undeclared source: %q", findings[0].Message)
		}
	})
	t.Run("rename_fully_declared", func(t *testing.T) {
		input := baseInput()
		input.DeclaredPaths = []string{"internal/core/thing.go", "internal/core/moved.go"}
		input.Changes = []scope.Change{{Kind: "R", Path: "internal/core/moved.go", PreviousPath: "internal/core/thing.go"}}
		if findings := CheckDiffScope(input); len(findings) != 0 {
			t.Fatalf("fully declared rename refused: %+v", findings)
		}
	})
}

// R4.2: work planned against history the worktree no longer has.
func TestDiffScopeRejectsPreBaselineChange(t *testing.T) {
	input := baseInput()
	input.BaselineIsAncestor = false
	findings := CheckDiffScope(input)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "predate") {
		t.Errorf("finding does not explain the drift: %q", findings[0].Message)
	}
}

// An unresolvable baseline stops the check rather than reporting violations
// measured against a reference point that does not exist.
func TestDiffScopeUnresolvableBaselineReportsOnlyItself(t *testing.T) {
	input := baseInput()
	input.BaselineResolvable = false
	input.Changes = []scope.Change{change("M", "wildly/undeclared.go")}
	findings := CheckDiffScope(input)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want exactly the baseline finding: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "does not resolve") {
		t.Errorf("finding does not name the unresolvable baseline: %q", findings[0].Message)
	}
	if !strings.Contains(findings[0].Message, "fresh mission") {
		t.Errorf("finding does not name the recovery: %q", findings[0].Message)
	}
}

// R4.3: harness-owned state is never in scope. This is the rule core.DeriveDiff
// would silently disable, since it strips .specd/ before returning.
func TestDiffScopeRejectsHarnessOwnedState(t *testing.T) {
	cases := []string{
		".specd/specs/demo/tasks.md",
		".specd/specs/demo/requirements.md",
		".specd/specs/demo/design.md",
		".specd/roles/craftsman.md",
		".specd/steering/workflow.md",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			input := baseInput()
			input.Changes = []scope.Change{change("M", path)}
			findings := CheckDiffScope(input)
			if len(findings) != 1 {
				t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
			}
			if !strings.Contains(findings[0].Message, "harness-owned") {
				t.Errorf("finding does not name the rule: %q", findings[0].Message)
			}
		})
	}
}

// R4.3: declaring a .specd path must not legalize editing it. A task cannot
// grant itself authority over harness state by listing it.
func TestDiffScopeHarnessStateRefusedEvenWhenDeclared(t *testing.T) {
	input := baseInput()
	input.DeclaredPaths = []string{".specd/specs/demo/tasks.md"}
	input.Changes = []scope.Change{change("M", ".specd/specs/demo/tasks.md")}
	findings := CheckDiffScope(input)
	if len(findings) != 1 {
		t.Fatalf("declaring harness state legalized editing it: %+v", findings)
	}
	if !strings.Contains(findings[0].Message, "harness-owned") {
		t.Errorf("wrong rule fired: %q", findings[0].Message)
	}
}

// R4.4: two missions authorized to write the same path race, and the loser's
// work is silently overwritten.
func TestDiffScopeRejectsActiveLeaseOverlap(t *testing.T) {
	input := baseInput()
	input.OtherLeaseScopes = map[string][]string{"lease-2": {"internal/core/thing.go"}}
	findings := CheckDiffScope(input)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "lease-2") {
		t.Errorf("finding does not name the conflicting lease: %q", findings[0].Message)
	}
}

func TestDiffScopeAllowsNonOverlappingLease(t *testing.T) {
	input := baseInput()
	input.OtherLeaseScopes = map[string][]string{"lease-2": {"internal/cmd/elsewhere.go"}}
	if findings := CheckDiffScope(input); len(findings) != 0 {
		t.Fatalf("non-overlapping lease refused: %+v", findings)
	}
}

// Determinism: the same diff must report the same findings in the same order,
// including when the lease map iterates differently.
func TestDiffScopeIsDeterministic(t *testing.T) {
	input := baseInput()
	input.Changes = []scope.Change{
		change("M", "b/undeclared.go"),
		change("A", "a/undeclared.go"),
		change("M", ".specd/specs/demo/tasks.md"),
	}
	input.OtherLeaseScopes = map[string][]string{
		"lease-9": {"internal/core/thing.go"},
		"lease-2": {"internal/core/thing.go"},
	}
	first := CheckDiffScope(input)
	for i := 0; i < 50; i++ {
		next := CheckDiffScope(input)
		if len(next) != len(first) {
			t.Fatalf("finding count varies between runs: %d then %d", len(first), len(next))
		}
		for j := range first {
			if first[j].Message != next[j].Message {
				t.Fatalf("finding order varies at %d:\n%q\n%q", j, first[j].Message, next[j].Message)
			}
		}
	}
}

// The check only ever refuses: it has no path that returns "satisfied", so no
// bypass can be built from it.
func TestDiffScopeNeverSatisfiesOnlyRefuses(t *testing.T) {
	input := baseInput()
	input.Changes = nil
	if findings := CheckDiffScope(input); len(findings) != 0 {
		t.Fatalf("empty diff produced findings: %+v", findings)
	}
	// Every finding this gate can emit is an error. A warn or info would let a
	// caller treat a scope violation as advisory.
	input = baseInput()
	input.Changes = []scope.Change{change("M", "undeclared.go"), change("M", ".specd/specs/demo/tasks.md")}
	input.BaselineIsAncestor = false
	for _, finding := range CheckDiffScope(input) {
		if finding.Severity != Error {
			t.Fatalf("finding %q has severity %q; scope violations are never advisory", finding.Message, finding.Severity)
		}
		if finding.Gate != "diffscope" {
			t.Errorf("finding not attributed to diffscope: %+v", finding)
		}
	}
}

// The harness writes its own runtime ledgers into .specd/ during a normal task:
// verify appends evidence, session open writes the session file. Those are not
// agent edits and must not be reported as undeclared, or the loop refuses its
// own bookkeeping.
func TestDiffScopeIgnoresHarnessRuntimeState(t *testing.T) {
	runtime := []string{
		".specd/specs/demo/state.json",
		".specd/specs/demo/evidence.jsonl",
		".specd/specs/demo/driver-session.json",
		".specd/specs/demo/runs.jsonl",
		".specd/specs/demo/evals/run.json",
	}
	for _, path := range runtime {
		t.Run(path, func(t *testing.T) {
			input := baseInput()
			input.Changes = []scope.Change{change("M", path)}
			if findings := CheckDiffScope(input); len(findings) != 0 {
				t.Fatalf("harness runtime write reported as a violation: %+v", findings)
			}
		})
	}
}
