package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const legacyConfigBackupSuffix = ".specd-v1.bak"

type ConfigMigrationOperation struct {
	Action string `json:"action"`
	Path   string `json:"path"`
}

// ConfigMigration is both the dry-run plan and the digest-only completion
// result. EffectiveEquivalent is computed by parsing both inputs, never by
// comparing formatting.
type ConfigMigration struct {
	Source              string                     `json:"source"`
	Target              string                     `json:"target"`
	Backup              string                     `json:"backup"`
	Permissions         string                     `json:"permissions"`
	SourceDigest        string                     `json:"source_digest"`
	EffectiveDigest     string                     `json:"effective_digest"`
	EffectiveEquivalent bool                       `json:"effective_equivalent"`
	Conflicts           []string                   `json:"conflicts"`
	Operations          []ConfigMigrationOperation `json:"operations"`
	Completed           bool                       `json:"completed"`
}

// PlanConfigMigration returns the exact plan used by MigrateConfig and writes
// nothing. source is required only when both legacy spellings exist.
func PlanConfigMigration(cwd, source string) (ConfigMigration, error) {
	root, err := FindRoot(cwd)
	if err != nil {
		return ConfigMigration{}, err
	}
	return planConfigMigration(root, source)
}

// MigrateConfig atomically installs canonical policy, validates effective
// equivalence, then preserves the legacy source as a non-overwriting backup.
func MigrateConfig(cwd, source string) (ConfigMigration, error) {
	root, err := FindRoot(cwd)
	if err != nil {
		return ConfigMigration{}, err
	}
	return WithSpecLock(root, func() (ConfigMigration, error) {
		plan, err := planConfigMigration(root, source)
		if err != nil || plan.Completed {
			return plan, err
		}
		raw, err := os.ReadFile(plan.Source)
		if err != nil {
			return ConfigMigration{}, fmt.Errorf("read migration source %s: %w", plan.Source, err)
		}
		if _, err := os.Stat(plan.Target); os.IsNotExist(err) {
			mode, err := sourceMode(plan.Source)
			if err != nil {
				return ConfigMigration{}, err
			}
			if err := atomicWriteConfig(plan.Target, raw, mode); err != nil {
				return ConfigMigration{}, err
			}
		}
		targetRaw, err := os.ReadFile(plan.Target)
		if err != nil {
			return ConfigMigration{}, fmt.Errorf("revalidate canonical configuration: %w", err)
		}
		targetValues, err := parseSimpleYAML(string(targetRaw))
		if err != nil {
			return ConfigMigration{}, fmt.Errorf("revalidate canonical configuration: %w", err)
		}
		if digestConfigValues(targetValues) != plan.EffectiveDigest {
			return ConfigMigration{}, fmt.Errorf("canonical configuration is not effectively equivalent to %s", plan.Source)
		}
		if err := os.Rename(plan.Source, plan.Backup); err != nil {
			return ConfigMigration{}, fmt.Errorf("preserve legacy configuration: %w", err)
		}
		if err := syncDir(root); err != nil {
			return ConfigMigration{}, fmt.Errorf("sync project root: %w", err)
		}
		plan.Completed = true
		return plan, nil
	})
}

