package mcp_test

import (
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// safeBuffer is a goroutine-safe sink: Serve's request handler and the phase
// watcher both write to it, while the test reads snapshots concurrently.
type safeBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func phaseConfig() *core.Config {
	cfg := core.LoadConfig(".")
	cfg.MCP.Expose = "phase"
	return &cfg
}

func setSpecStatus(t *testing.T, h *th.Harness, slug string, status core.SpecStatus) {
	t.Helper()
	path := h.SpecPath(slug, "state.json")
	raw := core.ReadOrNull(path)
	if raw == nil {
		t.Fatalf("state.json not found at %s", path)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(*raw), &m); err != nil {
		t.Fatalf("unmarshal state.json: %v", err)
	}
	enc, _ := json.Marshal(string(status))
	m["status"] = enc
	out, _ := json.MarshalIndent(m, "", "  ")
	if err := core.AtomicWrite(path, string(out)+"\n"); err != nil {
		t.Fatalf("write state.json: %v", err)
	}
}

// lastToolsList returns the tool names from the last tools/list response found in
// the captured stream, plus how many list_changed notifications it carried.
func parseStream(t *testing.T, raw string) (lastTools map[string]bool, notifications int) {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("stream line not JSON: %q: %v", line, err)
		}
		if m["method"] == "notifications/tools/list_changed" {
			notifications++
			continue
		}
		res, ok := m["result"].(map[string]any)
		if !ok {
			continue
		}
		tools, ok := res["tools"].([]any)
		if !ok {
			continue
		}
		lastTools = map[string]bool{}
		for _, tv := range tools {
			td, _ := tv.(map[string]any)
			if name, _ := td["name"].(string); name != "" {
				lastTools[name] = true
			}
		}
	}
	return lastTools, notifications
}

// TestPhaseModeAdvertisesListChanged checks the capability is on under
// expose:"phase" (spec AC1) end-to-end through Serve.
func TestPhaseModeAdvertisesListChanged(t *testing.T) {
	h := th.New(t)
	seedDesignSpec(h, "auth")

	in, inW := io.Pipe()
	var out safeBuffer
	cfg := phaseConfig()
	done := make(chan struct{})
	go func() { _ = mcp.Serve(in, &out, cmd.Dispatch, cfg); close(done) }()

	_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n")
	waitFor(t, &out, func(s string) bool { return strings.Contains(s, `"protocolVersion"`) })
	_ = inW.Close()
	<-done

	var resp map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		_ = json.Unmarshal([]byte(line), &resp)
		if resp["id"] != nil {
			break
		}
	}
	caps := resp["result"].(map[string]any)["capabilities"].(map[string]any)
	if lc := caps["tools"].(map[string]any)["listChanged"].(bool); !lc {
		t.Error("expose:phase should advertise tools.listChanged:true")
	}
}

// TestPhaseModeNotifiesAndAdaptsList drives design→executing and asserts the host
// receives a list_changed notification and a subsequent tools/list reflects the
// executing subset (spec AC2/AC3).
func TestPhaseModeNotifiesAndAdaptsList(t *testing.T) {
	h := th.New(t)
	seedDesignSpec(h, "auth")

	in, inW := io.Pipe()
	var out safeBuffer
	cfg := phaseConfig()
	done := make(chan struct{})
	go func() { _ = mcp.Serve(in, &out, cmd.Dispatch, cfg); close(done) }()

	// Initial list under the design phase.
	_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n")
	waitFor(t, &out, func(s string) bool { return strings.Contains(s, `"specd_inspect"`) })

	// Transition the spec; the watcher should detect it, swap, and notify.
	setSpecStatus(t, h, "auth", core.StatusExecuting)
	waitFor(t, &out, func(s string) bool {
		return strings.Contains(s, "notifications/tools/list_changed")
	})

	// Re-fetch: the list must now be the executing subset.
	_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`+"\n")
	waitFor(t, &out, func(s string) bool { return strings.Contains(s, `"specd_next"`) })

	_ = inW.Close()
	<-done

	tools, notifications := parseStream(t, out.String())
	if notifications < 1 {
		t.Errorf("expected at least one list_changed notification, got %d", notifications)
	}
	if !tools["specd_next"] {
		t.Errorf("post-transition list missing executing tool specd_next: %v", tools)
	}
	if tools["specd_check"] {
		t.Errorf("post-transition list still exposes planning tool specd_check: %v", tools)
	}
}

func seedDesignSpec(h *th.Harness, slug string) {
	h.Spec(slug).
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusDesign).
		Build()
}

func waitFor(t *testing.T, out *safeBuffer, cond func(string) bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond(out.String()) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within timeout; stream so far:\n%s", out.String())
}
