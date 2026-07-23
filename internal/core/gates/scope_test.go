package gates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestScopeRejectsOutsideDeclaredFiles(t *testing.T) {
	if err := CheckScope([]string{"a.go"}, []string{"a.go", "a_test.go"}); err != nil {
		t.Fatal(err)
	}
	if err := CheckScope([]string{"a.go", "x.go"}, []string{"a.go"}); err == nil {
		t.Fatal("outside scope accepted")
	}
}

// TestAcceptanceReachabilityRefOutsideDeclared pins spec R5.1: when a cited
// requirement id is referenced in the repo only outside the row's declared
// files, the gate warns, naming the id and the referencing file; a row that
// declares the referencing file passes.
func TestAcceptanceReachabilityRefOutsideDeclared(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "impl.go"), []byte("package impl\n// implements spec R9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	arm := string(core.StatusTasks)

	outside := []core.TaskRow{{ID: "T1", Kind: "feature", Files: "other.go", Refs: []string{"R9.9"}}}
	f := acceptanceReach(CheckCtx{ApproveTarget: arm, Root: root, Tasks: outside})
	if len(f) != 1 || f[0].Severity != Warn {
		t.Fatalf("R5.1: expected one warn, got %+v", f)
	}
	for _, want := range []string{"T1", "R9.9", "impl.go"} {
		if !strings.Contains(f[0].Message, want) {
			t.Fatalf("R5.1 finding %q missing %q", f[0].Message, want)
		}
	}

	// Declaring the referencing file makes the id reachable → no warn.
	inScope := []core.TaskRow{{ID: "T2", Kind: "feature", Files: "impl.go", Refs: []string{"R9.9"}}}
	if f := acceptanceReach(CheckCtx{ApproveTarget: arm, Root: root, Tasks: inScope}); len(f) != 0 {
		t.Fatalf("R5.1: reachable id must not warn: %+v", f)
	}
}

// TestAcceptanceReachabilityScopeVsAcceptance pins spec R5.2: a production-kind
// row whose acceptance names a Go path no declared file can produce yields a
// distinct scope-versus-acceptance error; a same-directory declared file
// satisfies it.
func TestAcceptanceReachabilityScopeVsAcceptance(t *testing.T) {
	arm := string(core.StatusTasks)

	bad := []core.TaskRow{{ID: "T1", Kind: "feature", Files: "internal/other/x.go", Acceptance: "behaviour lives in internal/foo/bar.go"}}
	f := acceptanceReach(CheckCtx{ApproveTarget: arm, Tasks: bad})
	if !HasErrors(f) {
		t.Fatalf("R5.2: scope-vs-acceptance not raised: %+v", f)
	}
	for _, want := range []string{"T1", "scope-vs-acceptance", "internal/foo/bar.go"} {
		if !strings.Contains(f[0].Message, want) {
			t.Fatalf("R5.2 finding %q missing %q", f[0].Message, want)
		}
	}

	// A declared file in the same package can produce it → no error.
	ok := []core.TaskRow{{ID: "T2", Kind: "feature", Files: "internal/foo/other.go", Acceptance: "behaviour lives in internal/foo/bar.go"}}
	if f := acceptanceReach(CheckCtx{ApproveTarget: arm, Tasks: ok}); HasErrors(f) {
		t.Fatalf("R5.2: same-package declared file must satisfy acceptance: %+v", f)
	}

	// Non-production kind carries no obligation.
	docs := []core.TaskRow{{ID: "T3", Kind: "docs", Files: "README.md", Acceptance: "see internal/foo/bar.go"}}
	if f := acceptanceReach(CheckCtx{ApproveTarget: arm, Tasks: docs}); HasErrors(f) {
		t.Fatalf("R5.2: non-production kind must not error: %+v", f)
	}
}

// TestAcceptanceReachabilityCleanAndParity pins that a fully in-scope plan and
// an empty CheckCtx both yield nothing.
func TestAcceptanceReachabilityCleanAndParity(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "impl.go"), []byte("package impl\n// spec R9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	clean := []core.TaskRow{{ID: "T1", Kind: "feature", Files: "impl.go", Refs: []string{"R9.9"}, Acceptance: "implement impl.go"}}
	if f := acceptanceReach(CheckCtx{ApproveTarget: string(core.StatusTasks), Root: root, Tasks: clean}); len(f) != 0 {
		t.Fatalf("clean plan produced findings: %+v", f)
	}
	if f := acceptanceReach(CheckCtx{}); len(f) != 0 {
		t.Fatalf("empty CheckCtx produced findings: %+v", f)
	}
}