func planConfigMigration(root, requested string) (ConfigMigration, error) {
	canonical := filepath.Join(root, ".specd", "config.yaml")
	legacy := []string{filepath.Join(root, "project.yml"), filepath.Join(root, "project.yaml")}
	present := make([]string, 0, 2)
	for _, path := range legacy {
		if _, err := os.Stat(path); err == nil {
			present = append(present, path)
		} else if !os.IsNotExist(err) {
			return ConfigMigration{}, fmt.Errorf("inspect configuration %s: %w", path, err)
		}
	}
	requestedPath, err := migrationSource(root, requested)
	if err != nil {
		return ConfigMigration{}, err
	}
	if len(present) == 2 && requestedPath == "" {
		return ConfigMigration{}, fmt.Errorf("both legacy configurations exist; select --source project.yml or --source project.yaml")
	}
	if requestedPath != "" {
		found := false
		for _, path := range present {
			found = found || path == requestedPath
		}
		if !found {
			return completedMigration(root, canonical, requestedPath)
		}
		present = []string{requestedPath}
	}
	if len(present) == 0 {
		for _, path := range legacy {
			if plan, done := completedMigration(root, canonical, path); done == nil {
				return plan, nil
			}
		}
		return ConfigMigration{}, fmt.Errorf("no legacy configuration to migrate")
	}
	source := present[0]
	if len(legacy) == 2 {
		other := legacy[0]
		if other == source {
			other = legacy[1]
		}
		if raw, readErr := os.ReadFile(other); readErr == nil {
			a, err := readConfigValues(source)
			if err != nil {
				return ConfigMigration{}, err
			}
			b, err := parseSimpleYAML(string(raw))
			if err != nil {
				return ConfigMigration{}, fmt.Errorf("%s: %w", other, err)
			}
			if conflicts := sortedConfigConflicts(a, b); len(conflicts) != 0 {
				return ConfigMigration{}, ConfigConflictError{Paths: []string{source, other}, Keys: conflicts}
			}
		}
	}
	values, err := readConfigValues(source)
	if err != nil {
		return ConfigMigration{}, err
	}
	if err := validateMigrationConfig(source); err != nil {
		return ConfigMigration{}, err
	}
	raw, _ := os.ReadFile(source)
	mode, err := sourceMode(source)
	if err != nil {
		return ConfigMigration{}, err
	}
	backup := source + legacyConfigBackupSuffix
	if _, err := os.Stat(backup); err == nil {
		return ConfigMigration{}, fmt.Errorf("migration backup already exists: %s", backup)
	} else if !os.IsNotExist(err) {
		return ConfigMigration{}, fmt.Errorf("inspect migration backup %s: %w", backup, err)
	}
	plan := ConfigMigration{Source: source, Target: canonical, Backup: backup, Permissions: mode.String(), SourceDigest: digestBytes(raw), EffectiveDigest: digestConfigValues(values), EffectiveEquivalent: true, Conflicts: []string{}}
	if target, readErr := os.ReadFile(canonical); readErr == nil {
		targetValues, parseErr := parseSimpleYAML(string(target))
		if parseErr != nil {
			return ConfigMigration{}, fmt.Errorf("%s: %w", canonical, parseErr)
		}
		if conflicts := sortedConfigConflicts(values, targetValues); len(conflicts) != 0 {
			return ConfigMigration{}, ConfigConflictError{Paths: []string{source, canonical}, Keys: conflicts}
		}
		if err := validateMigrationConfig(canonical); err != nil {
			return ConfigMigration{}, err
		}
	} else if os.IsNotExist(readErr) {
		plan.Operations = append(plan.Operations, ConfigMigrationOperation{Action: "write-canonical", Path: canonical})
	} else {
		return ConfigMigration{}, fmt.Errorf("read canonical configuration %s: %w", canonical, readErr)
	}
	plan.Operations = append(plan.Operations, ConfigMigrationOperation{Action: "backup-legacy", Path: backup})
	return plan, nil
}

func completedMigration(root, canonical, source string) (ConfigMigration, error) {
	backup := source + legacyConfigBackupSuffix
	canonicalRaw, canonicalErr := os.ReadFile(canonical)
	backupRaw, backupErr := os.ReadFile(backup)
	if canonicalErr != nil || backupErr != nil {
		return ConfigMigration{}, fmt.Errorf("selected legacy configuration does not exist: %s", source)
	}
	a, err := parseSimpleYAML(string(canonicalRaw))
	if err != nil {
		return ConfigMigration{}, fmt.Errorf("%s: %w", canonical, err)
	}
	b, err := parseSimpleYAML(string(backupRaw))
	if err != nil {
		return ConfigMigration{}, fmt.Errorf("%s: %w", backup, err)
	}
	if conflicts := sortedConfigConflicts(a, b); len(conflicts) != 0 {
		return ConfigMigration{}, ConfigConflictError{Paths: []string{canonical, backup}, Keys: conflicts}
	}
	mode, err := sourceMode(backup)
	if err != nil {
		return ConfigMigration{}, err
	}
	return ConfigMigration{Source: source, Target: canonical, Backup: backup, Permissions: mode.String(), SourceDigest: digestBytes(backupRaw), EffectiveDigest: digestConfigValues(a), EffectiveEquivalent: true, Conflicts: []string{}, Operations: []ConfigMigrationOperation{}, Completed: true}, nil
}

func migrationSource(root, source string) (string, error) {
	if source == "" {
		return "", nil
	}
	if source != "project.yml" && source != "project.yaml" {
		return "", fmt.Errorf("migration source must be project.yml or project.yaml")
	}
	return filepath.Join(root, source), nil
}

func readConfigValues(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration %s: %w", path, err)
	}
	values, err := parseSimpleYAML(string(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return values, nil
}

func validateMigrationConfig(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	values, err := parseSimpleYAML(string(raw))
	if err != nil {
		return err
	}
	cfg := DefaultConfig
	var diagnostics []Diagnostic
	applyConfigMap(&cfg, values, path, &diagnostics)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return fmt.Errorf("%s: %s", path, diagnostic.Message)
		}
	}
	return nil
}

func sortedConfigConflicts(a, b map[string]string) []string {
	conflicts := differingConfigKeys(a, b)
	keys := make([]string, 0, len(conflicts))
	for key := range conflicts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sourceMode(path string) (os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("stat configuration %s: %w", path, err)
	}
	return info.Mode().Perm(), nil
}

func atomicWriteConfig(path string, raw []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".config.*.tmp")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	if err == nil {
		err = file.Chmod(mode)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write canonical configuration: %w", err)
	}
	_, err = parseSimpleYAML(string(raw))
	if err != nil {
		return fmt.Errorf("validate canonical configuration: %w", err)
	}
	if err := validateMigrationConfig(temp); err != nil {
		return fmt.Errorf("validate canonical configuration: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		return fmt.Errorf("install canonical configuration: %w", err)
	}
	return syncDir(dir)
}
