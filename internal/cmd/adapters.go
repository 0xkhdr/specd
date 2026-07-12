package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/adapter"
	"github.com/0xkhdr/specd/internal/core"
)

// adaptersManifestPath is the read-only manifest of opt-in adapters a project
// has configured. It lives under .specd and carries no secret values — only
// names, paths, versions, and offered capabilities (R7.2/R6.3).
func adaptersManifestPath(root string) string {
	return filepath.Join(core.SpecdDir(root), "adapters.json")
}

// adapterState is the read-only inspection projection for one configured adapter
// (R7.2). It distinguishes configured, missing, incompatible, and disabled
// without loading any secret or executing anything.
type adapterState struct {
	Name         string   `json:"name"`
	Version      string   `json:"version,omitempty"`
	State        string   `json:"state"` // configured | missing | incompatible | disabled
	Capabilities []string `json:"capabilities,omitempty"`
	Detail       string   `json:"detail,omitempty"`
}

type adaptersReport struct {
	SchemaVersion string         `json:"schema_version"`
	Adapters      []adapterState `json:"adapters"`
}

// runAdapters implements `specd adapters [--json]`: a read-only projection of the
// configured adapter manifest distinguishing configured, missing, incompatible,
// and disabled adapters (R7.2). It loads no secrets and executes no adapter, so
// it is safe to run in any phase. A missing manifest is zero adapters — core
// stays fully usable with no adapters configured (R8.1).
func runAdapters(root string, args []string, flags map[string]string) error {
	if len(args) != 0 {
		return fmt.Errorf("%w: specd adapters [--json]", ErrUsage)
	}
	manifest, err := loadAdapterManifest(root)
	if err != nil {
		return err
	}
	report := adaptersReport{
		SchemaVersion: adapter.SchemaVersion,
		Adapters:      make([]adapterState, 0, len(manifest)),
	}
	for _, a := range manifest {
		report.Adapters = append(report.Adapters, classifyAdapter(a))
	}
	sort.Slice(report.Adapters, func(i, j int) bool {
		return report.Adapters[i].Name < report.Adapters[j].Name
	})

	if flagEnabled(flags, "json") {
		return writeJSON(report)
	}
	if len(report.Adapters) == 0 {
		fmt.Fprintln(os.Stdout, "no adapters configured")
		return nil
	}
	for _, a := range report.Adapters {
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", a.Name, a.State, a.Detail)
	}
	return nil
}

// classifyAdapter inspects one manifest entry read-only (R7.2). It never loads a
// secret and never runs the executable; incompatibility is decided from the
// declared schema version, presence from a stat/PATH lookup of the binary.
func classifyAdapter(a adapter.Adapter) adapterState {
	st := adapterState{Name: a.Name, Version: a.Version, Capabilities: a.Capabilities}
	switch {
	case !a.Enabled:
		st.State = "disabled"
		st.Detail = "disabled in manifest"
	case !binaryPresent(a.Path):
		st.State = "missing"
		st.Detail = "executable not found: " + a.Path
	case a.SchemaVersion != "" && a.SchemaVersion != adapter.SchemaVersion:
		st.State = "incompatible"
		st.Detail = "adapter schema " + a.SchemaVersion + " != " + adapter.SchemaVersion
	default:
		st.State = "configured"
	}
	return st
}

// binaryPresent reports whether an adapter executable can be located without
// running it. A path with a separator is stat'd; a bare name is resolved on PATH.
func binaryPresent(path string) bool {
	if path == "" {
		return false
	}
	if strings.ContainsRune(path, filepath.Separator) {
		info, err := os.Stat(path)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(path)
	return err == nil
}

// loadAdapterManifest reads the adapter manifest. A missing file is an empty
// manifest (offline continuity, R8.1); a malformed file is a fail-closed usage
// error rather than a silent empty projection.
func loadAdapterManifest(root string) ([]adapter.Adapter, error) {
	data, err := os.ReadFile(adaptersManifestPath(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var manifest []adapter.Adapter
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("%w: malformed adapter manifest: %v", ErrUsage, err)
	}
	return manifest, nil
}
