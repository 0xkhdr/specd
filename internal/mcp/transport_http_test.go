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
	if err := mcp.Serve(strings.NewReader(request+"\n"), &out, cmd.Dispatch); err != nil {
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
	go func() { _ = mcp.ServeHTTP(addr, cmd.Dispatch) }()
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
	go func() { _ = mcp.ServeHTTP(":8799", cmd.Dispatch) }()
	waitReady(t, "127.0.0.1:8799")
	if c, err := net.DialTimeout("tcp", "127.0.0.1:8799", time.Second); err == nil {
		_ = c.Close()
	} else {
		t.Errorf("loopback default not reachable on 127.0.0.1: %v", err)
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
