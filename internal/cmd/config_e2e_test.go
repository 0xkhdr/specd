package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func TestConfigInitE2E(t *testing.T) {
	t.Run("init_creates_project_and_global_yaml", func(t *testing.T) {
		root := initTestRoot(t)
		xdg := filepath.Join(t.TempDir(), "xdg")
		t.Setenv("XDG_CONFIG_HOME", xdg)
		t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))
		_, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{"--agent", "none"}), core.DefaultInitExecutor())
		if code != core.ExitOK || stderr != "" {
			t.Fatalf("exit=%d stderr=%q", code, stderr)
		}
		for _, p := range []string{filepath.Join(root, ".specd", "config.yml"), filepath.Join(xdg, "specd", "config.yml")} {
			if _, err := os.Stat(p); err != nil {
				t.Fatalf("missing YAML config %s: %v", p, err)
			}
		}
	})
}
