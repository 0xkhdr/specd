package core

import (
	"os"
	"path/filepath"
)

// GlobalConfigScaffoldResult reports the outcome of
// EnsureGlobalConfigScaffold: the global config path it used and whether it
// had to create the file.
type GlobalConfigScaffoldResult struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
}

// EnsureGlobalConfigScaffold ensures a global config.yml exists, returning
// the path of the first existing candidate unchanged, or — if none exist —
// rendering the config.yml template (via readTemplate) to the first
// candidate path and reporting it as created.
func EnsureGlobalConfigScaffold(readTemplate func(string) (string, error)) (GlobalConfigScaffoldResult, error) {
	paths := GlobalConfigPaths()
	if len(paths) == 0 {
		return GlobalConfigScaffoldResult{}, os.ErrNotExist
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return GlobalConfigScaffoldResult{Path: p}, nil
		} else if err != nil && !os.IsNotExist(err) {
			return GlobalConfigScaffoldResult{Path: p}, err
		}
	}
	content, err := readTemplate("config.yml")
	if err != nil {
		return GlobalConfigScaffoldResult{Path: paths[0]}, err
	}
	if err := os.MkdirAll(filepath.Dir(paths[0]), 0o755); err != nil {
		return GlobalConfigScaffoldResult{Path: paths[0]}, err
	}
	if err := AtomicWrite(paths[0], content); err != nil {
		return GlobalConfigScaffoldResult{Path: paths[0]}, err
	}
	return GlobalConfigScaffoldResult{Path: paths[0], Created: true}, nil
}
