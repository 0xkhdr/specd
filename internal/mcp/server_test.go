package mcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func TestMCPAuthorityDeniesValidatorWrite(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a, err := core.BuildAuthority(core.TaskRow{ID: "T1", Role: "validator", DeclaredFiles: []string{"a.go"}}, "controller", "w", "demo", "execute", "abc", "policy", "required", now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	a.AllowedTools = append(a.AllowedTools, core.ToolAuthority{ID: "review"})
	a.Digest = ""
	core.FinalizeAuthority(&a)
	req := Request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: []byte(`{"name":"review","arguments":{"args":["demo"]}}`)}
	resp := DispatchAuthorized(req, CoreTools(), func(string, []string, map[string]string) (string, error) { t.Fatal("executor called"); return "", nil }, &a, now, "execute")
	if resp.Error == nil || resp.Error.Code != -32001 {
		t.Fatalf("response=%+v", resp)
	}
}

func TestMCPUnknownToolDefaultDenied(t *testing.T) {
	req := Request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: []byte(`{"name":"invented"}`)}
	resp := Dispatch(req, CoreTools(), func(string, []string, map[string]string) (string, error) { return "", nil })
	if resp.Error == nil || resp.Error.Code != -32001 {
		t.Fatalf("response=%+v", resp)
	}
}

func TestMCPTelemetryAnnotationFlags(t *testing.T) {
	for _, toolName := range []string{"verify.task"} {
		tools := CoreTools()
		var found *Tool
		for i := range tools {
			if tools[i].Name == toolName {
				found = &tools[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("tool %s absent", toolName)
		}
		encoded, err := json.Marshal(found.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		raw := string(encoded)
		for _, flag := range []string{"input-tokens", "output-tokens", "cached-tokens", "provider", "model", "currency", "pricing-ref", "telemetry-source", "attestation-ref"} {
			if !strings.Contains(raw, flag) {
				t.Errorf("%s MCP schema missing %s: %s", toolName, flag, raw)
			}
		}
	}
}

func TestMCPCompleteTaskUsesNarrowAuthorizedRoute(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a, err := core.BuildAuthority(core.TaskRow{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"a.go"}}, "controller", "w", "demo", "execute", "abc", "policy", "required", now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	req := Request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: []byte(`{"name":"complete-task","arguments":{"args":["demo","T1"]}}`)}
	called := false
	resp := DispatchAuthorized(req, CoreTools(), func(command string, args []string, flags map[string]string) (string, error) {
		called = true
		if command != "complete-task" || len(args) != 2 || args[0] != "demo" || args[1] != "T1" {
			t.Fatalf("route = %q %v", command, args)
		}
		return "completed demo T1\n", nil
	}, &a, now, "execute")
	if resp.Error != nil || !called {
		t.Fatalf("response = %+v, called=%v", resp, called)
	}
	for _, tool := range CoreTools() {
		if tool.Name == "task.complete" {
			t.Fatal("legacy broad task completion operation exposed")
		}
	}
}
