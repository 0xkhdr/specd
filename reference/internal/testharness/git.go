package testharness

import (
	"os/exec"
	"strings"
)

// InitGit initialises a real but hermetic git repository at the project root and
// records an initial commit, giving commands that shell out to git (notably
// verify's gitHead capture) a HEAD to read. It uses a fixed in-repo identity and
// never touches the user's global git config. If git is not installed the test
// is skipped rather than failed.
//
// This is the harness's substitute for a mocked git: a real repo confined to the
// temp root is both more faithful and simpler than reimplementing git plumbing.
func (h *Harness) InitGit() {
	h.T.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		h.T.Skip("git not installed; skipping git-dependent test")
	}
	h.git("init", "-q")
	h.git("config", "user.email", "specd@test.local")
	h.git("config", "user.name", "specd test")
	h.git("config", "commit.gpgsign", "false")
	h.git("commit", "--allow-empty", "-q", "-m", "initial")
}

// GitCommitAll stages every change in the project root and commits it, returning
// the resulting HEAD hash.
func (h *Harness) GitCommitAll(msg string) string {
	h.T.Helper()
	h.git("add", "-A")
	h.git("commit", "-q", "--allow-empty", "-m", msg)
	return h.GitHead()
}

// GitHead returns the current HEAD commit hash.
func (h *Harness) GitHead() string {
	h.T.Helper()
	return strings.TrimSpace(h.git("rev-parse", "HEAD"))
}

func (h *Harness) git(args ...string) string {
	h.T.Helper()
	cmd := exec.Command("git", append([]string{"-C", h.Root}, args...)...) //nolint:gosec // test harness; git is a fixed binary with test-supplied args
	out, err := cmd.CombinedOutput()
	if err != nil {
		h.T.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
