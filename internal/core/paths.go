package core

import (
	"os"
	"path/filepath"
)

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

func RequireSpecdRoot() (string, error) {
	root, ok := FindSpecdRoot("")
	if !ok {
		return "", NotFoundError("no .specd/ found in this directory or any parent. Run `specd init` first.")
	}
	return root, nil
}

func SpecdDir(root string) string      { return filepath.Join(root, ".specd") }
func SteeringDir(root string) string   { return filepath.Join(root, ".specd", "steering") }
func RolesDir(root string) string      { return filepath.Join(root, ".specd", "roles") }
func SkillsDir(root string) string     { return filepath.Join(root, ".specd", "skills") }
func SpecsDir(root string) string      { return filepath.Join(root, ".specd", "specs") }
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

func LegacyConfigPath(root string) string { return filepath.Join(root, ".specd", "config.json") }

// ConfigPath is the deprecated legacy JSON compatibility helper. New config
// discovery code should use ConfigPaths.
func ConfigPath(root string) string { return LegacyConfigPath(root) }

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

func IntegrationsPath(root string) string {
	return filepath.Join(root, ".specd", "integrations.json")
}
func AgentsPath(root string) string { return filepath.Join(root, "AGENTS.md") }
