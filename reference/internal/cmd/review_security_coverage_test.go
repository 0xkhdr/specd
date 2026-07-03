package cmd_test

import (
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

func TestApproveSecurityCleanPasses(t *testing.T) {
	h := th.New(t)
	h.Spec("auth").Req("Login", "As a user I can log in.").Build()

	res := h.Run("approve", "auth", "--security")
	if res.Code != core.ExitOK {
		t.Fatalf("approve exit=%d stdout=%q stderr=%q", res.Code, res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "approved") {
		t.Fatalf("expected approval output, got stdout=%q stderr=%q", res.Stdout, res.Stderr)
	}
}

func TestSubmitDryRunReportsGateWithIncompleteSpec(t *testing.T) {
	h := th.New(t)
	h.Spec("auth").Req("Login", "As a user I can log in.").Build()
	if err := os.WriteFile(core.ArtifactPath(h.Root, "auth", "design.md"), []byte("## Design\n\nREQ-1 is implemented by T1.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := h.Run("submit", "auth", "--dry-run", "--force", "--json", "true")
	if res.Code != core.ExitGate {
		t.Fatalf("submit exit=%d stdout=%q stderr=%q", res.Code, res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "submit blocked") {
		t.Fatalf("expected gate stderr, got stdout=%q stderr=%q", res.Stdout, res.Stderr)
	}
}
