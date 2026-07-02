package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWave4ConfigLoadsSecurityAndEscalation(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".specd")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := []byte(`{
		"security": {
			"secrets": "block",
			"injection": "warn",
			"slopsquat": "off"
		},
		"escalation": {
			"enabled": true,
			"verifyFailThreshold": 4,
			"blockerThreshold": 2,
			"complexityThreshold": 9.5
		},
		"orchestration": {
			"maxWorkers": 3,
			"resilience": {
				"autoResume": {
					"enabled": true,
					"maxAgeMinutes": 15
				}
			}
		}
	}`)
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), config, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, result := LoadConfigWithDiagnostics(root)
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for unsupported fixture values")
	}
	if cfg.Security.Secrets != "block" || cfg.Security.Injection != "warn" || cfg.Security.Slopsquat != "off" {
		t.Fatalf("security config = %#v", cfg.Security)
	}
	if !cfg.Escalation.Enabled || cfg.Escalation.VerifyFailThreshold != 4 || cfg.Escalation.BlockerThreshold != 2 {
		t.Fatalf("escalation config = %#v", cfg.Escalation)
	}
	if cfg.Orchestration.MaxWorkers != 3 || !cfg.Orchestration.Resilience.AutoResume.Enabled {
		t.Fatalf("orchestration config = %#v", cfg.Orchestration)
	}
}
