package mcp

import (
	"bufio"
	"encoding/json"
	"io"
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
}

func Serve(r io.Reader, w io.Writer, tools []Tool) error {
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
		response := Dispatch(req, tools)
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func Dispatch(req Request, tools []Tool) Response {
	resp := Response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "tools/list":
		resp.Result = map[string]any{"tools": tools}
	default:
		resp.Error = &ResponseError{Code: -32601, Message: "method not found"}
	}
	return resp
}
