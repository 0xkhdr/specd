package mcp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newHTTPServer must apply the A1 slow-client read/idle bounds and must NOT set
// a server-wide WriteTimeout — a global write bound would silently sever the
// long-lived /sse stream (R1, R3.1). /rpc carries its own per-response deadline.
func TestHTTPServerTimeoutBounds(t *testing.T) {
	srv := newHTTPServer("127.0.0.1:0", http.NewServeMux())
	if srv.ReadHeaderTimeout != httpReadHeaderTimeout {
		t.Errorf("ReadHeaderTimeout = %v, want %v", srv.ReadHeaderTimeout, httpReadHeaderTimeout)
	}
	if srv.ReadTimeout != httpReadTimeout {
		t.Errorf("ReadTimeout = %v, want %v", srv.ReadTimeout, httpReadTimeout)
	}
	if srv.IdleTimeout != httpIdleTimeout {
		t.Errorf("IdleTimeout = %v, want %v", srv.IdleTimeout, httpIdleTimeout)
	}
	if srv.WriteTimeout != 0 {
		t.Errorf("WriteTimeout = %v, want 0 (a global write bound would sever /sse)", srv.WriteTimeout)
	}
}

// The per-response write deadline on /rpc and the deadline-clear on /sse must
// not regress normal request handling (R2, R3). A live server exercises both
// paths end-to-end so the ResponseController calls are real, not recorder no-ops.
func TestHTTPDeadlinePlumbingDoesNotBreakDispatch(t *testing.T) {
	srv := httptest.NewServer(httpHandler(nil, nil, nil, ""))
	defer srv.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	for _, path := range []string{"/rpc", "/sse"} {
		resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		out, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s status = %d, want 200", path, resp.StatusCode)
		}
		if !strings.Contains(string(out), `"jsonrpc"`) {
			t.Errorf("%s body missing jsonrpc envelope: %q", path, out)
		}
	}
}
