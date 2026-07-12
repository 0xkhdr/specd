package verify

import (
	"context"
	"strings"
	"testing"
)

// TestScrubbedEnvDropsSecrets pins the log/leak boundary (T-04-04): verify lines
// run with a minimal allowlisted environment (HOME/PATH/TMPDIR only). A secret
// exported into specd's own process must never cross into the shelled-out verify
// command, where it could be echoed into evidence logs or CI output.
func TestScrubbedEnvDropsSecrets(t *testing.T) {
	in := []string{
		"HOME=/home/u", "PATH=/usr/bin", "TMPDIR=/tmp",
		"AWS_SECRET_ACCESS_KEY=AKIAABCDEFGHIJKLMNOP",
		"GITHUB_TOKEN=ghp_deadbeef", "MY_API_KEY=hunter2",
		"HOMEBREW=x", // must not match on HOME prefix
	}
	out := scrubbedEnv(in)
	got := strings.Join(out, "\n")
	for _, secret := range []string{"AWS_SECRET_ACCESS_KEY", "GITHUB_TOKEN", "MY_API_KEY", "HOMEBREW", "AKIAABCDEFGHIJKLMNOP", "ghp_deadbeef", "hunter2"} {
		if strings.Contains(got, secret) {
			t.Errorf("scrubbed env leaked %q: %v", secret, out)
		}
	}
	for _, keep := range []string{"HOME=/home/u", "PATH=/usr/bin", "TMPDIR=/tmp"} {
		if !strings.Contains(got, keep) {
			t.Errorf("scrubbed env dropped required var %q: %v", keep, out)
		}
	}
}

func TestSandboxFailClosed(t *testing.T) {
	_, err := Run(context.Background(), Options{
		Command:       "true",
		Dir:           t.TempDir(),
		Sandbox:       true,
		SandboxBinary: "definitely-missing-specd-sandbox",
	})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("Run sandbox error = %v, want fail closed", err)
	}
}

// TestSandboxWrapArgv pins the isolation contract: when a sandbox binary is
// present the verify command is actually wrapped (read-only root, no network,
// repo bound writable), not run bare. The bug this guards against was a
// presence-check that resolved the binary then discarded it, silently running
// unsandboxed.
func TestSandboxWrapArgv(t *testing.T) {
	name, args := sandboxArgv("/usr/bin/bwrap", "/repo", "/home/host", "go test ./...", Limits{})
	if name != "/usr/bin/bwrap" {
		t.Fatalf("sandbox argv[0] = %q, want the sandbox binary", name)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"--unshare-all", "--ro-bind / /", "--tmpfs /tmp", "--tmpfs /home/host", "--bind /repo /repo", "--chdir /repo", "--setenv HOME /tmp/specd-home", "--setenv PATH /usr/local/bin:/usr/bin:/bin", "/bin/sh -c go test ./..."} {
		if !strings.Contains(joined, want) {
			t.Errorf("sandbox argv missing %q; got %q", want, joined)
		}
	}
	// Unsandboxed path stays a bare shell invocation.
	n2, a2 := sandboxArgv("", "/repo", "/home/host", "go test", Limits{})
	if n2 != "/bin/sh" || len(a2) != 2 || a2[0] != "-c" || a2[1] != "go test" {
		t.Fatalf("unsandboxed argv = %q %v, want /bin/sh [-c \"go test\"]", n2, a2)
	}
}

func TestSandboxEnvIsSynthetic(t *testing.T) {
	got := strings.Join(scrubbedEnv([]string{"HOME=/host", "PATH=/evil", "TMPDIR=/host/tmp", "AWS_PROFILE=prod"}, true), "\n")
	if got != "HOME=/tmp/specd-home\nPATH=/usr/local/bin:/usr/bin:/bin\nTMPDIR=/tmp" {
		t.Fatalf("sandbox env = %q", got)
	}
}

func TestSandboxRecreatesRepoBelowHiddenHome(t *testing.T) {
	_, args := sandboxArgv("/usr/bin/bwrap", "/home/host/src/repo", "/home/host", "true", Limits{})
	joined := strings.Join(args, " ")
	for _, want := range []string{"--tmpfs /home/host", "--dir /home/host/src", "--dir /home/host/src/repo", "--bind /home/host/src/repo /home/host/src/repo"} {
		if !strings.Contains(joined, want) {
			t.Errorf("sandbox nested repo missing %q: %s", want, joined)
		}
	}
}

func TestSandboxProductionRequired(t *testing.T) {
	_, err := Run(context.Background(), Options{Command: "touch should-not-exist", Dir: t.TempDir(), RequireSandbox: true, SandboxBinary: "definitely-missing-specd-sandbox"})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("production sandbox error = %v", err)
	}
}
