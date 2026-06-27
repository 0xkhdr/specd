package core

import (
	"os"
	"path/filepath"
)

type GlobalConfigScaffoldResult struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
}

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
