package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The shell uninstaller (scripts/uninstall.sh) and the in-binary `specd
// uninstall` (uninstall.go) clean up the SAME install, so they must agree on
// every artifact path, the rc marker, and the backup suffix. uninstall.go's
// comments explicitly claim to "mirror scripts/uninstall.sh"; this test makes
// that claim load-bearing — change one side without the other and CI fails.
//
// The matrix runs on linux and macOS, so installDirFor() returns each
// platform's real path on its own runner and we check it against the script
// there. The literal cross-platform fragments below are asserted regardless of
// host so a single-OS run still catches the common cases.
func TestUninstallScriptInSyncWithCommand(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "scripts", "uninstall.sh"))
	if err != nil {
		t.Fatalf("read uninstall.sh: %v", err)
	}
	script := string(b)

	// The install dir for THIS platform, derived from the Go command itself so
	// editing installDirFor without editing the script breaks the build here.
	const sentinelHome = "/__home__"
	goInstallDir := installDirFor(sentinelHome)
	hostFrag := strings.TrimPrefix(goInstallDir, sentinelHome)
	if !strings.Contains(script, hostFrag) {
		t.Errorf("uninstall.sh missing install dir %q (installDirFor on %s) — drifted from uninstall.go",
			hostFrag, runtime.GOOS)
	}

	// Shared, platform-independent contract. These appear in both
	// implementations; if either side renames one, the pair has drifted.
	shared := []string{
		"/.local/bin/specd",   // bin link (computeUninstallPlan)
		"/.local/share/specd", // linux/WSL install dir (installDirFor)
		"/.specd-repo",        // darwin/default install dir (installDirFor)
		".specd.bak",          // backup suffix (cleanPathFile)
		"# specd",             // rc marker (hasSpecdMarker / cleanPathFile)
		"/proc/version",       // WSL probe (uninstall.sh)
	}
	for _, frag := range shared {
		if !strings.Contains(script, frag) {
			t.Errorf("uninstall.sh missing %q — drifted from uninstall.go", frag)
		}
	}

	// Every rc file the Go command rewrites must be one the script also cleans.
	for _, rc := range shellConfigs {
		if !strings.Contains(script, rc) {
			t.Errorf("uninstall.sh does not clean %q but shellConfigs lists it", rc)
		}
	}
}
