package core

import (
	"strings"
	"testing"
)

func TestRuntimePathsCheckpoint(t *testing.T) {
	paths, err := NewACPRuntimePaths(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sessionID := strings.Repeat("a", 32)

	dir, err := paths.CheckpointDir(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dir, "/sessions/"+sessionID+"/checkpoints") {
		t.Fatalf("unexpected checkpoint dir: %s", dir)
	}

	path, err := paths.CheckpointPath(sessionID, "T2", 3)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/sessions/"+sessionID+"/checkpoints/T2-3.json") {
		t.Fatalf("unexpected checkpoint path: %s", path)
	}

	// Reject traversal / malformed task IDs and bad attempts.
	if _, err := paths.CheckpointPath(sessionID, "../etc", 1); err == nil {
		t.Fatal("expected rejection of traversal task ID")
	}
	if _, err := paths.CheckpointPath(sessionID, "t2", 1); err == nil {
		t.Fatal("expected rejection of lowercase task ID")
	}
	if _, err := paths.CheckpointPath(sessionID, "T2", 0); err == nil {
		t.Fatal("expected rejection of attempt < 1")
	}
	if _, err := paths.CheckpointPath("short", "T2", 1); err == nil {
		t.Fatal("expected rejection of bad session ID")
	}
}
