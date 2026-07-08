package cmd_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// bareRemote initialises a bare git repo for a harness sharing round-trip.
func bareRemote(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", "--quiet", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}
	return dir
}

func writePolicy(t *testing.T, root, rel, data string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestHarnessPushPullEnableE2E drives the full command surface: push a policy
// bundle, pull it into a fresh project, confirm the executable deploy template is
// quarantined (not installed), list it, and enable it explicitly.
func TestHarnessPushPullEnableE2E(t *testing.T) {
	remote := bareRemote(t)

	// Producer project with policy to share.
	src := th.New(t)
	writePolicy(t, src.Root, ".specd/guardrails.json", `{"rules":[{"id":"no-todo","pattern":"TODO","message":"no TODO"}]}`)
	writePolicy(t, src.Root, ".specd/deploy/prod.json", `{"env":"prod","steps":[{"name":"ship","command":"deploy.sh"}]}`)
	src.RunExpect(core.ExitOK, "harness", "push", remote, "--name", "team")

	// Consumer project pulls the bundle.
	dst := th.New(t)
	res := dst.RunExpect(core.ExitOK, "harness", "pull", remote)
	if !strings.Contains(res.Stdout, "quarantined") {
		t.Fatalf("pull output did not report quarantine:\n%s", res.Stdout)
	}
	// The executable deploy template must NOT be installed yet.
	if _, err := os.Stat(dst.Path(".specd/deploy/prod.json")); err == nil {
		t.Fatal("executable artifact installed without explicit enable")
	}
	dst.AssertFileExists(".specd/guardrails.json")

	// list --json surfaces the quarantine.
	list := dst.RunExpect(core.ExitOK, "harness", "list", "--json")
	var lj struct {
		Quarantined []string `json:"quarantined"`
	}
	if err := json.Unmarshal([]byte(list.Stdout), &lj); err != nil {
		t.Fatalf("list json: %v (%q)", err, list.Stdout)
	}
	if len(lj.Quarantined) != 1 || lj.Quarantined[0] != ".specd/deploy/prod.json" {
		t.Fatalf("quarantine list = %v", lj.Quarantined)
	}

	// Enable installs it.
	dst.RunExpect(core.ExitOK, "harness", "enable", ".specd/deploy/prod.json")
	dst.AssertFileExists(".specd/deploy/prod.json")
}

// TestHarnessListTextAndEnableErrors covers the text-mode list rendering and the
// enable failure branches (missing arg, path not under quarantine).
func TestHarnessListTextAndEnableErrors(t *testing.T) {
	remote := bareRemote(t)

	src := th.New(t)
	writePolicy(t, src.Root, ".specd/guardrails.json", `{"rules":[{"id":"x","pattern":"X","message":"m"}]}`)
	writePolicy(t, src.Root, ".specd/deploy/prod.json", `{"env":"prod","steps":[{"name":"ship","command":"deploy.sh"}]}`)
	src.RunExpect(core.ExitOK, "harness", "push", remote, "--name", "team")

	dst := th.New(t)
	dst.RunExpect(core.ExitOK, "harness", "pull", remote)

	// Text-mode listing renders the bundle + quarantine section.
	list := dst.RunExpect(core.ExitOK, "harness", "list")
	if !strings.Contains(list.Stdout, ".specd/deploy/prod.json") {
		t.Fatalf("text list missing quarantined path:\n%s", list.Stdout)
	}

	// enable with no argument is a usage error.
	dst.RunExpect(core.ExitUsage, "harness", "enable")
	// enable of a path that was never quarantined fails closed.
	dst.RunExpect(core.ExitNotFound, "harness", "enable", ".specd/not/quarantined.json")
}

// TestHarnessJSONAndRefused covers the --json push/pull output branches and the
// refused (locally modified, no --force) path.
func TestHarnessJSONAndRefused(t *testing.T) {
	remote := bareRemote(t)

	src := th.New(t)
	writePolicy(t, src.Root, ".specd/guardrails.json", `{"rules":[{"id":"x","pattern":"X","message":"m"}]}`)
	push := src.RunExpect(core.ExitOK, "harness", "push", remote, "--name", "team", "--json")
	if !strings.Contains(push.Stdout, `"action":"push"`) && !strings.Contains(push.Stdout, `"action": "push"`) {
		t.Fatalf("push --json missing action:\n%s", push.Stdout)
	}

	dst := th.New(t)
	pull := dst.RunExpect(core.ExitOK, "harness", "pull", remote, "--json")
	if !strings.Contains(pull.Stdout, `"installed"`) {
		t.Fatalf("pull --json missing installed:\n%s", pull.Stdout)
	}

	// Locally modify the installed policy, then a second pull must refuse it.
	writePolicy(t, dst.Root, ".specd/guardrails.json", `{"rules":[{"id":"local","pattern":"L","message":"changed"}]}`)
	dst.RunExpect(core.ExitGate, "harness", "pull", remote)
	// With --force the refusal is overridden.
	dst.RunExpect(core.ExitOK, "harness", "pull", remote, "--force")
}

// TestHarnessPushPullBadArgs covers the argument-validation and bad-remote error
// branches of push/pull (missing remote, unreachable remote).
func TestHarnessPushPullBadArgs(t *testing.T) {
	h := th.New(t)
	writePolicy(t, h.Root, ".specd/guardrails.json", `{"rules":[{"id":"x","pattern":"X","message":"m"}]}`)

	// Missing <git-url> positional → usage error.
	h.RunExpect(core.ExitUsage, "harness", "push")
	h.RunExpect(core.ExitUsage, "harness", "pull")

	// A path that is not a git repo → non-zero (clone/push fails), never a panic.
	notARepo := h.Path("not-a-repo")
	if code := h.Run("harness", "push", notARepo, "--name", "team").Code; code == core.ExitOK {
		t.Fatal("push to non-repo returned ExitOK")
	}
	if code := h.Run("harness", "pull", notARepo).Code; code == core.ExitOK {
		t.Fatal("pull from non-repo returned ExitOK")
	}
}

// TestHarnessListWithoutBundle reports a clean not-found (exit 3), never a panic.
func TestHarnessListWithoutBundle(t *testing.T) {
	h := th.New(t)
	h.RunExpect(core.ExitNotFound, "harness", "list")
}
