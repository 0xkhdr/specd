package mcp

import "github.com/0xkhdr/specd/internal/core"

func BrainTools(config core.Config) []Tool {
	if !config.Orchestration.Enabled {
		return nil
	}
	return []Tool{{
		Name:        "brain.next",
		Description: "Return deterministic orchestration guidance when brain mode is configured.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
		},
	}}
}
