package mcp

import (
	"bytes"
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// mcpTokenEnv names the optional bearer token. When set, /rpc and /sse require a
// matching `Authorization: Bearer <token>` header (A2 R3).
const mcpTokenEnv = "SPECD_MCP_TOKEN"

// stdErr holds the package-level reference to os.Stderr captured at startup
// to prevent data races during concurrent test execution when os.Stderr is swapped.
var stdErr io.Writer = os.Stderr

// HTTP transport timeout bounds (A1). Kept as named constants so the slow-client
// posture is discoverable and tunable without code archaeology.
//
//   - httpReadHeaderTimeout bounds the request-header read (Slowloris guard).
//   - httpReadTimeout bounds the full request read, body included.
//   - httpIdleTimeout bounds keep-alive idle time between requests.
//   - httpRPCWriteTimeout bounds a single /rpc response write. It is applied
//     per-response (not as a server-wide WriteTimeout) so the long-lived /sse
//     stream is never severed by it.
const (
	httpReadHeaderTimeout = 10 * time.Second
	httpReadTimeout       = 60 * time.Second
	httpIdleTimeout       = 60 * time.Second
	httpRPCWriteTimeout   = 60 * time.Second
)

// ServeHTTP exposes the same JSON-RPC dispatch as the stdio Serve loop over an
// opt-in HTTP transport (R4). It is a second front door onto the identical
// request router — it adds transport, never business logic:
//
//   - POST /rpc : a single JSON-RPC 2.0 request body → its JSON-RPC response.
//   - GET|POST /sse : the same dispatch, returned as one server-sent event frame.
//
// The listener binds loopback by default (R4.2): a bare or empty address is
// rewritten to 127.0.0.1 so spec contents never leave the host unless an
// operator supplies an explicit external address. Tool calls are serialised
// with a mutex because callTool's capture() swaps the process-global os.Stdout;
// concurrent dispatch would interleave captured output (R7). The stdio path is
// untouched, so leaving --http unset keeps today's behaviour byte-identical
// (R4.3). Stdlib-only, no third-party MCP SDK (R4.4).
func ServeHTTP(addr string, dispatch Dispatcher, cfg *core.Config) error {
	return ServeHTTPPinned(addr, dispatch, cfg, "")
}

// ServeHTTPPinned exposes the same dispatch with optional pinned active spec
// affinity. Blank pin preserves historical global fallback.
func ServeHTTPPinned(addr string, dispatch Dispatcher, cfg *core.Config, pinned string) error {
	// expose:"phase" gets a shared registry the watcher keeps current so tools/list
	// reflects the active phase even over the one-shot HTTP/SSE adapter. The adapter
	// has no standing server→client stream, so it cannot push
	// notifications/tools/list_changed — the watcher runs with a nil notify and the
	// host re-fetches tools/list (the spec §8 graceful-degradation path). Other
	// modes pass a nil registry and behave exactly as before (R6).
	var registry *toolRegistry
	if phaseMode(cfg) {
		registry = newToolRegistry(buildToolsForSpec(cfg, pinned))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		startPhaseWatcher(ctx, registry, cfg, pinned, nil)
	}
	resolved := loopbackAddr(addr)
	token := os.Getenv(mcpTokenEnv)
	// A non-loopback bind with no token exposes unauthenticated workflow control;
	// warn loudly so an accidental 0.0.0.0 bind is never silent (A2 R2).
	warnExposure(stdErr, resolved, token)
	handler := tokenAuth(token, httpHandler(dispatch, cfg, registry, pinned))
	srv := newHTTPServer(resolved, handler)
	return srv.ListenAndServe()
}

// newHTTPServer builds the MCP transport's http.Server with the A1 slow-client
// bounds. It deliberately sets no server-wide WriteTimeout: that would sever the
// long-lived /sse stream. /rpc instead carries a per-response write deadline
// (httpHandler), and /sse explicitly clears any deadline.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		IdleTimeout:       httpIdleTimeout,
	}
}

