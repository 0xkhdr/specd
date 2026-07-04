package core

import (
	"os"
	"path/filepath"

	embedtemplates "github.com/0xkhdr/specd/internal/core/embed_templates"
)

func WriteScaffold(root string) error {
	for _, dir := range []string{".specd/roles", ".specd/steering"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return err
		}
	}
	for _, base := range []string{"roles", "steering"} {
		entries, err := embedtemplates.FS.ReadDir(base)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := filepath.Join(base, entry.Name())
			raw, err := embedtemplates.FS.ReadFile(name)
			if err != nil {
				return err
			}
			target := filepath.Join(root, ".specd", name)
			if _, err := os.Stat(target); err == nil {
				continue
			} else if !os.IsNotExist(err) {
				return err
			}
			if err := os.WriteFile(target, raw, 0o644); err != nil {
				return err
			}
		}
	}
	return writeAgents(root)
}

// writeAgents materializes AGENTS.md at the project root, merging into any
// existing file so user-authored content outside the managed markers survives
// (Spec 06 R6.3/R6.4). Idempotent: re-running replaces only the marked block.
func writeAgents(root string) error {
	generated, err := embedtemplates.FS.ReadFile("AGENTS.md")
	if err != nil {
		return err
	}
	target := filepath.Join(root, "AGENTS.md")
	existing, err := os.ReadFile(target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return AtomicWrite(target, MergeAgents(string(existing), string(generated)))
}
