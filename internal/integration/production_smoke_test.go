package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionSmokeLane(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	scriptPath := filepath.Join(root, "scripts", "production-smoke.sh")
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("production smoke lane missing: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("production smoke lane is not executable")
	}
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"init", "new", "approve", "context", "verify", "complete-task", "review", "submit"} {
		if !strings.Contains(string(script), command) {
			t.Errorf("production smoke does not exercise %q", command)
		}
	}

	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflow), "./scripts/production-smoke.sh") {
		t.Fatal("CI does not run production smoke lane")
	}
}
