package core

import (
	"encoding/json"
	"path/filepath"
)

// ContextManifestTools is the per-spec MCP tool policy declared in a spec's
// manifest.json (context-manifest spec C1). It is the most precise exposure
// layer, composed after the config/phase filter: required/optional define an
// allowlist, forbidden a hard exclude. Tool names use the MCP specd_*/brain_*
// namespace (post-composite), matching the names emitted on tools/list.
type ContextManifestTools struct {
	RequiredTools  []string `json:"requiredTools"`
	OptionalTools  []string `json:"optionalTools"`
	ForbiddenTools []string `json:"forbiddenTools"`
}

// Present reports whether the manifest declared any tool policy. An absent or
// empty manifest leaves MCP exposure to the config/phase plan unchanged (R5).
func (m ContextManifestTools) Present() bool {
	return len(m.RequiredTools) > 0 || len(m.OptionalTools) > 0 || len(m.ForbiddenTools) > 0
}

func contextManifestPath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), "manifest.json")
}

// LoadContextManifest reads a spec's manifest.json tool policy. It is read-only
// and deterministic (R6): a missing or malformed file yields an empty policy so
// the MCP server degrades to the config/phase plan rather than erroring. The
// policy lives under the optional top-level "contextManifest" key (spec §5.1),
// keeping manifest.json open to other future sections.
func LoadContextManifest(root, slug string) ContextManifestTools {
	raw := ReadOrNull(contextManifestPath(root, slug))
	if raw == nil {
		return ContextManifestTools{}
	}
	var wrapper struct {
		ContextManifest ContextManifestTools `json:"contextManifest"`
	}
	if err := json.Unmarshal([]byte(*raw), &wrapper); err != nil {
		return ContextManifestTools{}
	}
	return wrapper.ContextManifest
}
