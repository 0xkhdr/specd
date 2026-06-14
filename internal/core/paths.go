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
func ConfigPath(root string) string    { return filepath.Join(root, ".specd", "config.json") }
func AgentsPath(root string) string    { return filepath.Join(root, "AGENTS.md") }