// httpHandler builds the /rpc and /sse routes sharing one dispatch mutex. cfg is
// threaded through to tools/list filtering exactly as on the stdio path; registry
// (non-nil only under expose:"phase") supplies the live phase subset.
func httpHandler(dispatch Dispatcher, cfg *core.Config, registry *toolRegistry, pinned string) http.Handler {
	// Single-flight by design: one process-wide mutex serialises ALL dispatch
	// across /rpc and /sse, so the server processes exactly one in-flight request
	// at a time. This is deliberate, not a missing optimization. callTool's
	// capture() swaps the process-global os.Stdout, so concurrent dispatch would
	// interleave captured output; serialising also preserves the determinism
	// invariant and matches the local-first, single-agent model. The throughput
	// ceiling this imposes is intentional — do not load-test this transport as if
	// it were concurrent. See docs/mcp-guide.md "Concurrency model".
	var mu sync.Mutex
	dispatchLocked := func(raw []byte) []byte {
		mu.Lock()
		defer mu.Unlock()
		return dispatchOnce(raw, dispatch, cfg, registry, pinned)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		// Bound this single response write so a stalled reader cannot pin the
		// writer forever (A1 R2). /rpc is request/response, so a deadline is safe.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(httpRPCWriteTimeout))
		raw, rerr := readBody(r)
		var resp []byte
		if rerr.err != nil {
			resp = rpcErrorResponse(rerr.rpcError())
		} else {
			resp = dispatchLocked(raw)
		}
		w.Header().Set("Content-Type", "application/json")
		// JSON-RPC carries its own error envelope, so even a parse/size error is a
		// 200 with an error body — clients parse the result uniformly.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(resp)
	})
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
			return
		}
		// Clear any inherited write deadline: the SSE stream is long-lived and
		// must outlive the /rpc write bound (A1 R3). The zero time disables it.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
		raw, rerr := readBody(r)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		if rerr.err != nil {
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(rpcErrorResponse(rerr.rpcError()))
			_, _ = w.Write([]byte("\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			return // an empty stream open is a valid no-op
		}
		resp := dispatchLocked(raw)
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(resp)
		_, _ = w.Write([]byte("\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})
	return mux
}

// dispatchOnce routes a single raw JSON-RPC message through the exact same
// conn.handle/route path as the stdio loop, capturing the framed response into
// a buffer. A notification (no id) yields no bytes, matching stdio. The conn has
// no reader because handle never reads — it only routes the bytes it is given.
func dispatchOnce(raw []byte, dispatch Dispatcher, cfg *core.Config, registry *toolRegistry, pinned string) []byte {
	var buf bytes.Buffer
	c := &conn{w: &buf, mode: framingNewline, registry: registry, pinned: pinned}
	c.handle(raw, dispatch, cfg)
	return bytes.TrimRight(buf.Bytes(), "\n")
}

func rpcErrorResponse(rerr *rpcError) []byte {
	var buf bytes.Buffer
	c := &conn{w: &buf, mode: framingNewline}
	_ = c.writeMessage(rpcResponse{Jsonrpc: "2.0", Error: rerr})
	return bytes.TrimRight(buf.Bytes(), "\n")
}

// readBody reads a bounded request body.
func readBody(r *http.Request) ([]byte, rpcReadError) {
	defer r.Body.Close()
	if r.ContentLength > maxRPCBody {
		return nil, oversizedRPCError()
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxRPCBody+1))
	if err != nil {
		return nil, rpcReadError{err: &rpcError{Code: errInvalidRequest, Message: "read body: " + err.Error()}}
	}
	if len(raw) > maxRPCBody {
		return nil, oversizedRPCError()
	}
	return raw, rpcReadError{}
}

// tokenAuth wraps h with optional bearer-token auth (A2 R3). An empty token is a
// pass-through so the loopback-default path is byte-for-byte unchanged (R3.4).
// Otherwise every request MUST carry a matching `Authorization: Bearer <token>`
// header; a miss returns 401 and never reaches dispatch (R3.2). The comparison
// is constant-time to avoid leaking the token via timing (R3.3).
func tokenAuth(token string, h http.Handler) http.Handler {
	if token == "" {
		return h
	}
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		// ConstantTimeCompare is 0 on a length mismatch too, so unequal lengths
		// still fail closed without an early, timing-revealing return.
		if subtle.ConstantTimeCompare(got, want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// warnExposure prints a startup warning when the resolved bind is non-loopback
// and no auth token is set (A2 R2). Loopback binds never warn (R2.3). The token
// value is never echoed (only its presence gates the warning), per the
// token-leakage risk in the spec.
func warnExposure(w io.Writer, resolvedAddr, token string) {
	if token != "" || isLoopbackBind(resolvedAddr) {
		return
	}
	fmt.Fprintf(w, "warn: MCP --http is bound to non-loopback %s with no auth token. "+
		"Workflow control (dispatch, phase transitions) is exposed UNAUTHENTICATED. "+
		"Set %s to require a bearer token, or bind a loopback address.\n", resolvedAddr, mcpTokenEnv)
}

// isLoopbackBind reports whether addr targets a loopback (or unspecified-host)
// interface. A bare/host-less addr has already been rewritten to loopback by
// loopbackAddr; an unparseable host is treated as external (fail loud).
func isLoopbackBind(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" || host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// loopbackAddr defaults an empty or host-less address to loopback so the
// transport never binds a public interface implicitly (R4.2).
func loopbackAddr(addr string) string {
	switch {
	case addr == "":
		return "127.0.0.1:8765"
	case strings.HasPrefix(addr, ":"):
		return "127.0.0.1" + addr
	default:
		return addr
	}
}
