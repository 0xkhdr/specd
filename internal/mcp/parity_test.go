package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestMCPParity(t *testing.T) {
	tools := CoreTools()
	if len(tools) == 0 {
		t.Fatal("expected tools")
	}
	seen := map[string]bool{}
	for _, tool := range tools {
		seen[tool.Name] = true
		if core.ForbiddenTool(tool.Name) {
			t.Fatalf("forbidden tool exposed: %s", tool.Name)
		}
	}
	for _, operation := range core.Operations {
		if core.ForbiddenTool(operation.Command) {
			continue
		}
		if !seen[operation.ID] {
			t.Fatalf("operation missing from MCP tools: %s", operation.ID)
		}
	}

	var out bytes.Buffer
	err := Serve(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"), &out, tools, nil)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	var response Response
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatalf("response json: %v", err)
	}
	if response.Error != nil || response.Result == nil {
		t.Fatalf("bad response: %#v", response)
	}
}

func TestOperationEffectParity(t *testing.T) {
	tools := map[string]Tool{}
	for _, tool := range CoreTools() {
		tools[tool.Name] = tool
	}
	for _, id := range []string{"eval.import", "archive", "link", "new", "recurring.record", "spike", "unlink"} {
		tool, ok := tools[id]
		if !ok {
			t.Fatalf("operation %q missing", id)
		}
		if tool.Effect == core.EffectRead || !tool.Mutable {
			t.Fatalf("operation %q falsely read-only: %+v", id, tool)
		}
	}
	if tool := tools["eval.status"]; tool.Effect != core.EffectRead || tool.Mutable {
		t.Fatalf("eval.status effect mismatch: %+v", tool)
	}
}

func TestParityAgentsGuideDoctorRoute(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range CoreTools() {
		if tool.Name != "agents.guide" && tool.Name != "agents.doctor" {
			continue
		}
		props := tool.InputSchema["properties"].(map[string]any)
		if _, ok := props["args"]; !ok {
			t.Fatalf("%s MCP tool lacks positional route", tool.Name)
		}
		seen[tool.Name] = true
	}
	if !seen["agents.guide"] || !seen["agents.doctor"] {
		t.Fatalf("agents MCP operations missing: %v", seen)
	}
}

// TestDenyList pins the MCP deny list itself (R2.1): the named human-gate and
// host-only verbs must be absent from tools/list AND refused by tools/call.
// Removing any entry from core.ForbiddenTool breaks CI here at both layers.
func TestDenyList(t *testing.T) {
	denied := []string{"approve", "init", "mcp", "brain"}

	listed := map[string]bool{}
	for _, tool := range CoreTools() {
		listed[tool.Name] = true
	}
	for _, name := range denied {
		if listed[name] {
			t.Fatalf("tools/list must exclude %q", name)
		}
		resp := Dispatch(Request{
			JSONRPC: "2.0", ID: 1, Method: "tools/call",
			Params: []byte(`{"name":"` + name + `"}`),
		}, CoreTools(), nil)
		wantCode := -32001
		if name == "approve" {
			wantCode = MCPHandoffRequiredCode
		}
		if resp.Error == nil || resp.Error.Code != wantCode {
			t.Fatalf("tools/call %q: want policy error -32001, got %#v", name, resp.Error)
		}
	}
}

func TestBrainToolsGatedByConfig(t *testing.T) {
	if got := BrainTools(core.Config{}); len(got) != 0 {
		t.Fatalf("brain tools should be disabled: %#v", got)
	}
	if got := BrainTools(core.Config{Orchestration: core.OrchestrationConfig{Enabled: true}}); len(got) != 1 {
		t.Fatalf("brain tools should be enabled: %#v", got)
	}
}

func TestParityTypedHandoffBaseline(t *testing.T) {
	resp := Dispatch(Request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: []byte(`{"name":"approve"}`)}, CoreTools(), nil)
	if resp.Error == nil || resp.Error.Code != MCPHandoffRequiredCode {
		t.Fatalf("forbidden mutation must return typed handoff: %#v", resp.Error)
	}
}

func TestDriverConformanceHostParity(t *testing.T) {
	// MCP exposes same read/driver contract as CLI; transport adds no authority.
	for _, tool := range CoreTools() {
		if tool.Name == "approve" || tool.Name == "task" {
			t.Fatalf("human/evidence lifecycle verb leaked into MCP: %s", tool.Name)
		}
	}
	if len(CoreTools()) != len(core.ManifestToolContracts()) {
		t.Fatalf("MCP/CLI tool contract count diverged")
	}
}

func TestRemoteEnvelopeMissingPinFailsClosed(t *testing.T) {
	d := core.DispatchV1{ProtocolVersion: core.DriverProtocolVersion, Root: "/repo", SpecSlug: "demo", TaskID: "T1", Role: "craftsman", Verify: "printf ok", ContextRef: "ctx", ConfigDigest: "cfg", PaletteDigest: "pal", AuthorityRef: "auth", SubjectHead: "head"}
	if err := core.ValidateDispatchV1(d); err == nil {
		t.Fatal("missing context pin silently accepted")
	}
}

func TestGuidanceDispatchParity(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range CoreTools() {
		seen[tool.Name] = true
		decision := core.ProjectRoute(tool.Name, core.RouteContext{
			Transport: core.RouteMCP, Phase: core.PhaseExecute, Actor: tool.Actor, Authority: core.RouteAuthorityAvailable,
		})
		if !decision.Executable {
			t.Errorf("MCP advertises unreachable tool %s: %+v", tool.Name, decision)
		}
	}
	for _, operation := range core.Operations {
		decision := core.ProjectRoute(operation.ID, core.RouteContext{
			Transport: core.RouteMCP, Phase: core.PhaseExecute, Actor: operation.Actor, Authority: core.RouteAuthorityAvailable,
		})
		if decision.Executable && !seen[operation.ID] {
			t.Errorf("MCP omits executable route %s", operation.ID)
		}
	}
}
