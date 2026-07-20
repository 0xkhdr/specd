package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestSlugTraversalRejected pins the security invariant that no spec-resolving
// verb accepts a path-traversal slug. An unvalidated slug like "../../x" would
// escape .specd/specs/ and read or write arbitrary filesystem paths (a real
// risk when an agent chooses the slug). Every guarded verb must fail closed and
// touch nothing outside the tree.
//
// The case list is derived from core.Commands rather than hand-maintained:
// slug validation lives in ~29 individual call sites, not in the path builders
// (StatePath, EvidencePath, SpecMemoryPath and friends take a slug and
// filepath.Join it with no check), so the sink cannot catch a verb whose author
// forgets. Enumerating the palette means a spec-taking verb is covered the
// moment it is declared.
func TestSlugTraversalRejected(t *testing.T) {
	const canary = "specd_traversal_canary"

	for _, command := range core.Commands {
		if command.Deferred || !takesSpecSlug(command) {
			continue
		}
		t.Run(command.Name, func(t *testing.T) {
			// The slug position varies by verb and the palette does not record
			// it uniformly (SpecSlugArg is set only on phase-enforced verbs),
			// so probe every positional slot a slug could occupy.
			for slot := 0; slot < 3; slot++ {
				root := t.TempDir()
				sentinel := filepath.Join(root, "..", canary)
				os.Remove(sentinel)

				args := make([]string, slot+2)
				for i := range args {
					args[i] = "T1"
				}
				args[slot] = "../../" + canary

				if err := Run(root, command.Name, args, map[string]string{"text": "x"}); err == nil {
					t.Fatalf("%s accepted a traversal slug at position %d", command.Name, slot)
				}

				// The security property, independent of which refusal fired: an
				// arity or usage rejection is equally safe, so long as it wrote
				// nothing outside the tree.
				if _, err := os.Stat(sentinel); err == nil {
					os.Remove(sentinel)
					t.Fatalf("%s created %s outside the tree (position %d)", command.Name, sentinel, slot)
				}
			}
		})
	}
}

// takesSpecSlug reports whether a verb resolves a spec from a positional
// argument. There is no single palette field for this: SpecSlugArg covers only
// phase-enforced verbs, and the usage strings spell the same argument
// <spec>, <slug>, <from-slug>, or <new-spec>. Both signals are read so a new
// verb is covered however it declares itself.
func takesSpecSlug(command core.Command) bool {
	if command.SpecSlugArg != nil {
		return true
	}
	for _, token := range []string{"<spec>", "<slug>", "<from-slug>", "<to-slug>", "<new-spec>"} {
		if strings.Contains(command.Usage, token) {
			return true
		}
	}
	return false
}

// TestSlugTraversalRejectedForSlugReason guards the gap the palette sweep
// cannot see: a verb that rejects a traversal slug only incidentally, because
// some other check fires first. If that check is ever reordered or relaxed, the
// traversal reaches a path builder. These verbs must refuse on the slug itself.
func TestSlugTraversalRejectedForSlugReason(t *testing.T) {
	esc := "../../specd_traversal_canary"
	for _, tc := range []struct {
		verb string
		args []string
	}{
		{"status", []string{esc}},
		{"check", []string{esc}},
		{"report", []string{esc}},
		{"memory", []string{esc, "add"}},
		{"verify", []string{esc, "T1"}},
		{"next", []string{esc}},
		{"context", []string{esc, "T1"}},
		{"review", []string{esc}},
		{"submit", []string{esc}},
		{"approve", []string{esc}},
		{"midreq", []string{esc}},
		{"decision", []string{esc}},
		{"link", []string{esc, "other"}},
		{"link", []string{"other", esc}},
		{"complete-task", []string{esc, "T1"}},
	} {
		t.Run(tc.verb, func(t *testing.T) {
			err := Run(t.TempDir(), tc.verb, tc.args, map[string]string{"text": "x"})
			if err == nil {
				t.Fatalf("%s %v: expected rejection, got nil", tc.verb, tc.args)
			}
			refusal, ok := core.AsRefusal(err)
			if ok && refusal.Code == "SPEC_INVALID" {
				return
			}
			if !strings.Contains(err.Error(), "invalid slug") {
				t.Fatalf("%s %v: want invalid-slug rejection, got %v", tc.verb, tc.args, err)
			}
		})
	}
}
