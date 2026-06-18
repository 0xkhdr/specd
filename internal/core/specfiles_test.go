package core

import (
	"reflect"
	"testing"
)

func TestConfigOrchestrationDefault(t *testing.T) {
	t.Run("missing config", func(t *testing.T) {
		got := LoadConfig(t.TempDir())
		if !reflect.DeepEqual(got, DefaultConfig) {
			t.Fatalf("LoadConfig() = %#v, want %#v", got, DefaultConfig)
		}
	})

	t.Run("malformed config", func(t *testing.T) {
		root := t.TempDir()
		if err := AtomicWrite(ConfigPath(root), `{"orchestration":`); err != nil {
			t.Fatal(err)
		}

		got := LoadConfig(root)
		if !reflect.DeepEqual(got, DefaultConfig) {
			t.Fatalf("LoadConfig() = %#v, want %#v", got, DefaultConfig)
		}
	})
}

func TestConfigOrchestrationLegacy(t *testing.T) {
	root := t.TempDir()
	legacy := `{
		"version": 7,
		"defaultVerify": "go test ./...",
		"report": {"format": "html", "autoRefreshSeconds": 15},
		"roles": {"subagentMode": "delegate"},
		"promotionThreshold": 5,
		"gates": {"traceability": "error", "acceptance": "warn", "scope": "error"},
		"verify": {"sandbox": "bwrap"}
	}`
	if err := AtomicWrite(ConfigPath(root), legacy); err != nil {
		t.Fatal(err)
	}

	got := LoadConfig(root)
	if got.Version != 7 ||
		got.DefaultVerify != "go test ./..." ||
		got.Report != (ReportCfg{Format: "html", AutoRefreshSeconds: 15}) ||
		got.Roles != (RolesCfg{SubagentMode: "delegate"}) ||
		got.PromotionThreshold != 5 ||
		got.Gates.Traceability != "error" ||
		got.Gates.Acceptance != "warn" ||
		got.Gates.Scope != "error" ||
		got.Verify != (VerifyCfg{Sandbox: "bwrap"}) {
		t.Fatalf("legacy fields changed during load: %#v", got)
	}
	if !reflect.DeepEqual(got.Orchestration, DefaultConfig.Orchestration) {
		t.Fatalf("orchestration = %#v, want defaults %#v", got.Orchestration, DefaultConfig.Orchestration)
	}
}

func TestConfigOrchestrationPartial(t *testing.T) {
	root := t.TempDir()
	partial := `{
		"defaultVerify": "make test",
		"orchestration": {
			"enabled": true,
			"approvalPolicy": "planning",
			"maxWorkers": 8,
			"hostReportedCostLimitUSD": 0,
			"transport": {
				"pollIntervalMillis": 250,
				"heartbeatSeconds": 10
			},
			"program": {
				"maxConcurrentSpecs": 3
			}
		}
	}`
	if err := AtomicWrite(ConfigPath(root), partial); err != nil {
		t.Fatal(err)
	}

	want := DefaultConfig
	want.DefaultVerify = "make test"
	want.Orchestration.Enabled = true
	want.Orchestration.ApprovalPolicy = "planning"
	want.Orchestration.MaxWorkers = 8
	want.Orchestration.HostReportedCostLimitUSD = 0
	want.Orchestration.Transport.PollIntervalMillis = 250
	want.Orchestration.Transport.HeartbeatSeconds = 10
	want.Orchestration.Program.MaxConcurrentSpecs = 3

	got := LoadConfig(root)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadConfig() = %#v, want %#v", got, want)
	}
}
