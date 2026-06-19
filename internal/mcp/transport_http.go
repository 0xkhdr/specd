package mcp

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
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
func ServeHTTP(addr string, dispatch Dispatcher) error {
	srv := &http.Server{
		Addr:    loopbackAddr(addr),
		Handler: httpHandler(dispatch),
	}
	return srv.ListenAndServe()
}

// httpHandler builds the /rpc and /sse routes sharing one dispatch mutex.
func httpHandler(dispatch Dispatcher) http.Handler {
	var mu sync.Mutex
	dispatchLocked := func(raw []byte) []byte {
		mu.Lock()
		defer mu.Unlock()
		return dispatchOnce(raw, dispatch)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
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
func dispatchOnce(raw []byte, dispatch Dispatcher) []byte {
	var buf bytes.Buffer
	c := &conn{w: &buf, mode: framingNewline}
	c.handle(raw, dispatch)
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
