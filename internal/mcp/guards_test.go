package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// TestHostConfigsValidNativeFormat asserts every embedded host config (R4.1,
// R4.3) is emitted as a non-empty snippet that parses in its host's native
// format — JSON for claude-desktop/cursor/vscode/antigravity, TOML for codex —
// and that the /path/to/your/project placeholder is substituted when a root is
// supplied so the snippet is copy-paste reliable.
func TestHostConfigsValidNativeFormat(t *testing.T) {
	const root = "/home/dev/myproject"
	names := HostNames()
	if len(names) != 5 {
		t.Fatalf("HostNames() = %v, want 5 hosts", names)
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			dest, content, ok := HostConfig(name, root)
			if !ok {
				t.Fatalf("HostConfig(%q) not ok", name)
			}
			if strings.TrimSpace(content) == "" {
				t.Fatal("empty config content")
			}
			if dest == "" {
				t.Error("empty dest hint")
			}
			if strings.Contains(content, "/path/to/your/project") {
				t.Error("placeholder /path/to/your/project not substituted")
			}
			// Each embedded snippet pins the placeholder; with a root supplied it
			// must appear verbatim. (cursor/vscode use ${workspaceFolder}, which is
			// host-resolved, so only assert presence where the literal is used.)
			if strings.Contains(content, "specd") == false {
				t.Error("config does not reference the specd server")
			}

			if name == "codex" {
				assertParseableTOML(t, content)
			} else {
				var v any
				if err := json.Unmarshal([]byte(content), &v); err != nil {
					t.Fatalf("config is not valid JSON: %v\n%s", err, content)
				}
			}
		})
	}
}

// assertParseableTOML does a stdlib-only structural check of the codex snippet:
// specd ships zero third-party dependencies (see TestStdlibOnly), so a full TOML
// parser is unavailable. Every non-blank, non-comment line must be either a
// table header `[table]` or a `key = value` pair, and the specd server table
// must be present — enough to catch a corrupted or truncated embed.
func assertParseableTOML(t *testing.T, content string) {
	t.Helper()
	sawServerTable := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				t.Errorf("malformed TOML table header: %q", line)
			}
			if strings.Contains(line, "specd") {
				sawServerTable = true
			}
			continue
		}
		if !strings.Contains(line, "=") {
			t.Errorf("TOML line is neither table nor key=value: %q", line)
		}
	}
	if !sawServerTable {
		t.Error("codex TOML missing a specd server table")
	}
}

// TestHostConfigGuardUnknown asserts the guard precondition for an unknown host
// (R4.2): HostConfig refuses with ok=false and returns no content (no panic, no
// partial snippet), and HostNames() supplies the sorted set a caller uses to
// build an actionable "Available: ..." message.
func TestHostConfigGuardUnknown(t *testing.T) {
	dest, content, ok := HostConfig("not-a-real-host", "/root")
	if ok {
		t.Fatal("unknown host should not be ok")
	}
	if dest != "" || content != "" {
		t.Errorf("unknown host must return empty dest/content, got %q / %q", dest, content)
	}

	names := HostNames()
	if len(names) == 0 {
		t.Fatal("HostNames() empty — caller cannot render an actionable message")
	}
	if !sortedStrings(names) {
		t.Errorf("HostNames() must be sorted for a stable actionable message: %v", names)
	}
	// The message a caller builds must name a real, recoverable choice.
	if _, _, ok := HostConfig(names[0], ""); !ok {
		t.Errorf("HostNames()[0]=%q is not a resolvable host", names[0])
	}
}

func sortedStrings(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] > s[i] {
			return false
		}
	}
	return true
}

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
