package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// shellConfigs are the rc files install.sh appends a "# specd" PATH line to.
var shellConfigs = []string{".bashrc", ".zshrc", ".profile"}

// uninstallPlan is the deterministic view of what `specd uninstall` would
// remove, derived entirely from $HOME (never from os.Executable). Keeping the
// running binary out of the calculation means the command never self-deletes
// the in-place dev/test binary — it only removes the artifacts install.sh
// created under HOME, mirroring scripts/uninstall.sh.
type uninstallPlan struct {
	InstallDir string   `json:"installDir,omitempty"`
	BinLink    string   `json:"binLink,omitempty"`
	PathFiles  []string `json:"pathFiles"`
}

func (p uninstallPlan) empty() bool {
	return p.InstallDir == "" && p.BinLink == "" && len(p.PathFiles) == 0
}

// installDirFor returns the directory install.sh extracts the repo/binary into,
// keyed on platform exactly as the shell script does.
func installDirFor(home string) string {
	switch runtime.GOOS {
	case "linux":
		// Linux and Windows/WSL both use the XDG-style share dir.
		return filepath.Join(home, ".local", "share", "specd")
	case "darwin":
		return filepath.Join(home, ".specd-repo")
	default:
		return filepath.Join(home, ".specd-repo")
	}
}

// computeUninstallPlan inspects HOME and returns only the artifacts that
// actually exist, so the preview and the removal agree on a single source.
func computeUninstallPlan(home string) uninstallPlan {
	var p uninstallPlan

	installDir := installDirFor(home)
	if fi, err := os.Stat(installDir); err == nil && fi.IsDir() {
		p.InstallDir = installDir
	}

	binLink := filepath.Join(home, ".local", "bin", "specd")
	if _, err := os.Lstat(binLink); err == nil {
		p.BinLink = binLink
	}

	for _, name := range shellConfigs {
		rc := filepath.Join(home, name)
		if hasSpecdMarker(rc) {
			p.PathFiles = append(p.PathFiles, rc)
		}
	}
	return p
}

// hasSpecdMarker reports whether rc contains a "# specd" PATH line.
func hasSpecdMarker(rc string) bool {
	b, err := os.ReadFile(rc)
	if err != nil {
		return false
	}
	return strings.Contains(string(b), "# specd")
}

// cleanPathFile rewrites rc with every "# specd" line dropped, after backing
// the original up to rc+".specd.bak" — the same contract as uninstall.sh.
func cleanPathFile(rc string) error {
	b, err := os.ReadFile(rc)
	if err != nil {
		return err
	}
	if err := os.WriteFile(rc+".specd.bak", b, 0o644); err != nil {
		return fmt.Errorf("backup %s: %w", rc, err)
	}
	var kept []string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "# specd") {
			continue
		}
		kept = append(kept, line)
	}
	return os.WriteFile(rc, []byte(strings.Join(kept, "\n")), 0o644)
}

func RunUninstall(args cli.Args) int {
	jsonOut := args.Bool("json")
	dryRun := args.Bool("dry-run")
	force := args.Bool("force")

	home, err := os.UserHomeDir()
	if err != nil {
		core.Error(fmt.Sprintf("cannot resolve home directory: %v", err))
		return core.ExitGate
	}

	plan := computeUninstallPlan(home)

	if plan.empty() {
		if jsonOut {
			if err := core.PrintJSON(map[string]any{"kind": "uninstall", "removed": false, "reason": "nothing-installed", "plan": plan}); err != nil {
				return specdExit(err)
			}
			return core.ExitOK
		}
		core.Info("Nothing to uninstall.")
		return core.ExitOK
	}

	// Without --force (and not JSON), preview the plan and stop — a destructive
	// self-removal should never be the default of a bare invocation.
	if !force || dryRun {
		if jsonOut {
			if err := core.PrintJSON(map[string]any{"kind": "uninstall", "removed": false, "reason": "preview", "plan": plan}); err != nil {
				return specdExit(err)
			}
			return core.ExitOK
		}
		core.Header("uninstall plan")
		if plan.InstallDir != "" {
			fmt.Printf("  remove dir   %s\n", plan.InstallDir)
		}
		if plan.BinLink != "" {
			fmt.Printf("  remove link  %s\n", plan.BinLink)
		}
		for _, rc := range plan.PathFiles {
			fmt.Printf("  clean PATH   %s (backup → %s.specd.bak)\n", rc, rc)
		}
		core.Divider()
		if dryRun {
			core.Info("Dry run — nothing removed.")
		} else {
			core.Warn("Re-run with --force to remove the above.")
		}
		return core.ExitOK
	}

	// --- Perform removal (--force) ---
	var failed bool

	if plan.InstallDir != "" {
		if err := os.RemoveAll(plan.InstallDir); err != nil {
			core.Error(fmt.Sprintf("remove %s: %v", plan.InstallDir, err))
			failed = true
		}
	}
	if plan.BinLink != "" {
		if err := os.Remove(plan.BinLink); err != nil && !os.IsNotExist(err) {
			core.Error(fmt.Sprintf("remove %s: %v", plan.BinLink, err))
			failed = true
		}
	}
	for _, rc := range plan.PathFiles {
		if err := cleanPathFile(rc); err != nil {
			core.Error(fmt.Sprintf("clean %s: %v", rc, err))
			failed = true
		}
	}

	if jsonOut {
		if err := core.PrintJSON(map[string]any{"kind": "uninstall", "removed": !failed, "plan": plan}); err != nil {
			return specdExit(err)
		}
		if failed {
			return core.ExitGate
		}
		return core.ExitOK
	}

	if failed {
		core.Error("Uninstall completed with errors.")
		return core.ExitGate
	}
	core.Success("Uninstallation complete.")
	core.Warn("Any local project '.specd/' directories have been preserved.")
	return core.ExitOK
}
