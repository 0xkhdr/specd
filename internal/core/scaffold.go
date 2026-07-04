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
	return nil
}
