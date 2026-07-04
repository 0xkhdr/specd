package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// TestSurfaceMatchesADR enforces W2/R2.3: the CLI surface matches the charter
// (docs/charter.md) exactly, and the deferred `pinky` worker carries no
// reachable surface. If someone reintroduces `internal/cmd/pinky.go` and wires a
// verb without recording the superseding ADR row, this fails.
func TestSurfaceMatchesADR(t *testing.T) {
	// F11 regression guard: the deferred worker CLI is unregistered — dispatch
	// must fail closed, never no-op.
	if err := Run(t.TempDir(), "pinky", nil, map[string]string{}); !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("pinky must be unregistered surface, got err=%v", err)
	}

	// The charter is the authoritative verb→component map and claims to match the
	// registry exactly (docs/charter.md header). Pin both directions.
	charter := charterVerbs(t)
	registry := map[string]bool{}
	for _, name := range RegisteredCommandNames() {
		registry[name] = true
		if !charter[name] {
			t.Fatalf("registered verb %q missing from docs/charter.md verb table", name)
		}
	}
	for name := range charter {
		if !registry[name] {
			t.Fatalf("charter lists verb %q not in the registry", name)
		}
	}
}

// charterVerbs parses the backtick-quoted verbs from the charter's verb table.
func charterVerbs(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(charterPath(t))
	if err != nil {
		t.Fatalf("read charter: %v", err)
	}
	// Table rows look like: | `verb` | component | principle |
	row := regexp.MustCompile("(?m)^\\| `([a-z]+)` \\|")
	verbs := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(string(raw), -1) {
		verbs[m[1]] = true
	}
	if len(verbs) == 0 {
		t.Fatal("no verbs parsed from charter table")
	}
	return verbs
}

func charterPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	// internal/cmd/surface_test.go -> repo root -> docs/charter.md
	return filepath.Join(filepath.Dir(file), "..", "..", "docs", "charter.md")
}
