package core

import (
	"embed"
	"encoding/json"
	"strings"
)

// TemplatesFS embeds the contents of embed_templates, the built-in scaffold
// and config templates shipped with the binary.
//
//go:embed embed_templates
var TemplatesFS embed.FS

// ReadTemplate returns the contents of the embedded template at the
// embed_templates-relative path rel.
func ReadTemplate(rel string) (string, error) {
	b, err := TemplatesFS.ReadFile("embed_templates/" + rel)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ApplyVars replaces every "{{key}}" placeholder in text with its
// corresponding value from vars.
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
