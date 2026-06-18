package cmd

import (
	"encoding/json"
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
}
