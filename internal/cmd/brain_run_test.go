package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrainFailClosedRequiresOrchestrationConfig(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", "")

	err := runBrain(root, []string{"start", "demo"}, map[string]string{})
	if err == nil {
		t.Fatal("expected missing config precondition")
	}
	if !strings.Contains(err.Error(), "orchestration.enabled") {
		t.Fatalf("expected orchestration.enabled error, got %v", err)
	}
	assertNoBrainSession(t, root)
}

func TestBrainFailClosedRequiresOrchestratedSpecMode(t *testing.T) {
	root := newBrainTestRoot(t, "default", "orchestration:\n  enabled: true\n")

	err := runBrain(root, []string{"start", "demo"}, map[string]string{})
	if err == nil {
		t.Fatal("expected mode precondition")
	}
	if !strings.Contains(err.Error(), "spec mode must be orchestrated") {
		t.Fatalf("expected spec mode error, got %v", err)
	}
	assertNoBrainSession(t, root)
}

func TestBrainStartCreatesSessionWhenPreconditionsPass(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", "orchestration:\n  enabled: true\n")

	if err := runBrain(root, []string{"start", "demo"}, map[string]string{}); err != nil {
		t.Fatalf("brain start: %v", err)
	}
	if _, err := os.Stat(brainSessionPath(root)); err != nil {
		t.Fatalf("expected session: %v", err)
	}
}

func newBrainTestRoot(t *testing.T, mode, projectConfig string) string {
	t.Helper()
	root := t.TempDir()
	specDir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"schema_version":1,"slug":"demo","mode":"` + mode + `","status":"design","phase":"analyze","revision":1,"records":{}}`
	if err := os.WriteFile(filepath.Join(specDir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	if projectConfig != "" {
		if err := os.WriteFile(filepath.Join(root, "project.yml"), []byte(projectConfig), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func assertNoBrainSession(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(brainSessionPath(root)); !os.IsNotExist(err) {
		t.Fatalf("expected no session, stat err=%v", err)
	}
}

func brainSessionPath(root string) string {
	return filepath.Join(root, ".specd", "specs", "demo", "session.json")
}
