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

	release, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(release), "./scripts/production-smoke.sh") {
		t.Fatal("release workflow does not run ./scripts/production-smoke.sh")
	}
}

func TestCITierContract(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	ci := read(".github/workflows/ci.yml")
	heavy := read(".github/workflows/heavy.yml")
	release := read(".github/workflows/release.yml")
	local := read("scripts/ci-local.sh")

	for _, want := range []string{"pull_request:", "branches-ignore: [main]", "go test ./... -race -count=1"} {
		if !strings.Contains(ci, want) {
			t.Errorf("fast workflow missing %q", want)
		}
	}
	for _, forbidden := range []string{"regress-domains.sh", "stress.sh", "perf-gate.sh", "production-smoke.sh", "GOOS="} {
		if strings.Contains(ci, forbidden) {
			t.Errorf("fast workflow contains merge/release lane %q", forbidden)
		}
	}
	for _, want := range []string{"branches: [main]", "schedule:", "go test ./... -count=2", "regress-domains.sh", "stress.sh", "perf-gate.sh", "install-scripts-test.sh", "coverage-check.sh"} {
		if !strings.Contains(heavy, want) {
			t.Errorf("heavy workflow missing %q", want)
		}
	}
	for _, want := range []string{"production-smoke.sh", "goreleaser/goreleaser-action"} {
		if !strings.Contains(release, want) {
			t.Errorf("release workflow missing %q", want)
		}
	}
	for _, forbidden := range []string{"SKIP:", "install-scripts-test.sh", "go test ./... -count=2", "perf-gate.sh", "regress-domains.sh", "stress.sh"} {
		if strings.Contains(local, forbidden) {
			t.Errorf("local fast tier contains %q", forbidden)
		}
	}
	for _, want := range []string{"golangci-lint run", "govulncheck ./...", "shellcheck -S error", "go test ./... -race -count=1", "coverage-check.sh", "go build ./..."} {
		if !strings.Contains(local, want) {
			t.Errorf("local fast tier missing %q", want)
		}
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
