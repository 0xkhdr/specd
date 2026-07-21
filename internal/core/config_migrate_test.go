package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigMigrationDryRunAndReplay(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "project.yml")
	body := "version: 1\nagent: codex\n"
	if err := os.WriteFile(source, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	preview, err := PlanConfigMigration(root, "")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Completed || !preview.EffectiveEquivalent || len(preview.Operations) != 2 || preview.Permissions != "-rw-------" {
		t.Fatalf("preview = %#v", preview)
	}
	if _, err := os.Stat(preview.Target); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote canonical configuration")
	}
	result, err := MigrateConfig(root, "")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !result.Completed || result.EffectiveDigest != preview.EffectiveDigest {
		t.Fatalf("result = %#v", result)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("legacy source was not moved")
	}
	if info, err := os.Stat(result.Target); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("canonical permissions: info=%v err=%v", info, err)
	}
	replay, err := MigrateConfig(root, "project.yml")
	if err != nil || !replay.Completed || len(replay.Operations) != 0 {
		t.Fatalf("replay = %#v, %v", replay, err)
	}
}

func TestConfigMigrationRefusals(t *testing.T) {
	t.Run("dual-legacy-needs-explicit-equal-source", func(t *testing.T) {
		root := migrationRoot(t)
		for _, name := range []string{"project.yml", "project.yaml"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte("version: 1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := PlanConfigMigration(root, ""); err == nil || !strings.Contains(err.Error(), "select --source") {
			t.Fatalf("error = %v", err)
		}
		if _, err := PlanConfigMigration(root, "project.yaml"); err != nil {
			t.Fatalf("explicit equal source: %v", err)
		}
	})
	t.Run("dual-legacy-conflict", func(t *testing.T) {
		root := migrationRoot(t)
		_ = os.WriteFile(filepath.Join(root, "project.yml"), []byte("agent: codex\n"), 0o644)
		_ = os.WriteFile(filepath.Join(root, "project.yaml"), []byte("agent: pinky\n"), 0o644)
		if _, err := PlanConfigMigration(root, "project.yml"); err == nil || !strings.Contains(err.Error(), "agent") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("backup-collision", func(t *testing.T) {
		root := migrationRoot(t)
		source := filepath.Join(root, "project.yml")
		_ = os.WriteFile(source, []byte("version: 1\n"), 0o644)
		_ = os.WriteFile(source+legacyConfigBackupSuffix, []byte("owned\n"), 0o644)
		if _, err := MigrateConfig(root, ""); err == nil || !strings.Contains(err.Error(), "backup already exists") {
			t.Fatalf("error = %v", err)
		}
		if _, err := os.Stat(source); err != nil {
			t.Fatal("collision mutated source")
		}
	})
	t.Run("malformed", func(t *testing.T) {
		root := migrationRoot(t)
		_ = os.WriteFile(filepath.Join(root, "project.yml"), []byte("bad\n"), 0o644)
		if _, err := MigrateConfig(root, ""); err == nil || !strings.Contains(err.Error(), "config line 1") {
			t.Fatalf("error = %v", err)
		}
	})
}

func migrationRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}
