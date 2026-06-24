package contextpkg

// Manifest version and soft-ceiling bounds for the mission context manifest.
// ManifestVersion, MinSoftCeiling, and MaxSoftCeiling are exported so core's
// boundary-layer validator (validateMissionContextManifest) can enforce the
// same bounds the engine emits. missionContextSoftCeiling is the engine's
// private default ceiling and stays unexported.
const (
	ManifestVersion           = 1
	missionContextSoftCeiling = 12000
	MinSoftCeiling            = 1000
	MaxSoftCeiling            = 200000
)

// MissionContextManifest is the deterministic context-engineering contract for
// a Pinky mission. It names exactly what a host should load, in order, and how
// aggressively to expand each item under the soft token ceiling. The manifest is
// advisory context, not completion evidence.
type MissionContextManifest struct {
	Version          int                  `json:"version"`
	SoftTokenCeiling int                  `json:"softTokenCeiling"`
	Strategy         string               `json:"strategy"`
	Items            []MissionContextItem `json:"items"`
	// EstimatedTokens and Budget are additive accounting fields (omitempty for
	// wire back-compat at version 1). EstimatedTokens is the sum of required-item
	// hints; Budget is the effective ceiling derived from phase, role, file count
	// and any host-negotiated cap. Omitting them reproduces the pre-feature bytes.
	EstimatedTokens int `json:"estimatedTokens,omitempty"`
	Budget          int `json:"budget,omitempty"`
}

type MissionContextItem struct {
	Order     int    `json:"order"`
	Kind      string `json:"kind"`
	Path      string `json:"path,omitempty"`
	Command   string `json:"command,omitempty"`
	Mode      string `json:"mode"`
	Required  bool   `json:"required"`
	TokenHint int    `json:"tokenHint,omitempty"`
	Rationale string `json:"rationale"`
}
