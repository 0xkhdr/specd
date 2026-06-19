package cmd

import (
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func TestBrainRegistryAndUsage(t *testing.T) {
	if !testRegistryHas("brain") {
		t.Fatal("brain command not registered")
	}
	if got := RunBrain(cli.ParseArgs(nil)); got != core.ExitUsage {
		t.Fatalf("RunBrain(no args) = %d, want %d", got, core.ExitUsage)
	}
}

func TestBrainPolicyRequiresExplicitLimits(t *testing.T) {
	_, ok := brainPolicy(cli.ParseArgs([]string{
		"--approval-policy", "manual",
		"--max-workers", "4",
		"--max-retries", "2",
		"--timeout-seconds", "7200",
		"--cost-limit", "0",
	}))
	if !ok {
		t.Fatal("brainPolicy rejected valid explicit policy")
	}
	_, ok = brainPolicy(cli.ParseArgs([]string{"--approval-policy", "manual"}))
	if ok {
		t.Fatal("brainPolicy accepted missing explicit limits")
	}
}

func TestPinkyRegistryAndUsage(t *testing.T) {
	if !testRegistryHas("pinky") {
		t.Fatal("pinky command not registered")
	}
	if got := RunPinky(cli.ParseArgs(nil)); got != core.ExitUsage {
		t.Fatalf("RunPinky(no args) = %d, want %d", got, core.ExitUsage)
	}
}

func testRegistryHas(name string) bool {
	for _, command := range Registry {
		if command.Name == name {
			return true
		}
	}
	return false
}

func TestPinkyReportArgs(t *testing.T) {
	report, ok := pinkyTerminalArgs(cli.ParseArgs([]string{
		"--session", "s",
		"--worker", "w",
		"--spec", "spec",
		"--task", "T1",
		"--attempt", "1",
		"--verification-ref", "verify-pass",
		"--summary", "done",
		"--changed-files", "a.go,b.go",
		"--duration-ms", "12",
		"--host-tokens", "34",
	}))
	if !ok {
		t.Fatal("pinkyTerminalArgs rejected valid report")
	}
	if report.SessionID != "s" || report.WorkerID != "w" || report.Spec != "spec" || report.TaskID != "T1" {
		t.Fatalf("unexpected report identity: %+v", report)
	}
	if len(report.ChangedFiles) != 2 || report.DurationMs != 12 || report.HostTokens != 34 {
		t.Fatalf("unexpected report metrics: %+v", report)
	}
}
