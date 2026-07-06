package verify

import (
	"context"
	"strings"
	"testing"
)

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
	name, args := wrapArgv("/usr/bin/bwrap", "/repo", "go test ./...")
	if name != "/usr/bin/bwrap" {
		t.Fatalf("sandbox argv[0] = %q, want the sandbox binary", name)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"--unshare-all", "--ro-bind / /", "--tmpfs /tmp", "--bind /repo /repo", "--chdir /repo", "/bin/sh -c go test ./..."} {
		if !strings.Contains(joined, want) {
			t.Errorf("sandbox argv missing %q; got %q", want, joined)
		}
	}
	// Unsandboxed path stays a bare shell invocation.
	n2, a2 := wrapArgv("", "/repo", "go test")
	if n2 != "/bin/sh" || len(a2) != 2 || a2[0] != "-c" || a2[1] != "go test" {
		t.Fatalf("unsandboxed argv = %q %v, want /bin/sh [-c \"go test\"]", n2, a2)
	}
}
