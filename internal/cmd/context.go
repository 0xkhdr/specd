package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	speccontext "github.com/0xkhdr/specd/internal/context"
	"github.com/0xkhdr/specd/internal/core"
)

func runContext(root string, args []string, flags map[string]string) error {
	if len(args) != 2 {
		return usageError("context")
	}
	spec, err := loadSpec(root, args[0])
	if err != nil {
		return err
	}
	if err := checkMemoryBeforeContext(root, args[0]); err != nil {
		return err
	}
	manifest, err := speccontext.BuildManifest(root, args[0], spec.Tasks, args[1], contextBudget(root))
	if err != nil {
		return err
	}
	hud := flagEnabled(flags, "hud")
	asJSON := flagEnabled(flags, "json")
	if hud && asJSON {
		return fmt.Errorf("%w: --json and --hud are mutually exclusive; choose one render", ErrUsage)
	}
	if asJSON {
		config, _ := core.LoadConfig(configPaths(root), getenv())
		machine, err := speccontext.BuildMachineManifest(root, args[0], spec.Tasks, args[1], "context", "execute", contextBudget(root), core.BootstrapHandshake(config))
		if err != nil {
			return err
		}
		return writeJSON(machine)
	}
	if hud {
		fmt.Fprint(os.Stdout, speccontext.RenderHUD(manifest))
		return nil
	}
	for _, item := range manifest.Items {
		if item.Path != "" {
			fmt.Fprintln(os.Stdout, item.Path)
		}
	}
	return nil
}

func checkMemoryBeforeContext(root, slug string) error {
	cfg := loadSpecConfig(root)
	if !cfg.ProductionProfile() {
		return nil
	}
	var blocks []core.MemBlock
	for _, path := range []string{filepath.Join(core.SpecdDir(root), "steering", "memory.md"), core.SpecMemoryPath(root, slug)} {
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		parsed, err := core.IndexMemBlocks(string(raw))
		if err != nil {
			return fmt.Errorf("memory lint before context: %w", err)
		}
		blocks = append(blocks, parsed...)
	}
	findings := core.AnalyzeMemoryConflicts(blocks, core.Clock())
	if len(findings) > 0 {
		return errors.New("memory lint before context: " + findings[0].Message)
	}
	return nil
}
