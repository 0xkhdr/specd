package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

const (
	latestProtocolVersion = "2025-11-25"
	legacyProtocolVersion = "2024-11-05"
	serverInstructions    = "Call specd_status/context first. MCP is bounded: use brain start/step/status and watch --once only; host runs Pinky workers. Never edit state/tasks checkboxes. Approval is policy-gated; completion needs specd_verify evidence. Cancellation is cooperative."
)

// supportedProtocolVersions is newest-first so fallback negotiation is stable.
var supportedProtocolVersions = []string{
	latestProtocolVersion,
	"2025-06-18",
	legacyProtocolVersion,
}

// Dispatcher runs a registered specd command and reports whether it was known.
// It mirrors cmd.Dispatch exactly; injecting it keeps this package free of an
// import cycle with internal/cmd.
type Dispatcher func(command string, args cli.Args) (int, bool)

// Serve runs the MCP stdio loop until the input stream closes. It reads framed
// JSON-RPC requests from r, dispatches each into the existing command handlers,
// and writes framed responses to w. A malformed request never tears down the
// loop: it yields a JSON-RPC error and the server keeps reading (R5). All
// diagnostics belong on stderr; r/w carry only protocol bytes.
func Serve(r io.Reader, w io.Writer, dispatch Dispatcher) error {
	c := newConn(r, w)
	for {
		raw, err := c.readMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			if rerr, ok := err.(rpcReadError); ok {
				_ = c.writeMessage(rpcResponse{Jsonrpc: "2.0", Error: rerr.rpcError()})
				continue
			}
			return err
		}
		c.handle(raw, dispatch)
	}
}

func (c *conn) handle(raw []byte, dispatch Dispatcher) {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		_ = c.writeMessage(rpcResponse{Jsonrpc: "2.0", Error: &rpcError{Code: errParse, Message: "parse error: " + err.Error()}})
		return
	}
	if req.Jsonrpc != "2.0" || req.Method == "" {
		_ = c.writeMessage(rpcResponse{Jsonrpc: "2.0", ID: req.ID, Error: &rpcError{Code: errInvalidRequest, Message: "invalid JSON-RPC 2.0 request"}})
		return
	}

	result, rerr := route(req, dispatch)

	// A request without an id is a notification — acknowledge nothing.
	if len(req.ID) == 0 {
		return
	}
	resp := rpcResponse{Jsonrpc: "2.0", ID: req.ID}
	if rerr != nil {
		resp.Error = rerr
	} else {
		resp.Result = result
	}
	_ = c.writeMessage(resp)
}

func route(req rpcRequest, dispatch Dispatcher) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return initializeResult(req.Params), nil
	case "ping", "notifications/initialized", "notifications/cancelled":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": buildTools()}, nil
	case "tools/call":
		return callTool(req.Params, dispatch)
	default:
		return nil, &rpcError{Code: errMethodNotFound, Message: "method not found: " + req.Method}
	}
}

type initializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

func initializeResult(rawParams json.RawMessage) map[string]any {
	var params initializeParams
	_ = json.Unmarshal(rawParams, &params)

	return map[string]any{
		"protocolVersion": negotiateProtocolVersion(params.ProtocolVersion),
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
		"serverInfo": map[string]any{
			"name":        "specd",
			"title":       "specd",
			"version":     core.Version,
			"description": "Deterministic spec-driven coding harness",
		},
		"instructions": serverInstructions,
	}
}

func negotiateProtocolVersion(requested string) string {
	// Pre-negotiation clients sent no version and expected the historical
	// revision. Retain that compatibility while negotiating all valid requests.
	if requested == "" {
		return legacyProtocolVersion
	}
	for _, supported := range supportedProtocolVersions {
		if requested == supported {
			return requested
		}
	}
	return latestProtocolVersion
}

type callParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// callTool re-dispatches an MCP tool call into the matching specd handler with
// SPECD_JSON semantics, capturing its stdout/stderr and mapping its exit code to
// the MCP result. A non-zero exit becomes isError:true; a handler panic is
// recovered so one bad call never crashes the server (R3).
func callTool(rawParams json.RawMessage, dispatch Dispatcher) (any, *rpcError) {
	var p callParams
	if err := json.Unmarshal(rawParams, &p); err != nil {
		return nil, &rpcError{Code: errInvalidParams, Message: "invalid params: " + err.Error()}
	}
	command, ok := strings.CutPrefix(p.Name, toolPrefix)
	if !ok || metaCommands[command] {
		return nil, &rpcError{Code: errInvalidParams, Message: "unknown tool: " + p.Name}
	}
	argv, err := buildArgv(p.Arguments)
	if err != nil {
		return nil, &rpcError{Code: errInvalidParams, Message: err.Error()}
	}
	// Force structured handler output: append --json (a no-op for commands that
	// lack the flag) and set SPECD_JSON so JSON-mode suppression fires too.
	argv = append(argv, "--json")
	args := cli.ParseArgs(argv)
	if err := enforceBoundedToolCall(command, args); err != nil {
		return nil, &rpcError{Code: errInvalidParams, Message: err.Error()}
	}
	restoreEnv := setJSONMode()
	defer restoreEnv()

	var known bool
	stdout, stderr, code := capture(func() int {
		rc, k := dispatch(command, args)
		known = k
		return rc
	})
	if !known {
		return nil, &rpcError{Code: errInvalidParams, Message: "unknown tool: " + p.Name}
	}

	return toolResult(stdout, stderr, code), nil
}

// toolResult assembles the MCP tools/call payload. The text content is the
// handler's stdout (plus stderr diagnostics on failure, since handlers report
// errors there). When stdout is JSON it is also attached as structuredContent.
func toolResult(stdout, stderr string, code int) map[string]any {
	text := stdout
	isErr := code != core.ExitOK
	if isErr {
		if diag := strings.TrimSpace(stderr); diag != "" {
			if strings.TrimSpace(text) == "" {
				text = diag
			} else {
				text += "\n" + diag
			}
		}
	}
	result := map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isErr,
	}
	if trimmed := strings.TrimSpace(stdout); trimmed != "" {
		var structured any
		if json.Unmarshal([]byte(trimmed), &structured) == nil {
			result["structuredContent"] = structured
		}
	}
	return result
}

// buildArgv turns a tool's arguments object into a CLI argv that round-trips
// through cli.ParseArgs: the "args" array supplies ordered positionals, and each
// remaining key becomes a flag (booleans as bare --flag, others as --flag value).
// Flags are emitted in sorted order for deterministic argv.
func enforceBoundedToolCall(command string, args cli.Args) error {
	if command != "watch" {
		return nil
	}
	if args.Str("sse") != "" || args.Str("webhook") != "" {
		return fmt.Errorf("specd_watch over MCP does not allow --sse or --webhook; use CLI for streaming transports")
	}
	if !args.Bool("once") {
		return fmt.Errorf("specd_watch over MCP requires --once so one request stays bounded")
	}
	return nil
}

func buildArgv(arguments map[string]any) ([]string, error) {
	var argv []string
	if raw, ok := arguments["args"]; ok && raw != nil {
		list, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("'args' must be an array of strings")
		}
		for _, item := range list {
			argv = append(argv, fmt.Sprint(item))
		}
	}
	keys := make([]string, 0, len(arguments))
	for k := range arguments {
		if k != "args" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch v := arguments[k].(type) {
		case bool:
			if v {
				argv = append(argv, "--"+k)
			}
		case nil:
			// omitted flag
		default:
			argv = append(argv, "--"+k, fmt.Sprint(v))
		}
	}
	return argv, nil
}

// setJSONMode sets SPECD_JSON=1 and returns a function that restores its prior
// value, so a tool call never leaks JSON mode into the server's environment.
func setJSONMode() func() {
	prev, had := os.LookupEnv("SPECD_JSON")
	os.Setenv("SPECD_JSON", "1")
	return func() {
		if had {
			os.Setenv("SPECD_JSON", prev)
		} else {
			os.Unsetenv("SPECD_JSON")
		}
	}
}

// capture redirects os.Stdout/os.Stderr through pipes for the duration of fn,
// draining each in its own goroutine so a large handler output never deadlocks
// on a full pipe buffer. A panic in fn is recovered and reported as a gate
// failure so the server survives (R3). Calls are processed one at a time by
// Serve, so this process-global swap is safe within the loop.
func capture(fn func() int) (stdout, stderr string, code int) {
	origOut, origErr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr

	outCh := drain(rOut)
	errCh := drain(rErr)

	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(wErr, "panic: %v", r)
				code = core.ExitGate
			}
		}()
		code = fn()
	}()

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout, os.Stderr = origOut, origErr
	return <-outCh, <-errCh, code
}

func drain(r *os.File) <-chan string {
	ch := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		ch <- buf.String()
	}()
	return ch
}
