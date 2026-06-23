package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// gitBackend is a git-native StateBackend. It reuses the file backend's proven
// lock + revision-CAS spine verbatim — so the integrity guarantees are
// unweakened — and additionally commits each saved state into the project's git
// repository. The commit history is the distributed-sync surface: a clone/fetch
// carries the full audit trail, and the state's monotonic `revision` field plus
// the per-write commit give an externally-verifiable CAS chain.
//
// It uses git through the `git` CLI only — no Go git library dependency — so the
// default binary links nothing new. git-init and commits run while holding the
// spec lock, so concurrent writers can never race the repository setup.
type gitBackend struct{}

// GitBackend returns the git-native state backend.
func GitBackend() StateBackend { return gitBackend{} }

func (gitBackend) Name() string { return "git" }

func (gitBackend) Load(root, slug string) (*State, error) { return LoadState(root, slug) }

// Save commits state through the file backend's CAS (which rejects stale-base
// writes and writes atomically), then records the result as a git commit. The
// CAS runs first: a conflicting write is refused before anything is committed.
func (gitBackend) Save(root, slug string, state *State) error {
	if err := SaveState(root, slug, state); err != nil {
		return err
	}
	return gitCommitState(root, slug, state.Revision)
}

// WithLock holds the spec's advisory lock for the whole critical section and
// lazily initializes the git repository inside it, so repository setup is
// serialized with every writer and never races under concurrency.
func (gitBackend) WithLock(root, slug string, fn func() error) error {
	return fileBackend{}.WithLock(root, slug, func() error {
		if err := ensureGitRepo(root); err != nil {
			return err
		}
		return fn()
	})
}

// ensureGitRepo initializes a git repository at root if one is not already
// present. It is idempotent and, because callers invoke it under the spec lock,
// concurrency-safe.
func ensureGitRepo(root string) error {
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		return nil
	}
	if out, err := runGit(root, "init", "--quiet"); err != nil {
		return GateError(fmt.Sprintf("git backend: init failed: %s", out))
	}
	return nil
}

// gitCommitState stages and commits a spec's state.json. Identity is supplied
// per-invocation (-c user.*) so the commit succeeds in environments without a
// configured git user, and --allow-empty keeps an idempotent re-save from
// failing on an unchanged tree.
func gitCommitState(root, slug string, rev int) error {
	rel := ".specd/specs/" + slug + "/state.json"
	if out, err := runGit(root, "add", "--", rel); err != nil {
		return GateError(fmt.Sprintf("git backend: stage %s failed: %s", rel, out))
	}
	msg := fmt.Sprintf("specd(%s): state revision %d", slug, rev)
	if out, err := runGit(root,
		"-c", "user.email=specd@localhost",
		"-c", "user.name=specd",
		"commit", "--allow-empty", "--quiet", "-m", msg, "--", rel,
	); err != nil {
		return GateError(fmt.Sprintf("git backend: commit failed: %s", out))
	}
	return nil
}

func runGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...) //nolint:gosec // git is a fixed binary; args are specd-supplied, not a shell string (see SECURITY.md)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
