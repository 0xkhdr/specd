package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadReviewReport(t *testing.T) {
	root := t.TempDir()

	body, modTime := ReadReviewReport(root, "auth")
	if body != nil {
		t.Fatalf("expected absent report body, got %q", *body)
	}
	if !modTime.IsZero() {
		t.Fatalf("expected zero mod time for absent report, got %s", modTime)
	}

	path := ArtifactPath(root, "auth", reviewReportName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	want := "## Verdict\napprove\n"
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	body, modTime = ReadReviewReport(root, "auth")
	if body == nil {
		t.Fatal("expected report body")
	}
	if *body != want {
		t.Fatalf("body = %q, want %q", *body, want)
	}
	if modTime.IsZero() {
		t.Fatal("expected report mod time")
	}
}

func TestLatestTaskCompletion(t *testing.T) {
	oldTime := "2026-07-02T12:00:00Z"
	newTime := "2026-07-02T12:30:00Z"
	invalidTime := "not-a-time"
	state := &State{Tasks: map[string]TaskState{
		"old":     {FinishedAt: &oldTime},
		"new":     {FinishedAt: &newTime},
		"invalid": {FinishedAt: &invalidTime},
		"open":    {},
	}}

	got := LatestTaskCompletion(state)
	want, err := time.Parse(time.RFC3339Nano, newTime)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(want) {
		t.Fatalf("latest = %s, want %s", got, want)
	}
}
