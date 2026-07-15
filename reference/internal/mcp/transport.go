// Package mcp exposes specd's existing command handlers as Model Context
// Protocol (MCP) tools over a JSON-RPC 2.0 stdio transport. It adds no business
// logic: every tool call is a thin re-dispatch into the same handlers the CLI
// drives. The package is stdlib-only — the JSON-RPC framing and MCP envelopes
// are hand-rolled on encoding/json, with no third-party MCP SDK.
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// JSON-RPC 2.0 error codes (the subset MCP relies on).
const (
	errParse          = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602

	maxRPCBody = 1 << 20 // 1 MiB per JSON-RPC request
)

// rpcRequest is an incoming JSON-RPC 2.0 message. A request with no id is a
// notification and receives no response (per the spec).
type rpcRequest struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// rpcNotification is a server→client JSON-RPC notification: a method call with no
// id, so the client sends no response (dynamic-tool-list spec §5.3).
type rpcNotification struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
}

type rpcReadError struct{ err *rpcError }

func (e rpcReadError) Error() string       { return e.err.Message }
func (e rpcReadError) rpcError() *rpcError { return e.err }

func oversizedRPCError() rpcReadError {
	return rpcReadError{err: &rpcError{Code: errInvalidRequest, Message: fmt.Sprintf("message exceeds %d-byte limit", maxRPCBody)}}
}

type rpcResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// framing is the on-the-wire message delimiter style. MCP hosts use either
// newline-delimited JSON or LSP-style Content-Length headers; we auto-detect on
// the first byte and stay in that mode for the connection's lifetime.
type framing int

const (
	framingUnknown framing = iota
	framingNewline
	framingHeader
)

// conn reads and writes framed JSON-RPC messages over a single stdio pair.
type conn struct {
	r    *bufio.Reader
	w    io.Writer
	mode framing

	// wmu serialises writes so a server-initiated notification (the phase
	// watcher's notifications/tools/list_changed) can never interleave its bytes
	// with a concurrently written response (dynamic-tool-list spec §8 risk, R4).
	wmu sync.Mutex

	// registry, when non-nil, supplies the live tool list for tools/list under
	// expose:"phase"; the watcher swaps it as the active spec's status changes.
	// A nil registry means the static buildTools(cfg) path (today's behaviour).
	registry *toolRegistry

	// prefs holds the session's host-negotiation tool-shaping hints parsed from
	// initialize (host-negotiation spec C2). Zero value = no hints = no-op.
	prefs hostPrefs

	// pinned narrows active-spec resolution to one slug for this MCP process.
	// Blank preserves the historical global fallback.
	pinned string
}

func newConn(r io.Reader, w io.Writer) *conn {
	return &conn{r: bufio.NewReader(r), w: w, mode: framingUnknown}
}

// readMessage returns the next raw JSON message body. On the first call it
// detects the framing: a 'C' (Content-Length) selects header framing, anything
// else (a JSON message starts with '{') selects newline framing.
func (c *conn) readMessage() ([]byte, error) {
	if c.mode == framingUnknown {
		for {
			b, err := c.r.Peek(1)
			if err != nil {
				return nil, err
			}
			switch b[0] {
			case '\n', '\r', ' ', '\t':
				if _, err := c.r.ReadByte(); err != nil {
					return nil, err
				}
				continue
			case 'C':
				c.mode = framingHeader
			default:
				c.mode = framingNewline
			}
			break
		}
	}
	if c.mode == framingHeader {
		return c.readHeaderFramed()
	}
	return c.readLine()
}

// readLine returns the next non-blank newline-delimited message, capped so a
// single client line cannot grow memory without bound.
func (c *conn) readLine() ([]byte, error) {
	for {
		var line []byte
		for {
			part, err := c.r.ReadSlice('\n')
			if len(line)+len(part) > maxRPCBody {
				discardLine(c.r, err)
				return nil, oversizedRPCError()
			}
			line = append(line, part...)
			if errors.Is(err, bufio.ErrBufferFull) {
				continue
			}
			if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
				return trimmed, nil
			}
			if err != nil {
				return nil, err
			}
			break
		}
	}
}

func discardLine(r *bufio.Reader, lastErr error) {
	for errors.Is(lastErr, bufio.ErrBufferFull) {
		_, lastErr = r.ReadSlice('\n')
	}
}

// readHeaderFramed reads an LSP-style Content-Length framed message.
func (c *conn) readHeaderFramed() ([]byte, error) {
	length := -1
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // end of headers
		}
		if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
			length = n
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("message missing Content-Length header")
	}
	if length > maxRPCBody {
		return nil, oversizedRPCError()
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(c.r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// writeMessage marshals v and writes it using the connection's framing. Before
// any read has set the mode it defaults to newline framing.
func (c *conn) writeMessage(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if c.mode == framingHeader {
		if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(b)); err != nil {
			return err
		}
		_, err = c.w.Write(b)
		return err
	}
	_, err = c.w.Write(append(b, '\n'))
	return err
}

// notifyToolsListChanged writes a server-initiated JSON-RPC notification (no id)
// telling the host the tool list changed, so it re-fetches tools/list
// (dynamic-tool-list spec R3). Writes go through writeMessage's mutex, so a
// notification never interleaves with an in-flight response.
func (c *conn) notifyToolsListChanged() {
	_ = c.writeMessage(rpcNotification{Jsonrpc: "2.0", Method: "notifications/tools/list_changed"})
}
