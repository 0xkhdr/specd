package core

import (
	"embed"
	"encoding/json"
	"strings"
)

//go:embed embed_templates
var TemplatesFS embed.FS

func ReadTemplate(rel string) (string, error) {
	b, err := TemplatesFS.ReadFile("embed_templates/" + rel)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ApplyVars(text string, vars map[string]string) string {
	for k, v := range vars {
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
	}
	return text
}

// MarshalEffectiveOrchestrationPolicy returns the authority-bearing policy in
// canonical JSON field order. The concrete type makes credentials, environment
// data, and other unrelated config structurally impossible to serialize.
func MarshalEffectiveOrchestrationPolicy(cfg Config) ([]byte, error) {
	policy := cfg.Orchestration
	if err := ValidateOrchestrationConfig(&policy); err != nil {
		return nil, err
	}
	return json.Marshal(policy)
}
