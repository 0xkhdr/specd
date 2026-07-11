package context

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// ResolveError names the failing context source so a fail-closed build reports
// item identity and an actionable finding, never silently dropping required
// knowledge (R2.3).
type ResolveError struct {
	Source string
	Reason string
}

func (e ResolveError) Error() string {
	return fmt.Sprintf("context source %q: %s", e.Source, e.Reason)
}

// ResolveSource resolves a repo-relative context source beneath root (R2.2),
// refusing traversal, absolute escape, and disallowed symlink escape, and
// requiring the target to exist and be readable (R2.3). It returns the cleaned
// root-relative slash path actually used.
func ResolveSource(root, source string) (string, error) {
	abs, err := core.SafeJoin(root, source)
	if err != nil {
		return "", ResolveError{Source: source, Reason: err.Error()}
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", ResolveError{Source: source, Reason: "missing or unreadable"}
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", ResolveError{Source: source, Reason: "repository base unreadable"}
	}
	rel, err := filepath.Rel(rootReal, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ResolveError{Source: source, Reason: "escapes repository base via symlink"}
	}
	return filepath.ToSlash(rel), nil
}
