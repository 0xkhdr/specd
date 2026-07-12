package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"

	"github.com/0xkhdr/specd/internal/version"
)

const ContextSchemaVersion = "2"

type HandshakeSpec struct {
	Slug     string `json:"slug"`
	Status   Status `json:"status"`
	Revision int64  `json:"revision"`
}

type HandshakeAuthority struct {
	HarnessInstructions []string `json:"harness_instructions"`
	UntrustedInputs     []string `json:"untrusted_inputs"`
}

type Handshake struct {
	Version               string             `json:"version"`
	Agent                 string             `json:"agent,omitempty"`
	Tools                 []string           `json:"tools"`
	Binary                version.Info       `json:"binary"`
	StateSchemaVersion    int                `json:"state_schema_version"`
	ContextSchemaVersion  string             `json:"context_schema_version"`
	TemplateSchemaVersion int                `json:"template_schema_version"`
	WorkspaceRoot         string             `json:"workspace_root"`
	ActiveSpec            *HandshakeSpec     `json:"active_spec,omitempty"`
	ManagedDigest         string             `json:"managed_digest"`
	GuidanceDigest        string             `json:"guidance_digest"`
	ContextSchemaDigest   string             `json:"context_schema_digest"`
	NextCommands          []string           `json:"next_commands"`
	Authority             HandshakeAuthority `json:"authority"`
	// PaletteDigest and ConfigDigest let an agent detect that its cached command
	// palette or effective config has drifted from this binary's (spec 11 R6).
	// Both are SHA-256 over the canonical (stable-key-order) JSON.
	PaletteDigest string `json:"palette_digest"`
	ConfigDigest  string `json:"config_digest"`
	// PolicyDigest pins the effective lifecycle judgment policy (spec 01 R7.2):
	// the profile plus the criterion/review/integration gates it arms. It lets a
	// later approval detect that the policy governing an earlier decision has
	// changed, without diffing the whole config.
	PolicyDigest  string         `json:"policy_digest"`
	ToolContracts []ToolContract `json:"tool_contracts"`
}

func BootstrapHandshake(config Config) Handshake {
	hs, _ := BootstrapHandshakeForRoot(".", config, nil, nil)
	return hs
}

// BootstrapHandshakeForRoot emits one production-driver preflight packet. It
// is read-only: callers can validate every pinned identity before mutation.
func BootstrapHandshakeForRoot(root string, config Config, state *State, nextCommands []string) (Handshake, error) {
	tools := CommandNames()
	allowed := make([]string, 0, len(tools))
	for _, tool := range tools {
		if !ForbiddenTool(tool) {
			allowed = append(allowed, tool)
		}
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Handshake{}, err
	}
	managedDigest, err := ManagedDigest(absRoot)
	if err != nil {
		return Handshake{}, err
	}
	guidanceDigest, err := GuidanceDigest(absRoot)
	if err != nil {
		return Handshake{}, err
	}
	hs := Handshake{
		Version:               "1",
		Agent:                 config.Agent,
		Tools:                 allowed,
		Binary:                version.Get(),
		StateSchemaVersion:    StateSchemaVersion,
		ContextSchemaVersion:  ContextSchemaVersion,
		TemplateSchemaVersion: TemplateVersion,
		WorkspaceRoot:         filepath.Clean(absRoot),
		ManagedDigest:         managedDigest,
		GuidanceDigest:        guidanceDigest,
		ContextSchemaDigest:   ContextSchemaDigest(),
		NextCommands:          append([]string(nil), nextCommands...),
		Authority: HandshakeAuthority{
			HarnessInstructions: []string{"AGENTS.md", ".specd/roles", ".specd/steering", "command palette"},
			UntrustedInputs:     []string{"requirements", "source", "test_output", "adapter_observation"},
		},
		PaletteDigest: PaletteDigest(),
		ConfigDigest:  ConfigDigest(config),
		PolicyDigest:  PolicyDigest(config),
		ToolContracts: ManifestToolContracts(),
	}
	if state != nil {
		hs.ActiveSpec = &HandshakeSpec{Slug: state.Slug, Status: state.Status, Revision: state.Revision}
	}
	return hs, nil
}

// GuidanceDigest isolates managed guidance identity from palette/config and
// excludes user-owned bytes. ManagedDigest remains the compatibility name.
func GuidanceDigest(root string) (string, error) { return ManagedDigest(root) }

// ContextSchemaDigest pins semantic context metadata, not context payloads.
// Keep field order explicit: this contract is consumed by external hosts.
func ContextSchemaDigest() string {
	return digest(struct {
		Version string   `json:"version"`
		Fields  []string `json:"fields"`
	}{
		Version: ContextSchemaVersion,
		Fields:  []string{"kind", "path", "task_id", "role", "verify", "acceptance", "required", "mode", "bytes", "estimated_tokens", "reason", "priority", "digest"},
	})
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

// PolicyDigest is the SHA-256 of the effective lifecycle judgment policy: the
// profile and the criterion/review/integration gates it arms (spec 01 R7.2).
// It changes when the profile or any armed gate changes, so an approval pinned
// to it goes stale exactly when the policy that produced it moves.
func PolicyDigest(config Config) string {
	return digest(struct {
		Profile     string `json:"profile"`
		Criteria    bool   `json:"criteria"`
		Review      bool   `json:"review"`
		Integration bool   `json:"integration"`
	}{
		Profile:     config.Profile,
		Criteria:    config.CriteriaGateArmed(),
		Review:      config.ReviewGateArmed(),
		Integration: config.IntegrationPolicyArmed(),
	})
}

func digest(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
