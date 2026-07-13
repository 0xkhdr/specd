package verify

import (
	"fmt"
	"sort"
	"strings"
)

const (
	SandboxAdapterSchemaV1        = "sandbox-adapter/v1"
	CapabilityNetworkIsolation    = "network.isolated"
	CapabilityCredentialIsolation = "credentials.hidden"
	CapabilitySyntheticHome       = "home.synthetic"
	CapabilityWritableBoundary    = "filesystem.write-bounded"
	CapabilityResourceLimits      = "resources.bounded"
)

var RequiredSandboxCapabilities = []string{
	CapabilityCredentialIsolation,
	CapabilityNetworkIsolation,
	CapabilityResourceLimits,
	CapabilitySyntheticHome,
	CapabilityWritableBoundary,
}

var knownSandboxCapabilities = func() map[string]bool {
	m := make(map[string]bool, len(RequiredSandboxCapabilities))
	for _, capability := range RequiredSandboxCapabilities {
		m[capability] = true
	}
	return m
}()

// SandboxAdapterV1 is adapter-supplied metadata validated before execution.
// Binary and Args describe transport only; capabilities decide policy.
type SandboxAdapterV1 struct {
	SchemaVersion string   `json:"schema_version"`
	Name          string   `json:"name"`
	Platform      string   `json:"platform"`
	Binary        string   `json:"binary,omitempty"`
	Args          []string `json:"args,omitempty"`
	Capabilities  []string `json:"capabilities"`
}

// Validate negotiates sandbox policy without probing host state. Production
// accepts only complete capability declarations; unknown claims fail closed.
func (a SandboxAdapterV1) Validate(production bool) error {
	if a.SchemaVersion != SandboxAdapterSchemaV1 {
		return fmt.Errorf("unsupported sandbox adapter schema %q", a.SchemaVersion)
	}
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("sandbox adapter name is required")
	}
	switch a.Platform {
	case "linux", "darwin", "ci":
	default:
		return fmt.Errorf("unsupported sandbox adapter platform %q", a.Platform)
	}
	seen := make(map[string]bool, len(a.Capabilities))
	for _, capability := range a.Capabilities {
		if !knownSandboxCapabilities[capability] {
			return fmt.Errorf("unknown sandbox capability %q", capability)
		}
		if seen[capability] {
			return fmt.Errorf("duplicate sandbox capability %q", capability)
		}
		seen[capability] = true
	}
	if production {
		for _, required := range RequiredSandboxCapabilities {
			if !seen[required] {
				return fmt.Errorf("sandbox adapter %q missing required capability %q", a.Name, required)
			}
		}
	}
	return nil
}

// CanonicalCapabilities returns stable order for attestations and reports.
func (a SandboxAdapterV1) CanonicalCapabilities() []string {
	out := append([]string(nil), a.Capabilities...)
	sort.Strings(out)
	return out
}
