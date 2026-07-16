package mcp

import (
	"reflect"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// TestToolsCallExecutes pins that tools/call routes the MCP arguments object to
// the injected executor — positional operands under "args", everything else as
// flags — and wraps the captured output in MCP content. This is the documented
// use case (an agent invoking a verb as a tool) actually working, not the old
// "tool not implemented" stub.
func TestToolsCallExecutes(t *testing.T) {
	var gotName string
	var gotArgs []string
	var gotFlags map[string]string
	exec := func(name string, args []string, flags map[string]string, _ *core.AuthorityV1, _ time.Time) (string, error) {
		gotName, gotArgs, gotFlags = name, args, flags
		return "frontier:\nT1\n", nil
	}
	resp := Dispatch(Request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: []byte(`{"name":"next","arguments":{"args":["payments"],"json":true}}`),
	}, CoreTools(), exec)

	if resp.Error != nil {
		t.Fatalf("tools/call returned error: %+v", resp.Error)
	}
	if gotName != "next" {
		t.Errorf("executor name = %q, want next", gotName)
	}
	if !reflect.DeepEqual(gotArgs, []string{"payments"}) {
		t.Errorf("executor args = %v, want [payments]", gotArgs)
	}
	if gotFlags["json"] != "true" {
		t.Errorf("executor flags = %v, want json=true", gotFlags)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok || result["isError"] != false {
		t.Fatalf("result = %#v, want map with isError=false", resp.Result)
	}
}

// TestToolsCallVerbErrorIsToolError pins that a verb failure surfaces as a
// tool-level error (isError=true in the result), not a JSON-RPC protocol error.
func TestToolsCallVerbErrorIsToolError(t *testing.T) {
	exec := func(string, []string, map[string]string, *core.AuthorityV1, time.Time) (string, error) {
		return "", errVerbFailed
	}
	resp := Dispatch(Request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: []byte(`{"name":"status","arguments":{"args":["nope"]}}`),
	}, CoreTools(), exec)
	if resp.Error != nil {
		t.Fatalf("want tool-level error in result, got JSON-RPC error %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	if result["isError"] != true {
		t.Fatalf("result isError = %v, want true", result["isError"])
	}
}

var errVerbFailed = &verbErr{}

type verbErr struct{}

func (*verbErr) Error() string { return "verb failed" }
