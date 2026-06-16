package core

import "fmt"

// StateBackend is the storage contract for spec state. It abstracts the three
// guarantees every backend MUST honor, documented against the file backend in
// lock.go/state.go: serialize writers (WithLock), reject stale-revision writes
// (Save's CAS), and commit atomically. Extracting the interface lets alternative
// backends (git-native, Redis/Postgres behind build tags) slot in without
// weakening the integrity spine — they are held to the same conformance suite.
//
// The default backend is the on-disk file backend; behavior is identical to
// calling LoadState/SaveState/WithSpecLock directly.
type StateBackend interface {
	// Name identifies the backend for diagnostics and evidence ("file").
	Name() string
	// Load reads the current state, or (nil, nil) when none exists.
	Load(root, slug string) (*State, error)
	// Save commits state under a revision compare-and-swap. It MUST be called
	// inside WithLock for the same (root, slug).
	Save(root, slug string, state *State) error
	// WithLock runs fn while holding the spec's advisory lock, providing
	// cross-process and in-process exclusion plus owning-goroutine reentrancy.
	WithLock(root, slug string, fn func() error) error
}

// fileBackend is the default StateBackend: a thin adapter over the package's
// existing file-based lock + CAS functions, so selecting it changes nothing
// about today's on-disk behavior.
type fileBackend struct{}

func (fileBackend) Name() string { return "file" }

func (fileBackend) Load(root, slug string) (*State, error) { return LoadState(root, slug) }

func (fileBackend) Save(root, slug string, state *State) error { return SaveState(root, slug, state) }

func (fileBackend) WithLock(root, slug string, fn func() error) error {
	_, err := WithSpecLock[struct{}](root, slug, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

// DefaultBackend returns the on-disk file backend used by every command.
func DefaultBackend() StateBackend { return fileBackend{} }

// optionalBackends holds constructors for backends compiled in behind build
// tags (e.g. redis, postgres). The default build registers none, so the binary
// links no database driver — the registry is empty unless a `specd_*` tag is set.
var optionalBackends = map[string]func() StateBackend{}

func registerOptionalBackend(name string, ctor func() StateBackend) {
	optionalBackends[name] = ctor
}

// SelectBackend resolves a backend name to a StateBackend. "file"/"" is the
// default; "git" is always available (CLI-only, no driver); any other name must
// have been compiled in via its build tag, otherwise this fails closed.
func SelectBackend(name string) (StateBackend, error) {
	switch name {
	case "", "file":
		return DefaultBackend(), nil
	case "git":
		return GitBackend(), nil
	}
	if ctor, ok := optionalBackends[name]; ok {
		return ctor(), nil
	}
	return nil, GateError(fmt.Sprintf("unknown or not-compiled-in state backend %q (build with -tags specd_%s to enable)", name, name))
}
