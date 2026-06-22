package cmd_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestAgentsMDSubcommandsExist asserts every brain/pinky subcommand named in the
// AGENTS.md anchor is actually dispatched by the CLI (a `case "<sub>":` in the
// corresponding command file). This keeps the anchor's command list from drifting
// away from the real CLI surface.
func TestAgentsMDSubcommandsExist(t *testing.T) {
	agents, err := core.ReadTemplate("AGENTS.md")
	if err != nil {
		t.Fatalf("ReadTemplate(AGENTS.md): %v", err)
	}

	for _, tc := range []struct{ verb, marker, src string }{
		{"brain", "specd brain <", "brain.go"},
		{"pinky", "specd pinky <", "pinky.go"},
	} {
		subs := pipeListAfter(t, agents, tc.marker)
		if len(subs) == 0 {
			t.Fatalf("%s: no subcommand list found after %q in AGENTS.md", tc.verb, tc.marker)
		}
		src, err := os.ReadFile(tc.src)
		if err != nil {
			t.Fatalf("read %s: %v", tc.src, err)
		}
		for _, sub := range subs {
			if !strings.Contains(string(src), fmt.Sprintf("case %q:", sub)) {
				t.Errorf("AGENTS.md lists `specd %s %s` but %s has no `case %q:`", tc.verb, sub, tc.src, sub)
			}
		}
	}
}

// pipeListAfter extracts the `<a|b|c>` list that follows marker in text.
func pipeListAfter(t *testing.T, text, marker string) []string {
	t.Helper()
	i := strings.Index(text, marker)
	if i < 0 {
		return nil
	}
	rest := text[i+len(marker):]
	end := strings.IndexByte(rest, '>')
	if end < 0 {
		return nil
	}
	return strings.Split(rest[:end], "|")
}
