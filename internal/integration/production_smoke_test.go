package integration

import (
	"os"
	"os/exec"
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

	// What matters is that some workflow runs the lane, not which one: CI is
	// tiered, and pinning this to a single file made the assertion stale the
	// moment the lane moved from ci.yml to heavy.yml while still running.
	workflowDir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		t.Fatal(err)
	}
	var ranBy []string
	for _, entry := range entries {
		workflow, err := os.ReadFile(filepath.Join(workflowDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(workflow), "./scripts/production-smoke.sh") {
			ranBy = append(ranBy, entry.Name())
		}
	}
	if len(ranBy) == 0 {
		t.Fatalf("no workflow in %s runs ./scripts/production-smoke.sh", workflowDir)
	}
}

func TestWorkflowCoherenceProduction(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs fresh production fixtures")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, "scripts", "production-smoke.sh")
	raw, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)
	for _, contract := range []string{
		"profile: production",
		"--criterion 1.1 --status pass",
		"independent-auditor",
		"<approve | reject | needs-changes>/approve/",
		"status smoke",
		"complete-task smoke T1",
	} {
		if !strings.Contains(script, contract) {
			t.Fatalf("production fixture omitted %q", contract)
		}
	}
	for _, args := range [][]string{{"--negative"}, nil} {
		command := exec.Command(scriptPath, args...)
		command.Dir = root
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("production smoke %v: %v\n%s", args, err, out)
		}
	}
}
