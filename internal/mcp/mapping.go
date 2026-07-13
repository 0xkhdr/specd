package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/0xkhdr/specd/internal/adapter"
)

// MissionToolCall projects an adapter mission request into MCP tools/call
// params without weakening its common-envelope pins. Receiver tool validates
// envelope and mission identity before dispatch.
func MissionToolCall(req adapter.Request, toolName string) (map[string]any, error) {
	if _, err := adapter.MissionFromRequest(req); err != nil {
		return nil, err
	}
	raw, err := req.Canonical()
	if err != nil {
		return nil, fmt.Errorf("encode mission tool call: %w", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode mission tool call: %w", err)
	}
	return map[string]any{"name": toolName, "arguments": map[string]any{"envelope": envelope}}, nil
}
