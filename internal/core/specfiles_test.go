package core

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestConfigOrchestrationDefault(t *testing.T) {
	t.Run("missing_config", func(t *testing.T) {
		got := LoadConfig(t.TempDir())
		if !reflect.DeepEqual(got, DefaultConfig) {
			t.Fatalf("LoadConfig() = %#v, want %#v", got, DefaultConfig)
		}
	})

	t.Run("malformed_config", func(t *testing.T) {
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
	legacy := `version: 7
defaultVerify: "go test ./..."
report:
  format: html
  autoRefreshSeconds: 15
roles:
  subagentMode: delegate
promotionThreshold: 5
gates:
  traceability: error
  acceptance: warn
  scope: error
verify:
  sandbox: bwrap
`
	writeConfigFile(t, projectConfig(root, "config.yml"), legacy)

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
	partial := `defaultVerify: make test
orchestration:
  enabled: true
  approvalPolicy: planning
  maxWorkers: 8
  hostReportedCostLimitUSD: 0
  transport:
    pollIntervalMillis: 250
    heartbeatSeconds: 10
  program:
    maxConcurrentSpecs: 3
`
	writeConfigFile(t, projectConfig(root, "config.yml"), partial)

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

func TestOrchestrationConfigBoundaryValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  OrchestrationCfg
		want OrchestrationCfg
	}{
		{
			name: "clamps lower bounds",
			cfg: OrchestrationCfg{
				ApprovalPolicy: "manual",
				WorkerMode:     "host",
				MaxWorkers:     -1,
				MaxRetries:     -1,
				Transport:      TransportCfg{Kind: "file"},
			},
			want: OrchestrationCfg{
				ApprovalPolicy:        "manual",
				WorkerMode:            "host",
				MaxWorkers:            minMaxWorkers,
				MaxRetries:            minMaxRetries,
				SessionTimeoutMinutes: minSessionTimeoutMinutes,
				Transport: TransportCfg{
					Kind:               "file",
					PollIntervalMillis: minPollIntervalMillis,
					MessageTTLSeconds:  minMessageTTLSeconds,
					LeaseSeconds:       minLeaseSeconds,
					HeartbeatSeconds:   minHeartbeatSeconds,
				},
				Program: ProgramCfg{MaxConcurrentSpecs: minMaxConcurrentSpecs},
			},
		},
		{
			name: "clamps upper bounds",
			cfg: OrchestrationCfg{
				ApprovalPolicy:        "session",
				WorkerMode:            "host",
				MaxWorkers:            maxMaxWorkers + 1,
				MaxRetries:            maxMaxRetries + 1,
				SessionTimeoutMinutes: maxSessionTimeoutMinutes + 1,
				Transport: TransportCfg{
					Kind:               "file",
					PollIntervalMillis: maxPollIntervalMillis + 1,
					MessageTTLSeconds:  maxMessageTTLSeconds + 1,
					LeaseSeconds:       maxLeaseSeconds + 1,
					HeartbeatSeconds:   maxHeartbeatSeconds + 1,
				},
				Program: ProgramCfg{MaxConcurrentSpecs: maxMaxConcurrentSpecs + 1},
			},
			want: OrchestrationCfg{
				ApprovalPolicy:        "session",
				WorkerMode:            "host",
				MaxWorkers:            maxMaxWorkers,
				MaxRetries:            maxMaxRetries,
				SessionTimeoutMinutes: maxSessionTimeoutMinutes,
				Transport: TransportCfg{
					Kind:               "file",
					PollIntervalMillis: maxPollIntervalMillis,
					MessageTTLSeconds:  maxMessageTTLSeconds,
					LeaseSeconds:       maxLeaseSeconds,
					HeartbeatSeconds:   maxHeartbeatSeconds,
				},
				Program: ProgramCfg{MaxConcurrentSpecs: maxMaxConcurrentSpecs},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateOrchestrationConfig(&tt.cfg); err != nil {
				t.Fatalf("ValidateOrchestrationConfig() error = %v", err)
			}
			if !reflect.DeepEqual(tt.cfg, tt.want) {
				t.Fatalf("config = %#v, want %#v", tt.cfg, tt.want)
			}
		})
	}
}

func TestOrchestrationConfigInvalidEnumValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*OrchestrationCfg)
	}{
		{"approval policy", func(cfg *OrchestrationCfg) { cfg.ApprovalPolicy = "automatic" }},
		{"worker mode", func(cfg *OrchestrationCfg) { cfg.WorkerMode = "embedded" }},
		{"transport kind", func(cfg *OrchestrationCfg) { cfg.Transport.Kind = "http" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig.Orchestration
			tt.mutate(&cfg)
			if err := ValidateOrchestrationConfig(&cfg); err == nil {
				t.Fatal("ValidateOrchestrationConfig() error = nil, want rejection")
			}
		})
	}
}

func TestOrchestrationConfigUnsafeValueValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*OrchestrationCfg)
	}{
		{"negative cost", func(cfg *OrchestrationCfg) { cfg.HostReportedCostLimitUSD = -1 }},
		{"nan cost", func(cfg *OrchestrationCfg) { cfg.HostReportedCostLimitUSD = math.NaN() }},
		{"infinite cost", func(cfg *OrchestrationCfg) { cfg.HostReportedCostLimitUSD = math.Inf(1) }},
		{"heartbeat reaches lease", func(cfg *OrchestrationCfg) {
			cfg.Transport.HeartbeatSeconds = cfg.Transport.LeaseSeconds
		}},
		{"lease exceeds ttl", func(cfg *OrchestrationCfg) {
			cfg.Transport.MessageTTLSeconds = minMessageTTLSeconds
			cfg.Transport.LeaseSeconds = minMessageTTLSeconds + 1
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig.Orchestration
			tt.mutate(&cfg)
			if err := ValidateOrchestrationConfig(&cfg); err == nil {
				t.Fatal("ValidateOrchestrationConfig() error = nil, want rejection")
			}
		})
	}
}

