package core

import (
	"strings"
	"testing"
)

// TestValidateEvalRubricRejectsHostileFields asserts the rubric validator fails
// closed on adversarial input: unknown kinds, path traversal, NUL bytes, bad
// regexes, out-of-range minScore, duplicate/blank IDs, and unknown predicates.
func TestValidateEvalRubricRejectsHostileFields(t *testing.T) {
	cases := []struct {
		name   string
		rubric EvalRubric
		want   string
	}{
		{"unknown kind", EvalRubric{Checks: []EvalCheck{{ID: "a", Kind: "spawn"}}}, "unknown kind"},
		{"path traversal", EvalRubric{Checks: []EvalCheck{{ID: "a", Kind: "regex", Pattern: "x", Path: "../../etc/passwd"}}}, "must stay under"},
		{"absolute path", EvalRubric{Checks: []EvalCheck{{ID: "a", Kind: "regex", Pattern: "x", Path: "/etc/passwd"}}}, "relative path"},
		{"nul in command", EvalRubric{Checks: []EvalCheck{{ID: "a", Kind: "command", Command: "echo\x00hi"}}}, "NUL byte"},
		{"bad regex", EvalRubric{Checks: []EvalCheck{{ID: "a", Kind: "regex", Path: "spec.md", Pattern: "("}}}, "invalid regex"},
		{"minScore high", EvalRubric{MinScore: 2, Checks: []EvalCheck{{ID: "a", Kind: "command", Command: "true"}}}, "minScore"},
		{"blank id", EvalRubric{Checks: []EvalCheck{{ID: "", Kind: "command", Command: "true"}}}, "id must match"},
		{"dup id", EvalRubric{Checks: []EvalCheck{{ID: "a", Kind: "command", Command: "true"}, {ID: "a", Kind: "command", Command: "true"}}}, "duplicate"},
		{"bad predicate", EvalRubric{Checks: []EvalCheck{{ID: "a", Kind: "trajectory", Predicate: "haxx"}}}, "unknown trajectory predicate"},
		{"bad sandbox", EvalRubric{Checks: []EvalCheck{{ID: "a", Kind: "command", Command: "true", Sandbox: "docker"}}}, "sandbox must be"},
		{"no checks", EvalRubric{}, "at least one check"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.rubric
			err := ValidateEvalRubric(&r)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

// TestRunEvalScorePurity asserts scoring is a pure function of the rubric and
// artifacts: two runs over identical inputs produce byte-identical scores and
// check verdicts (invariant 7). Only the timestamp may differ, and here the
// FakeClock is not installed so we compare the score fields, not GeneratedAt.
func TestRunEvalScorePurity(t *testing.T) {
	root := t.TempDir()
	slug := "demo"
	writeSpecFile(t, root, slug, "spec.md", "hello world token\n")
	rubric := &EvalRubric{
		MinScore: 0.5,
		Checks: []EvalCheck{
			{ID: "has-token", Kind: "regex", Path: "spec.md", Pattern: "token", Weight: 1},
			{ID: "missing", Kind: "regex", Path: "spec.md", Pattern: "nope", Weight: 1},
		},
	}
	if err := ValidateEvalRubric(rubric); err != nil {
		t.Fatal(err)
	}
	a, err := RunEval(root, slug, rubric, "sha256:x")
	if err != nil {
		t.Fatal(err)
	}
	b, err := RunEval(root, slug, rubric, "sha256:x")
	if err != nil {
		t.Fatal(err)
	}
	if a.Score != b.Score {
		t.Fatalf("non-deterministic score: %v vs %v", a.Score, b.Score)
	}
	if a.Score != 0.5 {
		t.Fatalf("score = %v, want 0.5 (1 of 2 equal-weight checks pass)", a.Score)
	}
	if !a.Passed {
		t.Fatalf("expected pass at minScore 0.5")
	}
	if len(a.Failures) != 1 || a.Failures[0] != "missing" {
		t.Fatalf("failures = %v, want [missing]", a.Failures)
	}
}

// TestEvalRubricSkeletonTransform asserts eval init's compile step is a
// deterministic, interpretation-free transform: one stub per acceptance
// criterion, IDs derived from the criterion IDs, count preserved.
func TestEvalRubricSkeletonTransform(t *testing.T) {
	req := `## Requirement 1: Auth
**Acceptance criteria:**
1. The system shall authenticate users.
2. When a token expires, the system shall reject it.

## Requirement 2: Logging
**Acceptance criteria:**
1. The system shall log every request.
`
	sk := EvalRubricSkeleton(req)
	if len(sk.Checks) != 3 {
		t.Fatalf("want 3 stub checks, got %d", len(sk.Checks))
	}
	wantIDs := []string{"crit-1-1", "crit-1-2", "crit-2-1"}
	for i, want := range wantIDs {
		if sk.Checks[i].ID != want {
			t.Errorf("check %d id = %q, want %q", i, sk.Checks[i].ID, want)
		}
		if sk.Checks[i].Kind != "regex" {
			t.Errorf("check %d kind = %q, want regex", i, sk.Checks[i].Kind)
		}
	}
	// The skeleton must itself validate (a valid, refinable rubric).
	if err := ValidateEvalRubric(sk); err != nil {
		t.Fatalf("skeleton does not validate: %v", err)
	}
}

func writeSpecFile(t *testing.T, root, slug, name, body string) {
	t.Helper()
	path := ArtifactPath(root, slug, name)
	if err := AtomicWrite(path, body); err != nil {
		t.Fatal(err)
	}
}

// FuzzLoadEvalRubric asserts the rubric parser never panics on arbitrary bytes.
func FuzzLoadEvalRubric(f *testing.F) {
	f.Add([]byte(`{"checks":[{"id":"a","kind":"command","command":"true"}]}`))
	f.Add([]byte(`{"minScore":0.5,"checks":[]}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(`{"checks":[{"id":"a","kind":"regex","path":"../x","pattern":"("}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		root := t.TempDir()
		path := ArtifactPath(root, "fz", "eval-rubric.json")
		if err := AtomicWrite(path, string(data)); err != nil {
			t.Skip()
		}
		// Must not panic; error or success are both fine.
		_, _, _ = LoadEvalRubric(path)
	})
}
