package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// TestSurfaceMatchesADR guards doc/code drift: the CLI surface must match the
// command palette in docs/command-reference.md exactly, and the deferred `pinky`
// worker must carry no reachable surface. If someone wires a verb without
// documenting it (or documents one that is not registered), this fails.
func TestSurfaceMatchesADR(t *testing.T) {
	// F11 regression guard: the deferred worker CLI is unregistered — dispatch
	// must fail closed, never no-op.
	if err := Run(t.TempDir(), "pinky", nil, map[string]string{}); !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("pinky must be unregistered surface, got err=%v", err)
	}

	// The command reference is the authoritative verb palette and claims to match
	// the registry exactly. Pin both directions.
	documented := paletteVerbs(t)
	registry := map[string]bool{}
	for _, name := range RegisteredCommandNames() {
		registry[name] = true
		if !documented[name] {
			t.Fatalf("registered verb %q missing from docs/command-reference.md palette", name)
		}
	}
	for name := range documented {
		if !registry[name] {
			t.Fatalf("command-reference.md documents verb %q not in the registry", name)
		}
	}
}

// paletteVerbs parses the `specd <verb>` entries from the command-reference palette.
func paletteVerbs(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(commandReferencePath(t))
	if err != nil {
		t.Fatalf("read command reference: %v", err)
	}
	// Verb entries are H3 headings: ### `verb`
	row := regexp.MustCompile("(?m)^### `([a-z]+)`\\s*$")
	verbs := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(string(raw), -1) {
		verbs[m[1]] = true
	}
	if len(verbs) == 0 {
		t.Fatal("no verbs parsed from command-reference palette")
	}
	return verbs
}

func commandReferencePath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	// internal/cmd/surface_test.go -> repo root -> docs/command-reference.md
	return filepath.Join(filepath.Dir(file), "..", "..", "docs", "command-reference.md")
}
