package mcp

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestResolveResourceURI pins the URI parser and containment guard (spec R6/AC5):
// only well-formed specd:// URIs for known artifacts/steering files resolve, and
// every traversal or malformed vector is rejected before any read.
func TestResolveResourceURI(t *testing.T) {
	root := "/proj"
	cases := []struct {
		uri      string
		wantOK   bool
		wantName string
	}{
		{"specd://specs/auth/tasks.md", true, "tasks.md"},
		{"specd://specs/auth/state.json", true, "state.json"},
		{"specd://steering/reasoning.md", true, "reasoning.md"},
		{"specd://specs/auth/secret.txt", false, ""},  // unknown artifact
		{"specd://specs/../../etc/passwd", false, ""}, // traversal via slug
		{"specd://specs/auth/../../../etc/passwd", false, ""},
		{"specd://steering/../config.json", false, ""}, // traversal via steering file
		{"specd://steering/passwd", false, ""},         // non-markdown steering
		{"specd://specs/auth", false, ""},              // too few segments
		{"file://specs/auth/tasks.md", false, ""},      // wrong scheme
		{"specd://unknown/auth/tasks.md", false, ""},   // unknown collection
	}
	for _, tc := range cases {
		t.Run(tc.uri, func(t *testing.T) {
			path, name, ok := resolveResourceURI(root, tc.uri)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (path=%q)", ok, tc.wantOK, path)
			}
			if ok {
				if name != tc.wantName {
					t.Errorf("name = %q, want %q", name, tc.wantName)
				}
				if !strings.HasPrefix(path, "/proj/.specd/") {
					t.Errorf("resolved path %q escaped .specd/", path)
				}
			}
		})
	}
}

// TestMimeForName covers the mime inference used by resources/read (spec R4).
func TestMimeForName(t *testing.T) {
	for name, want := range map[string]string{
		"tasks.md":   "text/markdown",
		"state.json": "application/json",
		"notes.txt":  "text/plain",
	} {
		if got := mimeForName(name); got != want {
			t.Errorf("mimeForName(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestArtifactResourceConformance guards spec §5.4: the resource artifact list
// must stay in sync with the canonical core artifact set so list/read never drift
// from what `specd new` writes.
func TestArtifactResourceConformance(t *testing.T) {
	if !isArtifactResource("state.json") {
		t.Error("state.json must be an addressable artifact resource")
	}
	for _, a := range core.Artifacts {
		if !isArtifactResource(a) {
			t.Errorf("core artifact %q missing from resource set", a)
		}
	}
}
