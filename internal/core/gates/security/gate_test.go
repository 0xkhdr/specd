package security

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

func gitRepoWithLeak(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	if err := os.WriteFile(filepath.Join(root, "leak.txt"), []byte("key = AKIAABCDEFGHIJKLMNOP\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	return root
}

func TestSeverity(t *testing.T) {
	root := gitRepoWithLeak(t)

	t.Run("error_severity_fails_gate", func(t *testing.T) {
		findings := GateFindings(Analyze(root, core.SecurityConfig{Secrets: "error"}))
		if len(findings) != 1 || findings[0].Severity != gates.Error {
			t.Fatalf("want one error finding, got %+v", findings)
		}
	})

	t.Run("warn_severity_prints_but_passes", func(t *testing.T) {
		findings := GateFindings(Analyze(root, core.SecurityConfig{Secrets: "warn"}))
		if len(findings) != 1 || findings[0].Severity != gates.Warn {
			t.Fatalf("want one warn finding, got %+v", findings)
		}
		if gates.HasErrors(findings) {
			t.Fatal("warn findings must not fail the gate")
		}
	})

	t.Run("off_severity_skips_scanner", func(t *testing.T) {
		if f := GateFindings(Analyze(root, core.SecurityConfig{Secrets: "off"})); len(f) != 0 {
			t.Fatalf("off scanner should produce nothing: %+v", f)
		}
	})

	t.Run("corrupt_allowlist_fails_gate_closed", func(t *testing.T) {
		// End-to-end trust-boundary proof (T-04-02): a corrupt allowlist must not
		// silently pass. loadAllowlist fails closed; Analyze must surface that as
		// an error-severity gate finding so `check --security` exits non-zero,
		// never suppressing the real leak that lives in the same tree.
		root := gitRepoWithLeak(t)
		writeAllow(t, root, `not json`)
		findings := GateFindings(Analyze(root, core.SecurityConfig{Secrets: "error"}))
		if !gates.HasErrors(findings) {
			t.Fatalf("corrupt allowlist must fail the gate closed, got %+v", findings)
		}
		var sawLoadError bool
		for _, f := range findings {
			if f.Severity == gates.Error && strings.Contains(f.Message, "allowlist") {
				sawLoadError = true
			}
		}
		if !sawLoadError {
			t.Fatalf("expected an allowlist load error finding, got %+v", findings)
		}
	})

	t.Run("allowlisted_finding_recorded_but_not_gated", func(t *testing.T) {
		fp := fingerprint("aws-access-key", "leak.txt", "AKIAABCDEFGHIJKLMNOP")
		writeAllow(t, root, `[{"fingerprint":"`+fp+`","reason":"synthetic test key"}]`)
		result := Analyze(root, core.SecurityConfig{Secrets: "error"})
		if len(GateFindings(result)) != 0 {
			t.Fatal("allowlisted finding must not fail the gate")
		}
		if len(result.Findings) != 1 || !result.Findings[0].Allowlisted {
			t.Fatalf("allowlisted finding must still be recorded: %+v", result.Findings)
		}
	})
}

func TestPolicyFindingsAreConfigDriven(t *testing.T) {
	root := t.TempDir()
	if err := exec.Command("git", "-C", root, "init").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := Analyze(root, core.SecurityConfig{
		CleanWorktree: "warn",
		Sandbox:       "error",
	})
	if !hasFinding(result.Findings, "policy", "clean-worktree") {
		t.Fatalf("missing clean-worktree policy finding: %#v", result.Findings)
	}
	if !hasFinding(result.Findings, "policy", "sandbox") {
		t.Fatalf("missing sandbox policy finding: %#v", result.Findings)
	}

	t.Setenv("SPECD_SANDBOX_ACTIVE", "1")
	result = Analyze(root, core.SecurityConfig{
		CleanWorktree: "off",
		Sandbox:       "error",
	})
	if hasFinding(result.Findings, "policy", "sandbox") {
		t.Fatalf("sandbox marker should satisfy policy: %#v", result.Findings)
	}
}

func hasFinding(findings []Finding, scanner, rule string) bool {
	for _, finding := range findings {
		if finding.Scanner == scanner && finding.Rule == rule {
			return true
		}
	}
	return false
}
