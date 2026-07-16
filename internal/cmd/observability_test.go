package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestContextHUDRendersAndExitDiscipline covers SPEC-06 T-06-03: the operator
// HUD renders from state without error, and the exit-1-vs-2 discipline holds —
// fail-closed rejections wrap ErrUsage (exit 2), real gate/verify failures do
// not (exit 1).
func TestContextHUDRendersAndExitDiscipline(t *testing.T) {
	root := newDemoSpec(t)
	gitInitRepo(t, root)
	advanceToExecuting(t, root)

	out, err := captureStdout(t, func() error {
		return Run(root, "context", []string{"demo", "T1"}, map[string]string{"hud": ""})
	})
	if err != nil {
		t.Fatalf("context --hud: %v", err)
	}
	for _, want := range []string{"mode:", "TOTAL"} {
		if !strings.Contains(out, want) {
			t.Fatalf("HUD missing %q:\n%s", want, out)
		}
	}

	// Exit 2: a fail-closed rejection (unsupported --format) wraps ErrUsage.
	if err := Run(root, "report", []string{"demo"}, map[string]string{"format": "html"}); !errors.Is(err, ErrUsage) {
		t.Fatalf("unsupported --format must wrap ErrUsage (exit 2), got %v", err)
	}

	// Exit 1: complete-task without a passing verify record is a real gate
	// failure — an error that must NOT wrap ErrUsage/ErrUnknownCommand.
	err = Run(root, "complete-task", []string{"demo", "T1"}, nil)
	if err == nil {
		t.Fatal("complete-task without evidence must fail")
	}
	if errors.Is(err, ErrUsage) || errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("gate failure must map to exit 1, not 2: %v", err)
	}
}

// TestErrorMessagesMatchTroubleshootingDocs guards SPEC-06 T-06-03: the CAS and
// sandbox error strings the code actually emits must appear verbatim in
// docs/troubleshooting.md, so the operator doc never drifts from real output.
func TestErrorMessagesMatchTroubleshootingDocs(t *testing.T) {
	doc := readRepoFile(t, "docs", "troubleshooting.md")

	// The CAS sentinel is emitted verbatim; the doc must quote it.
	if !strings.Contains(doc, core.ErrRevisionConflict.Error()) {
		t.Fatalf("troubleshooting.md missing CAS error %q", core.ErrRevisionConflict.Error())
	}
	// The sandbox fail-closed message ("sandbox binary %q unavailable").
	for _, want := range []string{"sandbox binary", "unavailable"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("troubleshooting.md missing sandbox error fragment %q", want)
		}
	}
	// The exit-code legend documents the 0/1/2 discipline this test asserts.
	for _, want := range []string{"`0`", "`1`", "`2`"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("troubleshooting.md missing exit-code %s in legend", want)
		}
	}
}

// readRepoFile reads a repo-root-relative file from a test in internal/cmd.
func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	raw, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	if err != nil {
		t.Fatalf("read %v: %v", parts, err)
	}
	return string(raw)
}
