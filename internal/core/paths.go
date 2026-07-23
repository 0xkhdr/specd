package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const specdDirName = ".specd"

type NotFoundError struct {
	Start string
}

func EvalStorePath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), "evals", "records.jsonl")
}

func EvalTracePath(root, slug, runID string) string {
	return filepath.Join(SpecDir(root, slug), "evals", "traces", runID+".jsonl")
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("specd root not found from %s", e.Start)
}

func (e NotFoundError) ExitCode() int {
	return 3
}

func SpecdDir(root string) string {
	return filepath.Join(root, specdDirName)
}

// SpecDir is the single sink for paths under .specd/specs/<slug>. Callers
// validate user input for an actionable error; the sink still fails closed if
// one is missed, so an invalid slug can never reach filepath.Join.
func SpecDir(root, slug string) string {
	if err := ValidateSlug(slug); err != nil {
		panic(fmt.Sprintf("invalid spec slug %q: %v", slug, err))
	}
	return filepath.Join(SpecdDir(root), "specs", slug)
}

// SafeJoin resolves a slash-separated repo-relative path against root, refusing
// empty input, absolute paths, and traversal ("..") that escapes the base. It
// returns the cleaned absolute path. It performs no symlink resolution and does
// not require existence — callers that need those (see context.ResolveSource)
// layer them on top.
func SafeJoin(root, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("absolute path not allowed: %q", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes repository base: %q", rel)
	}
	return filepath.Join(root, clean), nil
}

// SpecMemoryPath is the per-spec steering-memory store (RM.1).
func SpecMemoryPath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), "memory.md")
}

// SteeringMemoryPath is the shared steering store promotions land in (RM.3).
func SteeringMemoryPath(root string) string {
	return filepath.Join(SpecdDir(root), "steering", "memory.md")
}

// ListSpecs enumerates spec slugs under .specd/specs/, sorted. Missing dir
// yields an empty list, not an error.
func ListSpecs(root string) []string {
	entries, err := os.ReadDir(filepath.Join(SpecdDir(root), "specs"))
	if err != nil {
		return nil
	}
	var slugs []string
	for _, entry := range entries {
		if entry.IsDir() {
			slugs = append(slugs, entry.Name())
		}
	}
	sort.Strings(slugs)
	return slugs
}

func FindRoot(start string) (string, error) {
	if start == "" {
		start = "."
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(dir); resolveErr == nil {
		dir = resolved
	}
	info, err := os.Stat(dir)
	if err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if st, err := os.Stat(SpecdDir(dir)); err == nil && st.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", NotFoundError{Start: start}
		}
		dir = parent
	}
}
