package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type Handshake struct {
	Version string   `json:"version"`
	Agent   string   `json:"agent,omitempty"`
	Tools   []string `json:"tools"`
	// PaletteDigest and ConfigDigest let an agent detect that its cached command
	// palette or effective config has drifted from this binary's (spec 11 R6).
	// Both are SHA-256 over the canonical (stable-key-order) JSON.
	PaletteDigest string `json:"palette_digest"`
	ConfigDigest  string `json:"config_digest"`
}

func BootstrapHandshake(config Config) Handshake {
	tools := CommandNames()
	allowed := make([]string, 0, len(tools))
	for _, tool := range tools {
		if !ForbiddenTool(tool) {
			allowed = append(allowed, tool)
		}
	}
	return Handshake{
		Version:       "1",
		Agent:         config.Agent,
		Tools:         allowed,
		PaletteDigest: PaletteDigest(),
		ConfigDigest:  ConfigDigest(config),
	}
}

// PaletteDigest is the SHA-256 of the canonical `help --json` payload (spec 03).
// It is stable across runs (Go marshals struct fields in declaration order and
// map keys sorted) and changes when a verb or flag is added — exactly the drift
// an agent's --expect-palette-digest guard catches.
func PaletteDigest() string {
	return digest(BuildHelpPayload())
}

// ConfigDigest is the SHA-256 of the effective config.
func ConfigDigest(config Config) string {
	return digest(config)
}

func digest(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
