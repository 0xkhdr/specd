package mcp

import (
	"testing"
)

func TestForbiddenToolsDeniedOnCall(t *testing.T) {
	for _, name := range []string{"approve", "init", "mcp", "brain", "task"} {
		resp := Dispatch(Request{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params:  []byte(`{"name":"` + name + `"}`),
		}, CoreTools())
		if resp.Error == nil {
			t.Fatalf("%s: expected policy error", name)
		}
		if resp.Error.Code != -32001 {
			t.Fatalf("%s: expected policy error code, got %d", name, resp.Error.Code)
		}
	}
}
