package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func TestProbeHealthyServer(t *testing.T) {
	result, err := Probe(context.Background(), nil, time.Second)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.ProtocolVersion != latestProtocolVersion {
		t.Errorf("ProtocolVersion = %q, want %q", result.ProtocolVersion, latestProtocolVersion)
	}
	if result.ToolCount < len(requiredProbeTools()) {
		t.Errorf("ToolCount = %d, want at least %d", result.ToolCount, len(requiredProbeTools()))
	}
	if !reflect.DeepEqual(result.OrchestrationTools, orchestrationTools) {
		t.Errorf("OrchestrationTools = %v, want %v", result.OrchestrationTools, orchestrationTools)
	}
	if !reflect.DeepEqual(result.RequiredTools, requiredProbeTools()) {
		t.Errorf("RequiredTools = %v, want %v", result.RequiredTools, requiredProbeTools())
	}
	if result.Latency <= 0 {
		t.Errorf("Latency = %v, want positive", result.Latency)
	}
	if result.LatencyMillis < 0 {
		t.Errorf("LatencyMillis = %d, want non-negative", result.LatencyMillis)
	}
}

func TestProbeFailures(t *testing.T) {
	t.Run("malformed response", func(t *testing.T) {
		_, err := probe(context.Background(), nil, time.Second, func(r io.Reader, w io.Writer, _ Dispatcher, _ *core.Config) error {
			_, _ = bufio.NewReader(r).ReadBytes('\n')
			_, err := io.WriteString(w, "not-json\n")
			return err
		})
		assertProbeKind(t, err, ProbeFailureMalformed)
	})

	t.Run("timeout", func(t *testing.T) {
		_, err := probe(context.Background(), nil, 20*time.Millisecond, func(r io.Reader, _ io.Writer, _ Dispatcher, _ *core.Config) error {
			_, err := io.Copy(io.Discard, r)
			return err
		})
		assertProbeKind(t, err, ProbeFailureTimeout)
	})

	t.Run("protocol mismatch", func(t *testing.T) {
		_, err := probe(context.Background(), nil, time.Second, func(r io.Reader, w io.Writer, _ Dispatcher, _ *core.Config) error {
			_, _ = bufio.NewReader(r).ReadBytes('\n')
			return writeProbeResult(w, 1, map[string]any{"protocolVersion": legacyProtocolVersion})
		})
		assertProbeKind(t, err, ProbeFailureProtocolMismatch)
	})

	t.Run("missing baseline tool", func(t *testing.T) {
		_, err := probe(context.Background(), nil, time.Second, func(r io.Reader, w io.Writer, _ Dispatcher, _ *core.Config) error {
			reader := bufio.NewReader(r)
			_, _ = reader.ReadBytes('\n')
			if err := writeProbeResult(w, 1, map[string]any{"protocolVersion": latestProtocolVersion}); err != nil {
				return err
			}
			_, _ = reader.ReadBytes('\n')
			_, _ = reader.ReadBytes('\n')
			return writeProbeResult(w, 2, map[string]any{
				"tools": []map[string]any{{"name": "specd_status"}},
			})
		})
		assertProbeKind(t, err, ProbeFailureMissingTool)
		if !strings.Contains(err.Error(), "specd_init") {
			t.Errorf("error = %q, want first missing baseline tool", err)
		}
	})

	t.Run("missing orchestration tool", func(t *testing.T) {
		_, err := probe(context.Background(), nil, time.Second, func(r io.Reader, w io.Writer, _ Dispatcher, _ *core.Config) error {
			reader := bufio.NewReader(r)
			_, _ = reader.ReadBytes('\n')
			if err := writeProbeResult(w, 1, map[string]any{"protocolVersion": latestProtocolVersion}); err != nil {
				return err
			}
			_, _ = reader.ReadBytes('\n')
			_, _ = reader.ReadBytes('\n')
			tools := make([]map[string]any, 0, len(baselineTools))
			for _, name := range baselineTools {
				tools = append(tools, map[string]any{"name": name})
			}
			return writeProbeResult(w, 2, map[string]any{"tools": tools})
		})
		assertProbeKind(t, err, ProbeFailureMissingTool)
		if !strings.Contains(err.Error(), "specd_brain") {
			t.Errorf("error = %q, want first missing orchestration tool", err)
		}
	})
}

// TestProbeDeterministic asserts the probe's machine-contract fields
// (protocol version, tool count) are stable across runs. Latency is excluded:
// it is wall-clock and must never gate CI (task T26, R4.1).
func TestProbeDeterministic(t *testing.T) {
	first, err := Probe(context.Background(), nil, time.Second)
	if err != nil {
		t.Fatalf("first probe: %v", err)
	}
	second, err := Probe(context.Background(), nil, time.Second)
	if err != nil {
		t.Fatalf("second probe: %v", err)
	}
	if first.ProtocolVersion != second.ProtocolVersion {
		t.Errorf("ProtocolVersion drift: %q vs %q", first.ProtocolVersion, second.ProtocolVersion)
	}
	if first.ToolCount != second.ToolCount {
		t.Errorf("ToolCount drift: %d vs %d", first.ToolCount, second.ToolCount)
	}
}

// BenchmarkProbe records the in-process MCP handshake + tools/list latency
// baseline (docs/agent-harness-baselines.md, success metric p95 < 500ms).
func BenchmarkProbe(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := Probe(context.Background(), nil, time.Second); err != nil {
			b.Fatalf("probe: %v", err)
		}
	}
}

func writeProbeResult(w io.Writer, id int, result any) error {
	return json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func assertProbeKind(t *testing.T, err error, want ProbeFailureKind) {
	t.Helper()
	var probeErr *ProbeError
	if !errors.As(err, &probeErr) {
		t.Fatalf("error = %v, want *ProbeError", err)
	}
	if probeErr.Kind != want {
		t.Errorf("Kind = %q, want %q", probeErr.Kind, want)
	}
}
