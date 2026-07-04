package core

type Handshake struct {
	Version string   `json:"version"`
	Agent   string   `json:"agent,omitempty"`
	Tools   []string `json:"tools"`
}

func BootstrapHandshake(config Config) Handshake {
	tools := CommandNames()
	allowed := make([]string, 0, len(tools))
	for _, tool := range tools {
		if !ForbiddenTool(tool) {
			allowed = append(allowed, tool)
		}
	}
	return Handshake{Version: "1", Agent: config.Agent, Tools: allowed}
}
