package adapter

// Adapter is the manifest of one opt-in, project-selected integration
// executable. It is data only: it is never linked into the process (the
// boundary invariant). The same manifest backs read-only `specd adapters`
// inspection, so it carries names, paths, versions, and offered capabilities —
// never secret values (R6.3).
type Adapter struct {
	Name          string   `json:"name"`
	Version       string   `json:"version,omitempty"`
	SchemaVersion string   `json:"schema_version,omitempty"` // adapter-envelope schema the executable speaks
	Path          string   `json:"path"`                     // executable resolved by the project
	Args          []string `json:"args,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"` // capabilities_offered by this adapter
	EnvAllow      []string `json:"env_allow,omitempty"`    // env var NAMES the adapter process may see
	Enabled       bool     `json:"enabled"`                // false ⇒ configured but disabled
}