func TestOrchestrationConfigFailClosedValidation(t *testing.T) {
	tests := []struct {
		name          string
		orchestration string
	}{
		{"invalid enum", "  enabled: true\n  workerMode: embedded\n"},
		{"api key", "  enabled: true\n  apiKey: top-secret\n"},
		{"provider credentials", "  enabled: true\n  providerCredentials:\n    user: x\n"},
		{"shell command", "  enabled: true\n  transport:\n    command: \"curl example.test\"\n"},
		{"model selection", "  enabled: true\n  model: vendor-model\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			raw := "defaultVerify: make test\norchestration:\n" + tt.orchestration
			writeConfigFile(t, projectConfig(root, "config.yml"), raw)

			got := LoadConfig(root)
			if got.DefaultVerify != "make test" {
				t.Fatalf("unrelated config field changed: DefaultVerify = %q", got.DefaultVerify)
			}
			if !reflect.DeepEqual(got.Orchestration, DefaultConfig.Orchestration) {
				t.Fatalf("orchestration = %#v, want fail-closed defaults %#v", got.Orchestration, DefaultConfig.Orchestration)
			}
		})
	}
}

func TestConfigPolicyDeterministicRendering(t *testing.T) {
	first, err := MarshalEffectiveOrchestrationPolicy(DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarshalEffectiveOrchestrationPolicy(DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("policy rendering is not deterministic:\n%s\n%s", first, second)
	}

	var got OrchestrationCfg
	if err := json.Unmarshal(first, &got); err != nil {
		t.Fatalf("rendered policy is invalid JSON: %v", err)
	}
	if !reflect.DeepEqual(got, DefaultConfig.Orchestration) {
		t.Fatalf("rendered policy = %#v, want %#v", got, DefaultConfig.Orchestration)
	}
	for _, forbidden := range []string{"secret", "token", "password", "credential", "apiKey", "environment"} {
		if strings.Contains(strings.ToLower(string(first)), strings.ToLower(forbidden)) {
			t.Fatalf("rendered policy contains forbidden field %q: %s", forbidden, first)
		}
	}
}

func TestInitConfigPolicyDeterministic(t *testing.T) {
	raw, err := ReadTemplate("config.json")
	if err != nil {
		t.Fatal(err)
	}
	var shipped Config
	if err := json.Unmarshal([]byte(raw), &shipped); err != nil {
		t.Fatalf("embedded config is invalid JSON: %v", err)
	}
	if !reflect.DeepEqual(shipped, DefaultConfig) {
		t.Fatalf("embedded config = %#v, want DefaultConfig %#v", shipped, DefaultConfig)
	}
}

// TestConfigMCPAbsentBlock verifies an absent mcp block leaves MCPConfig zero
// and Configured() false — the backward-compatible passthrough contract (R1).
func TestConfigMCPAbsentBlock(t *testing.T) {
	got := LoadConfig(t.TempDir())
	if got.MCP.Configured() {
		t.Errorf("absent mcp block reported Configured()=true: %#v", got.MCP)
	}
	if got.MCP.IncludeOrchestration != nil {
		t.Errorf("IncludeOrchestration = %v, want nil when key absent", *got.MCP.IncludeOrchestration)
	}
}

// TestConfigMCPPartialMerge overlays only the supplied fields and preserves the
// "unset vs explicit false" distinction for IncludeOrchestration (spec §8 risk).
func TestConfigMCPPartialMerge(t *testing.T) {
	root := t.TempDir()
	raw := `mcp:
  expose: essential
  essentialTools: [status, verify]
  includeMeta: true
`
	writeConfigFile(t, projectConfig(root, "config.yml"), raw)
	got := LoadConfig(root)
	if got.MCP.Expose != "essential" {
		t.Errorf("Expose = %q, want essential", got.MCP.Expose)
	}
	if len(got.MCP.EssentialTools) != 2 || got.MCP.EssentialTools[0] != "status" {
		t.Errorf("EssentialTools = %v, want [status verify]", got.MCP.EssentialTools)
	}
	if !got.MCP.IncludeMeta {
		t.Error("IncludeMeta = false, want true")
	}
	// includeOrchestration absent from JSON must stay nil, not become false.
	if got.MCP.IncludeOrchestration != nil {
		t.Errorf("IncludeOrchestration = %v, want nil (key omitted)", *got.MCP.IncludeOrchestration)
	}
}

// TestConfigMCPExplicitFalse confirms an explicit includeOrchestration:false is
// preserved as a non-nil pointer, distinct from the unset case.
func TestConfigMCPExplicitFalse(t *testing.T) {
	root := t.TempDir()
	writeConfigFile(t, projectConfig(root, "config.yml"), "mcp:\n  includeOrchestration: false\n")
	got := LoadConfig(root)
	if got.MCP.IncludeOrchestration == nil {
		t.Fatal("IncludeOrchestration = nil, want explicit false pointer")
	}
	if *got.MCP.IncludeOrchestration {
		t.Error("IncludeOrchestration = true, want false")
	}
	if !got.MCP.Configured() {
		t.Error("Configured() = false despite explicit mcp block")
	}
}
