package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// listToolsOverServer runs the real stdio Serve loop with cfg and returns the
// advertised tool names from tools/list, exercising the full route → buildTools
// plumbing rather than buildTools in isolation (spec §7 integration).
func listToolsOverServer(t *testing.T, cfg *core.Config) []string {
	t.Helper()
	noop := func(string, cli.Args) (int, bool) { return core.ExitOK, true }
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	}, "\n") + "\n"

	var out strings.Builder
	if err := Serve(strings.NewReader(input), &out, noop, cfg); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var resp struct {
			ID     int `json:"id"`
			Result struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("bad response %q: %v", line, err)
		}
		if resp.ID == 2 {
			for _, tl := range resp.Result.Tools {
				names = append(names, tl.Name)
			}
		}
	}
	return names
}

// TestServerToolsListFiltered asserts tools/list honours each config variant
// end-to-end over the wire (spec AC1–AC5).
func TestServerToolsListFiltered(t *testing.T) {
	t.Run("absent block matches full set", func(t *testing.T) {
		got := len(listToolsOverServer(t, &core.Config{}))
		want := len(buildTools(nil))
		if got != want {
			t.Errorf("absent-block tool count = %d, want %d", got, want)
		}
	})

	t.Run("essential default set is eight", func(t *testing.T) {
		got := listToolsOverServer(t, &core.Config{MCP: core.MCPConfig{Expose: "essential"}})
		if len(got) != len(defaultEssentialTools) {
			t.Errorf("essential count = %d, want %d (%v)", len(got), len(defaultEssentialTools), got)
		}
	})

	t.Run("orchestration enabled exposes brain surface", func(t *testing.T) {
		got := listToolsOverServer(t, &core.Config{
			Orchestration: core.OrchestrationCfg{Enabled: true},
			MCP:           core.MCPConfig{Expose: "all"},
		})
		has := map[string]bool{}
		for _, n := range got {
			has[n] = true
		}
		if !has["specd_brain"] || !has["brain_orchestrate"] {
			t.Errorf("orchestration tools missing over wire: %v", got)
		}
	})
}
