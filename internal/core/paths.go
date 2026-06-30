package core

import (
	"os"
	"path/filepath"
)

// FindSpecdRoot walks upward from start (or the current working directory
// when start is empty) looking for a .specd directory, returning the
// containing root and true if found, or false if it reaches the filesystem
// root without finding one.
func FindSpecdRoot(start string) (string, bool) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", false
		}
	}
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, ".specd")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// RequireSpecdRoot resolves the specd root for the current working
// directory, returning a NotFoundError instructing the user to run
// `specd init` if none exists.
func RequireSpecdRoot() (string, error) {
	root, ok := FindSpecdRoot("")
	if !ok {
		return "", NotFoundError("no .specd/ found in this directory or any parent. Run `specd init` first.")
	}
	return root, nil
}

// SpecdDir returns the path to root's .specd directory.
func SpecdDir(root string) string { return filepath.Join(root, ".specd") }

// SteeringDir returns the path to root's .specd/steering directory.
func SteeringDir(root string) string { return filepath.Join(root, ".specd", "steering") }

// RolesDir returns the path to root's .specd/roles directory.
func RolesDir(root string) string { return filepath.Join(root, ".specd", "roles") }

// SkillsDir returns the path to root's .specd/skills directory.
func SkillsDir(root string) string { return filepath.Join(root, ".specd", "skills") }

// SpecsDir returns the path to root's .specd/specs directory.
func SpecsDir(root string) string { return filepath.Join(root, ".specd", "specs") }

// SpecDir returns the path to the spec directory for slug under root's
// .specd/specs directory.
func SpecDir(root, slug string) string { return filepath.Join(root, ".specd", "specs", slug) }

// ConfigPaths returns project config candidates in priority order. YAML is the
// human-authored default; JSON is retained as the legacy compatibility path.
func ConfigPaths(root string) []string {
	return []string{
		filepath.Join(root, ".specd", "config.yml"),
		filepath.Join(root, ".specd", "config.yaml"),
		LegacyConfigPath(root),
	}
}

// LegacyConfigPath returns the path to root's legacy .specd/config.json file.
func LegacyConfigPath(root string) string { return filepath.Join(root, ".specd", "config.json") }

// ConfigPath is the deprecated legacy JSON compatibility helper. New config
// discovery code should use ConfigPaths.
func ConfigPath(root string) string { return LegacyConfigPath(root) }

// GlobalConfigPaths returns the candidate paths for a user-global specd
// config, checked in priority order across the OS config directory and the
// user's home directory, in both YAML and JSON forms.
func GlobalConfigPaths() []string {
	paths := []string{}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		paths = append(paths,
			filepath.Join(dir, "specd", "config.yml"),
			filepath.Join(dir, "specd", "config.yaml"),
			filepath.Join(dir, "specd", "config.json"),
		)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".specd.yml"), filepath.Join(home, ".specd.yaml"), filepath.Join(home, ".specd.json"))
	}
	return paths
}

// IntegrationsPath returns the path to root's .specd/integrations.json file.
func IntegrationsPath(root string) string {
	return filepath.Join(root, ".specd", "integrations.json")
}

// AgentsPath returns the path to root's AGENTS.md file.
func AgentsPath(root string) string { return filepath.Join(root, "AGENTS.md") }
