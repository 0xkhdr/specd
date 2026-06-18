package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

const defaultProbeTimeout = 2 * time.Second

var baselineTools = []string{
	"specd_init",
	"specd_status",
	"specd_context",
	"specd_check",
	"specd_next",
	"specd_verify",
	"specd_task",
}

type ProbeFailureKind string

const (
	ProbeFailureTimeout          ProbeFailureKind = "timeout"
	ProbeFailureTransport        ProbeFailureKind = "transport"
	ProbeFailureMalformed        ProbeFailureKind = "malformed_response"
	ProbeFailureRPC              ProbeFailureKind = "rpc_error"
	ProbeFailureProtocolMismatch ProbeFailureKind = "protocol_mismatch"
	ProbeFailureMissingTool      ProbeFailureKind = "missing_tool"
)

type ProbeError struct {
	Kind ProbeFailureKind
	Step string
	Err  error
}

func (e *ProbeError) Error() string {
	return fmt.Sprintf("mcp probe %s failed (%s): %v", e.Step, e.Kind, e.Err)
}

func (e *ProbeError) Unwrap() error {
	return e.Err
}

type ProbeResult struct {
	ProtocolVersion string        `json:"protocolVersion"`
	ToolCount       int           `json:"toolCount"`
	Latency         time.Duration `json:"-"`
	LatencyMillis   int64         `json:"latencyMillis"`
}

type probeServer func(io.Reader, io.Writer, Dispatcher) error

// Probe performs an in-process MCP lifecycle check without executing a shell or
// starting a network listener.
func Probe(ctx context.Context, dispatch Dispatcher, timeout time.Duration) (ProbeResult, error) {
	return probe(ctx, dispatch, timeout, Serve)
}

func probe(ctx context.Context, dispatch Dispatcher, timeout time.Duration, serve probeServer) (result ProbeResult, err error) {
	if timeout <= 0 {
		timeout = defaultProbeTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()
	defer func() {
		_ = clientOut.Close()
		_ = clientIn.Close()
		_ = serverIn.Close()
		_ = serverOut.Close()
	}()

	go func() {
		serveErr := serve(serverIn, serverOut, dispatch)
		_ = serverOut.CloseWithError(serveErr)
	}()

	reader := bufio.NewReader(clientIn)
	requested := latestProtocolVersion
	if err := probeWrite(ctx, clientOut, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": requested,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "specd-probe", "version": core.Version},
		},
	}); err != nil {
		return ProbeResult{}, classifyProbeIO(ctx, "initialize", err)
	}

	var initialized struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := probeRead(ctx, reader, "initialize", &initialized); err != nil {
		return ProbeResult{}, err
	}
	if initialized.ProtocolVersion != requested {
		return ProbeResult{}, &ProbeError{
			Kind: ProbeFailureProtocolMismatch,
			Step: "initialize",
			Err:  fmt.Errorf("server returned %q, requested %q", initialized.ProtocolVersion, requested),
		}
	}

	if err := probeWrite(ctx, clientOut, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}); err != nil {
		return ProbeResult{}, classifyProbeIO(ctx, "initialized", err)
	}
	if err := probeWrite(ctx, clientOut, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}); err != nil {
		return ProbeResult{}, classifyProbeIO(ctx, "tools/list", err)
	}

	var listed struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := probeRead(ctx, reader, "tools/list", &listed); err != nil {
		return ProbeResult{}, err
	}
	available := make(map[string]bool, len(listed.Tools))
	for _, tool := range listed.Tools {
		available[tool.Name] = true
	}
	for _, required := range baselineTools {
		if !available[required] {
			return ProbeResult{}, &ProbeError{
				Kind: ProbeFailureMissingTool,
				Step: "tools/list",
				Err:  fmt.Errorf("required tool %q is missing", required),
			}
		}
	}

	latency := time.Since(started)
	return ProbeResult{
		ProtocolVersion: initialized.ProtocolVersion,
		ToolCount:       len(listed.Tools),
		Latency:         latency,
		LatencyMillis:   latency.Milliseconds(),
	}, nil
}

func probeWrite(ctx context.Context, w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	done := make(chan error, 1)
	go func() {
		_, err := w.Write(data)
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func probeRead(ctx context.Context, reader *bufio.Reader, step string, target any) error {
	type readResult struct {
		data []byte
		err  error
	}
	done := make(chan readResult, 1)
	go func() {
		data, err := reader.ReadBytes('\n')
		done <- readResult{data: data, err: err}
	}()

	var read readResult
	select {
	case read = <-done:
	case <-ctx.Done():
		return &ProbeError{Kind: ProbeFailureTimeout, Step: step, Err: ctx.Err()}
	}
	if read.err != nil {
		return &ProbeError{Kind: ProbeFailureTransport, Step: step, Err: read.err}
	}

	var response struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.Unmarshal(read.data, &response); err != nil {
		return &ProbeError{Kind: ProbeFailureMalformed, Step: step, Err: err}
	}
	if response.Error != nil {
		return &ProbeError{
			Kind: ProbeFailureRPC,
			Step: step,
			Err:  fmt.Errorf("%d: %s", response.Error.Code, response.Error.Message),
		}
	}
	if len(response.Result) == 0 || strings.TrimSpace(string(response.Result)) == "null" {
		return &ProbeError{Kind: ProbeFailureMalformed, Step: step, Err: errors.New("response has no result")}
	}
	if err := json.Unmarshal(response.Result, target); err != nil {
		return &ProbeError{Kind: ProbeFailureMalformed, Step: step, Err: err}
	}
	return nil
}

func classifyProbeIO(ctx context.Context, step string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &ProbeError{Kind: ProbeFailureTimeout, Step: step, Err: context.DeadlineExceeded}
	}
	return &ProbeError{Kind: ProbeFailureTransport, Step: step, Err: err}
}
