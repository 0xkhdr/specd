package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// TestStdlibOnly enforces the hard invariant that specd has zero runtime
// dependencies (spec acceptance: "go list -deps shows no new module"). Because
// go.mod is the single source of module requirements and the build is offline,
// any third-party import the MCP server pulled in would have to appear here. A
// require directive other than the module's own line fails the build's contract.
func TestStdlibOnly(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}

	// Match both single-line `require x v1` and `require ( ... )` block forms.
	blockRE := regexp.MustCompile(`(?s)require\s*\((.*?)\)`)
	if m := blockRE.FindStringSubmatch(string(data)); m != nil {
		for _, line := range strings.Split(m[1], "\n") {
			if line = strings.TrimSpace(line); line != "" {
				t.Errorf("unexpected external dependency in go.mod require block: %q", line)
			}
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "require ") {
			t.Errorf("unexpected external dependency: %q", line)
		}
	}
}
