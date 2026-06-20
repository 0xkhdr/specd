package mcp_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// stdioResult drives one JSON-RPC request through the stdio Serve loop and
// returns its parsed result object — the reference the HTTP adapter must match.
func stdioResult(t *testing.T, request string) map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := mcp.Serve(strings.NewReader(request+"\n"), &out, cmd.Dispatch, nil); err != nil {
		t.Fatalf("stdio Serve: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("stdio response not JSON: %q: %v", out.String(), err)
	}
	return resultOf(t, resp)
}

func resultOf(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	r, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("response has no result object: %v", resp)
	}
	return r
}

// freePort reserves an ephemeral loopback port and releases it, returning the
// address for ServeHTTP to re-bind. Good enough for a single-process test.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// httpResult POSTs one JSON-RPC request to the running adapter and returns the
// parsed result object.
func httpResult(t *testing.T, base, path, request string) map[string]any {
	t.Helper()
	resp, err := http.Post(base+path, "application/json", strings.NewReader(request))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// /sse frames the JSON-RPC response as a `data: ...` event; unwrap it.
	body = bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(body), []byte("data:")))
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("HTTP response not JSON: %q: %v", string(body), err)
	}
	return resultOf(t, m)
}

// TestHTTPTransportParity asserts the opt-in HTTP adapter is a faithful second
// front door onto the same dispatch as stdio: tools/list is identical over both
// transports, and a tools/call specd_status returns the same result (R6, R7).
// The stdio reference is gathered BEFORE the HTTP server starts because callTool
// swaps the process-global os.Stdout — the two paths must never run concurrently.
func TestHTTPTransportParity(t *testing.T) {
	h := th.New(t)
	h.Spec("auth").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()

	const listReq = `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	const statusReq = `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"specd_status","arguments":{"args":["auth"]}}}`

	// 1. Reference results over stdio (sequential, before any concurrent stdout swap).
	wantTools := stdioResult(t, listReq)
	wantStatus := stdioResult(t, statusReq)

	// 2. Start the HTTP adapter on a free loopback port.
	addr := freePort(t)
	go func() { _ = mcp.ServeHTTP(addr, cmd.Dispatch, nil) }()
	base := "http://" + addr
	waitReady(t, addr)

	// 3a. tools/list over HTTP must equal stdio's tools/list (R6.2).
	gotTools := httpResult(t, base, "/rpc", listReq)
	if !reflect.DeepEqual(gotTools["tools"], wantTools["tools"]) {
		t.Errorf("HTTP tools/list != stdio tools/list\n http: %v\nstdio: %v", gotTools["tools"], wantTools["tools"])
	}

	// 3b. tools/call specd_status over HTTP must equal the stdio result.
	gotStatus := httpResult(t, base, "/rpc", statusReq)
	if !reflect.DeepEqual(gotStatus, wantStatus) {
		t.Errorf("HTTP status call != stdio\n http: %v\nstdio: %v", gotStatus, wantStatus)
	}

	// 3c. The /sse endpoint exposes the identical dispatch, one event frame.
	sseTools := httpResult(t, base, "/sse", listReq)
	if !reflect.DeepEqual(sseTools["tools"], wantTools["tools"]) {
		t.Errorf("SSE tools/list != stdio tools/list")
	}
}

// TestHTTPLoopbackDefault asserts a bare port binds loopback, never a public
// interface, so spec contents stay on-host by default (R4.2).
func TestHTTPLoopbackDefault(t *testing.T) {
	go func() { _ = mcp.ServeHTTP(":8799", cmd.Dispatch, nil) }()
	waitReady(t, "127.0.0.1:8799")
	if c, err := net.DialTimeout("tcp", "127.0.0.1:8799", time.Second); err == nil {
		_ = c.Close()
	} else {
		t.Errorf("loopback default not reachable on 127.0.0.1: %v", err)
	}
}

// TestHTTPTransportMatrix runs a set of read-only tools over both stdio and
// HTTP /rpc and asserts byte-for-byte equal results, so transport is an
// operational, not behavioral, choice across more than a single tool (R3.1).
func TestHTTPTransportMatrix(t *testing.T) {
	h := th.New(t)
	h.Spec("matrix").
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Build()

	reqs := map[string]string{
		"status": `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"specd_status","arguments":{"args":["matrix"]}}}`,
		"waves":  `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"specd_waves","arguments":{"args":["matrix"]}}}`,
		"check":  `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"specd_check","arguments":{"args":["matrix"]}}}`,
	}

	// Gather every stdio reference BEFORE the HTTP server starts: callTool swaps
	// the process-global os.Stdout, so the two paths must never run concurrently.
	want := map[string]map[string]any{}
	for name, req := range reqs {
		want[name] = stdioResult(t, req)
	}

	addr := freePort(t)
	go func() { _ = mcp.ServeHTTP(addr, cmd.Dispatch, nil) }()
	base := "http://" + addr
	waitReady(t, addr)

	for name, req := range reqs {
		got := httpResult(t, base, "/rpc", req)
		if !reflect.DeepEqual(got, want[name]) {
			t.Errorf("tool %s: HTTP result != stdio\n http: %v\nstdio: %v", name, got, want[name])
		}
	}
}

// TestHTTPMalformedRequest covers R3.3: a malformed request gets a proper HTTP
// + MCP error rather than a crash. A wrong method is a 405; an unparseable JSON
// body is a 200 carrying a JSON-RPC parse-error (-32700) envelope, the MCP
// contract for transport-level vs protocol-level failures.
func TestHTTPMalformedRequest(t *testing.T) {
	addr := freePort(t)
	go func() { _ = mcp.ServeHTTP(addr, cmd.Dispatch, nil) }()
	base := "http://" + addr
	waitReady(t, addr)

	t.Run("wrong_method_is_405", func(t *testing.T) {
		resp, err := http.Get(base + "/rpc")
		if err != nil {
			t.Fatalf("GET /rpc: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("GET /rpc status = %d, want 405", resp.StatusCode)
		}
	})

	t.Run("garbage_body_is_mcp_parse_error", func(t *testing.T) {
		resp, err := http.Post(base+"/rpc", "application/json", strings.NewReader("{not valid json"))
		if err != nil {
			t.Fatalf("POST /rpc: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200 (MCP error envelope)", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("response not JSON: %q: %v", string(body), err)
		}
		e, ok := m["error"].(map[string]any)
		if !ok {
			t.Fatalf("expected JSON-RPC error envelope, got %v", m)
		}
		if code := e["code"].(float64); code != -32700 {
			t.Errorf("error code = %v, want -32700 (parse error)", code)
		}
	})
}

func TestMCPHTTPMalformedOversizedRequest(t *testing.T) {
	addr := freePort(t)
	go func() { _ = mcp.ServeHTTP(addr, cmd.Dispatch, nil) }()
	base := "http://" + addr
	waitReady(t, addr)

	tooLarge := strings.Repeat("x", 1<<20+1)
	for _, path := range []string{"/rpc", "/sse"} {
		t.Run(path, func(t *testing.T) {
			resp, err := http.Post(base+path, "application/json", strings.NewReader(tooLarge))
			if err != nil {
				t.Fatalf("POST %s: %v", path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200 MCP error envelope", resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			body = bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(body), []byte("data:")))
			var m map[string]any
			if err := json.Unmarshal(body, &m); err != nil {
				t.Fatalf("response not JSON: %q: %v", string(body), err)
			}
			e, ok := m["error"].(map[string]any)
			if !ok {
				t.Fatalf("expected JSON-RPC error envelope, got %v", m)
			}
			if code := e["code"].(float64); code != -32600 {
				t.Errorf("error code = %v, want -32600", code)
			}
			if !strings.Contains(e["message"].(string), "message exceeds") {
				t.Errorf("error message = %q, want size diagnostic", e["message"])
			}
		})
	}
}

// TestSSEFraming asserts the /sse endpoint emits MCP-compliant SSE framing
// (R3.2): a text/event-stream content type and a body framed as `data: <json>`
// terminated by a blank line, with the unwrapped payload being the same
// JSON-RPC result the /rpc endpoint returns.
func TestSSEFraming(t *testing.T) {
	addr := freePort(t)
	go func() { _ = mcp.ServeHTTP(addr, cmd.Dispatch, nil) }()
	base := "http://" + addr
	waitReady(t, addr)

	const listReq = `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	resp, err := http.Post(base+"/sse", "application/json", strings.NewReader(listReq))
	if err != nil {
		t.Fatalf("POST /sse: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	raw, _ := io.ReadAll(resp.Body)
	body := string(raw)
	if !strings.HasPrefix(body, "data: ") {
		t.Errorf("SSE body must start with 'data: ', got %q", body)
	}
	if !strings.HasSuffix(body, "\n\n") {
		t.Errorf("SSE event must terminate with a blank line, got %q", body)
	}
	// The unwrapped data payload must itself be a valid JSON-RPC result.
	payload := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(body), "data:"))
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("SSE data payload not JSON: %q: %v", payload, err)
	}
	if _, ok := m["result"]; !ok {
		t.Errorf("SSE payload missing result: %v", m)
	}
}

// waitReady blocks until the server accepts connections or the deadline passes.
func waitReady(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s never became ready", addr)
}
