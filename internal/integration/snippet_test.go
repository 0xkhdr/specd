package integration

import (
	"strings"
	"testing"
)

func TestSnippet(t *testing.T) {
	got := Snippet("codex", "demo", "T1")
	for _, want := range []string{"codex", "specd context demo T1 --json", "specd verify demo T1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Snippet() = %q, missing %q", got, want)
		}
	}
}

func TestSnippetFallback(t *testing.T) {
	got := NewRegistry().Snippet("unknown", "demo", "T1")
	if !strings.Contains(got, "unknown") || !strings.Contains(got, "specd context demo T1 --json") {
		t.Fatalf("fallback snippet = %q", got)
	}
}

func TestSnippetCompiles(t *testing.T) {
	got := Snippet("", "demo", "T1")
	if !strings.Contains(got, "agent") || !strings.Contains(got, "specd verify demo T1") {
		t.Fatalf("snippet = %q", got)
	}
}
