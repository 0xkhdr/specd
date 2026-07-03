package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	contextpkg "github.com/0xkhdr/specd/internal/context"
)

// loadLatestContextSnapshot reads the highest-turn context snapshot for a
// session, returning ok=false when none exist. Snapshots are named <turn>.json;
// the highest turn is the most recent context the worker was given (R2).
func loadLatestContextSnapshot(root, sessionID string) (contextpkg.ContextSnapshot, bool, error) {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return contextpkg.ContextSnapshot{}, false, err
	}
	dir, err := paths.ContextSnapshotDir(sessionID)
	if err != nil {
		return contextpkg.ContextSnapshot{}, false, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return contextpkg.ContextSnapshot{}, false, nil
	}
	if err != nil {
		return contextpkg.ContextSnapshot{}, false, fmt.Errorf("context snapshot: read dir: %w", err)
	}
	bestTurn := -1
	bestName := ""
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		turn, err := strconv.Atoi(strings.TrimSuffix(name, ".json"))
		if err != nil || turn < 0 {
			continue
		}
		if turn > bestTurn {
			bestTurn = turn
			bestName = name
		}
	}
	if bestName == "" {
		return contextpkg.ContextSnapshot{}, false, nil
	}
	raw, err := os.ReadFile(filepath.Join(dir, bestName))
	if err != nil {
		return contextpkg.ContextSnapshot{}, false, fmt.Errorf("context snapshot: read %s: %w", bestName, err)
	}
	var snapshot contextpkg.ContextSnapshot
	if err := decodeACPStrict(raw, &snapshot); err != nil {
		return contextpkg.ContextSnapshot{}, false, fmt.Errorf("context snapshot: decode %s: %w", bestName, err)
	}
	if err := contextpkg.ValidateContextSnapshot(snapshot); err != nil {
		return contextpkg.ContextSnapshot{}, false, err
	}
	return snapshot, true, nil
}

// latestContextSnapshotDiff diffs the latest snapshot against the working tree,
// returning the reference/reload verdict, or ok=false when no snapshot exists.
func latestContextSnapshotDiff(root, sessionID string) (*contextpkg.SnapshotDiff, bool, error) {
	snapshot, ok, err := loadLatestContextSnapshot(root, sessionID)
	if err != nil || !ok {
		return nil, false, err
	}
	diff, err := contextpkg.DiffContextSnapshot(snapshot, root)
	if err != nil {
		return nil, false, err
	}
	return &diff, true, nil
}
