package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// captureStderr runs fn with os.Stderr redirected to a pipe and returns what was
// written. core.Error writes straight to os.Stderr, so this is the only way to
// observe the deprecation warning a legacy alias emits.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	_ = w.Close()
	os.Stderr = orig
	return <-done
}

// TestLegacyAliasSunset is the merge-sunset guard (optimization-plan Action 1.3).
// It asserts every deprecated runtime alias (a) has a recorded removal version
// and a named survivor home and (b) emits a deprecation warning naming both —
// so no hidden alias can ship without a bounded grace period and a migration
// nudge. This is the test that lets the gates measure the kitchen (hidden
// aliases), not just the visible palette menu.
func TestLegacyAliasSunset(t *testing.T) {
	if len(legacyAliases) == 0 {
		t.Fatal("legacyAliases is empty — the sunset guard has nothing to protect")
	}

	inRegistry := map[string]bool{}
	for _, c := range Registry {
		inRegistry[c.Name] = true
	}

	for name, meta := range legacyAliases {
		t.Run(name, func(t *testing.T) {
			// (b-pre) every alias must be hidden from the live palette.
			if inRegistry[name] {
				t.Errorf("legacy alias %q is also a live Registry command — it is not actually deprecated", name)
			}

			// (a) a recorded removal version, shaped like a version tag.
			if !strings.HasPrefix(meta.removedIn, "v") {
				t.Errorf("alias %q removedIn = %q, want a version like v0.2.0", name, meta.removedIn)
			}
			if meta.home == "" {
				t.Errorf("alias %q has no survivor home named for the warning", name)
			}

			// (b) the rendered warning names the command, the home, and the
			// removal version.
			msg := deprecationMessage(name, meta)
			for _, want := range []string{"deprecated", name, meta.home, meta.removedIn} {
				if !strings.Contains(msg, want) {
					t.Errorf("warning %q missing %q", msg, want)
				}
			}

			// functional/terminal invariant: a functional alias delegates to a
			// real handler; a terminal one has none and exits non-zero.
			if meta.functional && meta.run == nil {
				t.Errorf("alias %q is functional but has a nil handler", name)
			}
			if !meta.functional && meta.run != nil {
				t.Errorf("alias %q is terminal but carries a handler", name)
			}

			// Terminal aliases need no .specd root to run, so we can assert the
			// observable contract directly: a stderr warning and a non-zero exit.
			if !meta.functional {
				run, ok := legacyAlias(name)
				if !ok {
					t.Fatalf("legacyAlias(%q) reported not found", name)
				}
				var code int
				stderr := captureStderr(t, func() { code = run(cli.Args{}) })
				if code == core.ExitOK {
					t.Errorf("terminal alias %q exited 0, want non-zero", name)
				}
				if !strings.Contains(stderr, name) {
					t.Errorf("terminal alias %q wrote no deprecation warning to stderr; got %q", name, stderr)
				}
			}
		})
	}
}
