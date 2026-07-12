package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Result  any            `json:"result,omitempty"`
	Error   *ResponseError `json:"error,omitempty"`
}

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

const MCPHandoffRequiredCode = -32002

type Handoff struct {
	Code    string `json:"code"`
	Actor   string `json:"actor"`
	Command string `json:"command"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// Executor runs one specd verb and returns its captured stdout. It is injected
// by the caller (internal/cmd) so the mcp package never imports the dispatcher —
// that back-edge would be an import cycle, which is why tool execution lived
// behind the injection seam rather than inside this package.
type Executor func(name string, args []string, flags map[string]string) (string, error)

func Serve(r io.Reader, w io.Writer, tools []Tool, exec Executor) error {
	scanner := bufio.NewScanner(r)
	encoder := json.NewEncoder(w)
	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			if err := encoder.Encode(Response{JSONRPC: "2.0", Error: &ResponseError{Code: -32700, Message: "parse error"}}); err != nil {
				return err
			}
			continue
		}
		response := Dispatch(req, tools, exec)
		if req.ID == nil {
			// JSON-RPC notification (e.g. notifications/initialized): no id, no
			// reply. Answering one is a protocol violation.
			continue
		}
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func Dispatch(req Request, tools []Tool, exec Executor) Response {
	return DispatchAuthorized(req, tools, exec, nil, time.Time{}, "")
}

func DispatchAuthorized(req Request, tools []Tool, exec Executor, authority *core.AuthorityV1, now time.Time, phase string) Response {
	resp := Response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		// MCP handshake: a compliant client sends `initialize` first and will not
		// proceed to tools/list until it succeeds. Without this the connection
		// never establishes, so even tool discovery is unreachable.
		resp.Result = map[string]any{
			"protocolVersion":       "2024-11-05",
			"driverProtocolVersion": core.DriverProtocolVersion,
			"capabilities":          map[string]any{"tools": map[string]any{}},
			"serverInfo":            map[string]any{"name": "specd", "version": "1"},
		}
	case "tools/list":
		resp.Result = map[string]any{"tools": tools}
	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil || params.Name == "" {
			resp.Error = &ResponseError{Code: -32602, Message: "invalid params"}
			return resp
		}
		if command, ok := core.CommandByName(params.Name); ok && command.HumanOnly {
			args, _ := splitArguments(params.Arguments)
			commandLine := "specd " + params.Name
			if len(args) > 0 {
				commandLine += " " + strings.Join(args, " ")
			}
			handoff := Handoff{Code: "MCP_HANDOFF_REQUIRED", Actor: "human", Command: commandLine}
			resp.Error = &ResponseError{Code: MCPHandoffRequiredCode, Message: handoff.Code + ": actor=" + handoff.Actor + " command=" + handoff.Command, Data: handoff}
			return resp
		}
		if authority == nil {
			if rawAuthority, ok := params.Arguments["authority"]; ok {
				raw, _ := json.Marshal(rawAuthority)
				var parsed core.AuthorityV1
				if json.Unmarshal(raw, &parsed) == nil {
					authority = &parsed
					now = time.Now()
					phase = parsed.Phase
				}
				delete(params.Arguments, "authority")
			}
		}
		if core.ForbiddenTool(params.Name) {
			resp.Error = &ResponseError{Code: -32001, Message: "tool denied by policy"}
			return resp
		}
		known := false
		mutable := false
		for _, tool := range tools {
			if tool.Name == params.Name {
				known = true
				break
			}
		}
		for _, tool := range core.ManifestToolContracts() {
			if tool.Name == params.Name {
				mutable = tool.Mutable
			}
		}
		if !known {
			resp.Error = &ResponseError{Code: -32001, Message: "tool denied by policy"}
			return resp
		}
		if authority != nil {
			if err := core.AuthorizeTool(*authority, params.Name, nil, now, phase, mutable); err != nil {
				resp.Error = &ResponseError{Code: -32001, Message: "tool denied by authority"}
				return resp
			}
		}
		if exec == nil {
			resp.Error = &ResponseError{Code: -32601, Message: "tool not implemented"}
			return resp
		}
		args, flags := splitArguments(params.Arguments)
		out, err := exec(params.Name, args, flags)
		if err != nil {
			// A verb failure (non-zero exit, gate/usage rejection) is a tool-level
			// error, not a JSON-RPC protocol error: report it in the result with
			// isError so the client sees both the message and any partial output.
			resp.Result = toolResult(out+err.Error(), true)
			return resp
		}
		resp.Result = toolResult(out, false)
	default:
		resp.Error = &ResponseError{Code: -32601, Message: "method not found"}
	}
	return resp
}

// splitArguments maps an MCP tool-call `arguments` object onto the dispatcher's
// (positional args, flags) shape. The reserved key "args" carries positional
// operands (spec slug, task id) as an ordered array; every other key is a flag.
func splitArguments(arguments map[string]any) ([]string, map[string]string) {
	flags := make(map[string]string)
	var args []string
	for key, value := range arguments {
		if key == "args" {
			if list, ok := value.([]any); ok {
				for _, item := range list {
					args = append(args, valueToString(item))
				}
			}
			continue
		}
		flags[key] = valueToString(value)
	}
	return args, flags
}

func valueToString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return ""
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

// toolResult wraps captured verb output in the MCP tools/call result shape.
func toolResult(text string, isError bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isError,
	}
}
