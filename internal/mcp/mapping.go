package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/0xkhdr/specd/internal/adapter"
	"github.com/0xkhdr/specd/internal/core"
)

// MissionToolCall projects an adapter mission request into MCP tools/call
// params without weakening its common-envelope pins. Receiver tool validates
// envelope and mission identity before dispatch.
func MissionToolCall(req adapter.Request, toolName string) (map[string]any, error) {
	if _, err := adapter.MissionFromRequest(req); err != nil {
		raw, _ := json.Marshal(req)
		entity := req.Subject.MissionID
		if entity == "" {
			entity = req.Subject.TaskID
		}
		return nil, core.Refusef("MISSION_INVALID", "invalid mission tool call: %v", err).
			WithContext(entity, "adapter request failed mission validation", "valid mission identity and authority envelope").
			WithInput("adapter request", raw).
			WithSuccessor(core.RefusalActorOperator, "brain.status", "specd brain status <slug>").
			Wrapping(err)
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

// RequestModeGuide maps resolved routing into the same guidance used by
// generated files and host adapters.
func RequestModeGuide(resolution core.RequestModeResolution, taskID string) string {
	return core.RequestModeGuide(resolution.Mode, resolution.SelectedSpec, taskID, resolution.Assurance)
}
