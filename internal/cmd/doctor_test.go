package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/integration"
)

func TestDoctorHealthFixAndJSON(t *testing.T) {
	t.Run("missing scaffold reports deterministic unhealthy JSON", func(t *testing.T) {
		initTestRoot(t)
		runtime := doctorRuntime{Registry: integration.MustRegistry(), Probe: passingProbe}
		stdout, stderr, code := captureOutput(t, func() int {
			return runDoctor(cli.ParseArgs([]string{"--json"}), runtime)
		})
		if code != core.ExitGate || stderr != "" {
			t.Fatalf("exit=%d stderr=%q", code, stderr)
		}
		var result doctorResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		if result.Status != "unhealthy" || result.Checks == nil || result.Hosts == nil || result.Remediations == nil {
			t.Fatalf("result=%+v", result)
		}
	})

	t.Run("fix repairs scaffold and owned project registration", func(t *testing.T) {
		initTestRoot(t)
		host := &onboardingAdapter{name: "codex", detected: true, scopes: []integration.Scope{integration.ScopeProject}}
		runtime := doctorRuntime{Registry: integration.MustRegistry(host), Probe: passingProbe}
		_, stderr, code := captureOutput(t, func() int {
			return runDoctor(cli.ParseArgs([]string{"--fix", "--agent", "codex"}), runtime)
		})
		if code != core.ExitOK || stderr != "" || host.installs != 1 {
			t.Fatalf("exit=%d stderr=%q installs=%d", code, stderr, host.installs)
		}
	})

	t.Run("reports orchestration capability separately from host lifecycle", func(t *testing.T) {
		initTestRoot(t)
		host := &onboardingAdapter{name: "vscode", detected: true, scopes: []integration.Scope{integration.ScopeProject}, registered: true, owned: true}
		runtime := doctorRuntime{Registry: integration.MustRegistry(host), Probe: passingProbe}
		stdout, stderr, code := captureOutput(t, func() int {
			return runDoctor(cli.ParseArgs([]string{"--json", "--agent", "vscode"}), runtime)
		})
		if code != core.ExitGate || stderr != "" {
			t.Fatalf("exit=%d stderr=%q", code, stderr)
		}
		var result doctorResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		if result.Orchestration.ServerCapability != "available" {
			t.Fatalf("serverCapability=%q, want available", result.Orchestration.ServerCapability)
		}
		if strings.Join(result.Orchestration.Tools, ",") != "specd_brain,specd_pinky" {
			t.Fatalf("orchestration tools=%v", result.Orchestration.Tools)
		}
		if !strings.Contains(result.Orchestration.HostLifecycle, "does not spawn") {
			t.Fatalf("host lifecycle boundary missing spawn disclaimer: %q", result.Orchestration.HostLifecycle)
		}
		var mcpDetail string
		for _, check := range result.Checks {
			if check.Name == "mcp" {
				mcpDetail = check.Detail
			}
		}
		if !strings.Contains(mcpDetail, "specd_brain") || !strings.Contains(mcpDetail, "specd_pinky") {
			t.Fatalf("mcp detail missing orchestration tools: %q", mcpDetail)
		}
		if len(result.Hosts) != 1 || !result.Hosts[0].ReloadRequired || !result.Hosts[0].TrustRequired {
			t.Fatalf("host lifecycle flags=%+v", result.Hosts)
		}
		if !strings.Contains(result.Hosts[0].LifecycleSupport, "does not spawn Pinky agents") {
			t.Fatalf("host lifecycle support missing boundary: %q", result.Hosts[0].LifecycleSupport)
		}
	})
}
