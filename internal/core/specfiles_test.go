package core

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
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
		{"invalid enum", `{"enabled":true,"workerMode":"embedded"}`},
		{"api key", `{"enabled":true,"apiKey":"top-secret"}`},
		{"provider credentials", `{"enabled":true,"providerCredentials":{"user":"x"}}`},
		{"shell command", `{"enabled":true,"transport":{"command":"curl example.test"}}`},
		{"model selection", `{"enabled":true,"model":"vendor-model"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			raw := `{"defaultVerify":"make test","orchestration":` + tt.orchestration + `}`
			if err := AtomicWrite(ConfigPath(root), raw); err != nil {
				t.Fatal(err)
			}

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
