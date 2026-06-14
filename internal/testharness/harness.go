// Package testharness is the first-class testing infrastructure for specd. It
// provides hermetic, deterministic building blocks that every specd test
// consumes: an isolated project root with env + working-directory isolation, a
// fluent SpecBuilder for authoring gate-valid specs, an in-process CommandRunner
// that captures stdout/stderr/exit-code exactly as the CLI would, a StateAsserter
// for state.json/filesystem/git assertions, and a FakeClock for reproducible
// timestamps.
//
// Nothing here touches the network or the user's real git/working-directory
// state beyond t.TempDir, t.Setenv and a chdir restored on cleanup. Because the
// harness mutates process-global state (cwd, os.Stdout/os.Stderr during Run, and
// core.Clock), tests using it MUST NOT call t.Parallel.
package testharness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// Harness is the root context for a single test: an isolated specd project plus
// the deterministic clock installed for its lifetime.
type Harness struct {
	T     *testing.T
	Root  string // absolute project root containing .specd/ (== process cwd)
	Clock *FakeClock
}

// New creates an isolated specd project: a fresh temp root with an empty
// .specd/specs/ tree, the working directory switched into it, hermetic
// environment defaults, and a FakeClock installed over core.Clock. All of it is
// torn down via t.Cleanup. The returned Harness.Root equals the process working
// directory, so it matches whatever the commands resolve via RequireSpecdRoot.
func New(t *testing.T) *Harness {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd", "specs"), 0o755); err != nil {
		t.Fatalf("testharness.New: mkdir .specd/specs: %v", err)
	}

	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("testharness.New: getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("testharness.New: chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWd) })

	// Canonicalise Root to the resolved cwd so path math matches the commands,
	// which derive their root from os.Getwd() (handles /tmp symlinks etc.).
	if resolved, err := os.Getwd(); err == nil {
		root = resolved
	}

	// Hermetic environment: no ANSI colour in captured output, never inherit a
	// stray JSON-mode flag, and keep lock/verify timeouts short so a wedged test
	// fails fast instead of hanging the suite.
	t.Setenv("NO_COLOR", "1")
	t.Setenv("SPECD_JSON", "")
	t.Setenv("SPECD_LOCK_TIMEOUT_MS", "2000")
	t.Setenv("SPECD_LOCK_STALE_MS", "1000")
	t.Setenv("SPECD_VERIFY_TIMEOUT_MS", "5000")

	clock := NewFakeClock()
	t.Cleanup(clock.install())

	return &Harness{T: t, Root: root, Clock: clock}
}

// Init runs the real `specd init` command in the project root, scaffolding
// steering/, roles/, config.json and AGENTS.md. Use it when a test needs role
// prompts, config, or the full project layout; New() alone only creates
// .specd/specs/.
func (h *Harness) Init() Result {
	h.T.Helper()
	res := h.Run("init")
	if res.Code != core.ExitOK {
		h.T.Fatalf("Init: specd init failed (code %d): %s", res.Code, res.Stderr)
	}
	return res
}

// Path returns an absolute path under the project root for a slash-relative
// path like ".specd/specs/foo/state.json".
func (h *Harness) Path(rel string) string {
	return filepath.Join(h.Root, filepath.FromSlash(rel))
}

// SpecPath returns the absolute path of an artifact (e.g. "requirements.md")
// inside a spec.
func (h *Harness) SpecPath(slug, name string) string {
	return core.ArtifactPath(h.Root, slug, name)
}
