package mcp

import "github.com/0xkhdr/specd/internal/core"

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func CoreTools() []Tool {
	tools := make([]Tool, 0, len(core.Commands))
	for _, command := range core.Commands {
		if core.ForbiddenTool(command.Name) {
			continue
		}
		tools = append(tools, Tool{
			Name:        command.Name,
			Description: command.Description,
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		})
	}
	return tools
}
