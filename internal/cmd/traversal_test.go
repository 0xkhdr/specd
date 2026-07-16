package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSlugTraversalRejected pins the security invariant that no spec-resolving
// verb accepts a path-traversal slug. An unvalidated slug like "../../x" would
// escape .specd/specs/ and read or write arbitrary filesystem paths (a real
// risk when an agent chooses the slug). Every guarded verb must fail closed
// with an "invalid slug" error and touch nothing outside the tree.
func TestSlugTraversalRejected(t *testing.T) {
	root := t.TempDir()
	sentinel := filepath.Join(root, "..", "specd_traversal_canary")
	t.Cleanup(func() { os.Remove(sentinel) })

	// Each entry is (verb, args) with args[0] (or the slug position) crafted to
	// escape the specs directory toward the sentinel path.
	esc := "../../specd_traversal_canary"
	cases := []struct {
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
		{"approve", []string{esc, "design"}},
		{"midreq", []string{esc}},
		{"decision", []string{esc}},
		{"link", []string{esc, "other"}},
		{"link", []string{"other", esc}},
		{"complete-task", []string{esc, "T1"}},
	}
	for _, tc := range cases {
		err := Run(root, tc.verb, tc.args, map[string]string{"text": "x"})
		if err == nil {
			t.Errorf("%s %v: expected rejection, got nil", tc.verb, tc.args)
			continue
		}
		if !strings.Contains(err.Error(), "invalid slug") {
			t.Errorf("%s %v: want invalid-slug error, got %v", tc.verb, tc.args, err)
		}
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatalf("traversal slug created %s outside the tree", sentinel)
	}
}
