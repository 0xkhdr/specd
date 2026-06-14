package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// fakeInstall lays down the artifacts install.sh creates under a temp HOME and
// returns that HOME. It also points os.UserHomeDir at it via $HOME.
func fakeInstall(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("uninstall path layout asserted on linux")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	installDir := filepath.Join(home, ".local", "share", "specd")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "specd"), []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "specd"), []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	rc := filepath.Join(home, ".bashrc")
	content := "alias ll='ls -l'\nexport PATH=\"${HOME}/.local/bin:${PATH}\" # specd\n"
	if err := os.WriteFile(rc, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestUninstallPreviewRemovesNothing(t *testing.T) {
	home := fakeInstall(t)

	if code := RunUninstall(cli.Args{}); code != core.ExitOK {
		t.Fatalf("preview exit = %d, want %d", code, core.ExitOK)
	}

	// Nothing should have been removed by a bare invocation.
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "specd")); err != nil {
		t.Errorf("install dir removed during preview: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(home, ".local", "bin", "specd")); err != nil {
		t.Errorf("bin link removed during preview: %v", err)
	}
	if !hasSpecdMarker(filepath.Join(home, ".bashrc")) {
		t.Error("PATH line cleaned during preview")
	}
}

func TestUninstallForceRemovesArtifacts(t *testing.T) {
	home := fakeInstall(t)

	if code := RunUninstall(cli.Args{Flags: map[string]string{"force": "true"}}); code != core.ExitOK {
		t.Fatalf("force exit = %d, want %d", code, core.ExitOK)
	}

	if _, err := os.Stat(filepath.Join(home, ".local", "share", "specd")); !os.IsNotExist(err) {
		t.Errorf("install dir not removed: err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(home, ".local", "bin", "specd")); !os.IsNotExist(err) {
		t.Errorf("bin link not removed: err=%v", err)
	}

	rc := filepath.Join(home, ".bashrc")
	if hasSpecdMarker(rc) {
		t.Error("PATH line not cleaned from .bashrc")
	}
	b, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "alias ll=") {
		t.Error("unrelated rc lines were dropped")
	}
	// Backup must capture the original "# specd" line.
	bak, err := os.ReadFile(rc + ".specd.bak")
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	if !strings.Contains(string(bak), "# specd") {
		t.Error("backup missing original PATH line")
	}
}

func TestUninstallNothingInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if code := RunUninstall(cli.Args{Flags: map[string]string{"force": "true"}}); code != core.ExitOK {
		t.Fatalf("exit = %d, want %d", code, core.ExitOK)
	}
}

func TestUninstallDryRunKeepsArtifacts(t *testing.T) {
	home := fakeInstall(t)

	if code := RunUninstall(cli.Args{Flags: map[string]string{"force": "true", "dry-run": "true"}}); code != core.ExitOK {
		t.Fatalf("dry-run exit = %d, want %d", code, core.ExitOK)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "specd")); err != nil {
		t.Errorf("dry-run removed install dir: %v", err)
	}
}
