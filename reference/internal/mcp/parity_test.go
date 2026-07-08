package mcp

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestCLIMCPParity asserts the command-mirror MCP surface is generated from the
// optimized CLI survivor set: every non-hidden, non-server/meta command has one
// specd_<command> tool, and no hidden/retired command is advertised. Extra
// semantic/composite MCP helpers are ignored here because they are not 1:1 CLI
// mirrors.
func TestCLIMCPParity(t *testing.T) {
	want := map[string]bool{}
	knownCommands := map[string]bool{}
	for _, c := range core.Commands {
		knownCommands[c.Command] = true
		if c.Hidden || metaCommands[c.Command] {
			continue
		}
		want[toolPrefix+c.Command] = true
	}

	got := map[string]bool{}
	for _, tl := range buildTools(nil) {
		if !strings.HasPrefix(tl.Name, toolPrefix) {
			continue
		}
		cmd := strings.TrimPrefix(tl.Name, toolPrefix)
		if !knownCommands[cmd] {
			continue
		}
		got[tl.Name] = true
	}

	if diff := symmetricDiff(want, got); len(diff) > 0 {
		t.Fatalf("CLI↔MCP command parity mismatch: %s", strings.Join(diff, ", "))
	}
}

func TestToolListGoldenParity(t *testing.T) {
	type snapshot struct {
		Name        string     `json:"name"`
		InputSchema jsonSchema `json:"inputSchema"`
	}
	got := make([]snapshot, 0, len(buildTools(nil)))
	for _, tl := range buildTools(nil) {
		got = append(got, snapshot{Name: tl.Name, InputSchema: tl.InputSchema})
	}

	if os.Getenv("UPDATE_GOLDEN") != "" {
		gb, _ := json.MarshalIndent(got, "", "  ")
		if err := os.WriteFile("testdata/tools_golden.json", append(gb, '\n'), 0o644); err != nil {
			t.Fatalf("write golden tool fixture: %v", err)
		}
		return
	}

	body, err := os.ReadFile("testdata/tools_golden.json")
	if err != nil {
		t.Fatalf("read golden tool fixture: %v", err)
	}
	var want []snapshot
	if err := json.Unmarshal(body, &want); err != nil {
		t.Fatalf("parse golden tool fixture: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		gb, _ := json.MarshalIndent(got, "", "  ")
		t.Fatalf("MCP tool list changed; update testdata/tools_golden.json only for intentional API changes\n%s", gb)
	}
}

func symmetricDiff(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, "missing in MCP: "+k)
		}
	}
	for k := range b {
		if !a[k] {
			out = append(out, "extra in MCP: "+k)
		}
	}
	sort.Strings(out)
	return out
}
