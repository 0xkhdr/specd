package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONInvariantMachinePathsRemainJSON(t *testing.T) {
	root := t.TempDir()
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	checks := []string{ProgramPath(root), IntegrationsPath(root), LegacyConfigPath(root)}
	for _, fn := range []func() (string, error){
		func() (string, error) { return paths.SessionPath("0123456789abcdef0123456789abcdef") },
		func() (string, error) { return paths.ProgramSessionPath("0123456789abcdef0123456789abcdef") },
		func() (string, error) { return paths.ProgramStatePath("0123456789abcdef0123456789abcdef") },
	} {
		p, err := fn()
		if err != nil {
			t.Fatal(err)
		}
		checks = append(checks, p)
	}
	for _, p := range checks {
		if filepath.Ext(p) != ".json" {
			t.Fatalf("machine path must remain JSON: %s", p)
		}
		if strings.HasSuffix(p, ".yml") || strings.HasSuffix(p, ".yaml") {
			t.Fatalf("machine path switched to YAML: %s", p)
		}
	}
}
