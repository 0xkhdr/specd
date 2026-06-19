package core

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Config struct {
	Version            int              `json:"version"`
	DefaultVerify      string           `json:"defaultVerify"`
	Report             ReportCfg        `json:"report"`
	Roles              RolesCfg         `json:"roles"`
	PromotionThreshold int              `json:"promotionThreshold"`
	Gates              GatesCfg         `json:"gates"`
	Verify             VerifyCfg        `json:"verify"`
	Orchestration      OrchestrationCfg `json:"orchestration"`
}

type OrchestrationCfg struct {
	Enabled                  bool         `json:"enabled"`
	ApprovalPolicy           string       `json:"approvalPolicy"`
	WorkerMode               string       `json:"workerMode"`
	MaxWorkers               int          `json:"maxWorkers"`
	MaxRetries               int          `json:"maxRetries"`
	SessionTimeoutMinutes    int          `json:"sessionTimeoutMinutes"`
	HostReportedCostLimitUSD float64      `json:"hostReportedCostLimitUSD"`
	Transport                TransportCfg `json:"transport"`
	Program                  ProgramCfg   `json:"program"`
}

type TransportCfg struct {
	Kind               string `json:"kind"`
	PollIntervalMillis int    `json:"pollIntervalMillis"`
	MessageTTLSeconds  int    `json:"messageTTLSeconds"`
	LeaseSeconds       int    `json:"leaseSeconds"`
	HeartbeatSeconds   int    `json:"heartbeatSeconds"`
}

type ProgramCfg struct {
	MaxConcurrentSpecs int `json:"maxConcurrentSpecs"`
}

const (
	minMaxWorkers            = 1
	maxMaxWorkers            = 64
	minMaxRetries            = 0
	maxMaxRetries            = 10
	minSessionTimeoutMinutes = 1
	maxSessionTimeoutMinutes = 24 * 60
	minPollIntervalMillis    = 50
	maxPollIntervalMillis    = 60 * 1000
	minMessageTTLSeconds     = 60
	maxMessageTTLSeconds     = 24 * 60 * 60
	minLeaseSeconds          = 10
	maxLeaseSeconds          = 60 * 60
	minHeartbeatSeconds      = 1
	maxHeartbeatSeconds      = 20 * 60
	minMaxConcurrentSpecs    = 1
	maxMaxConcurrentSpecs    = 64
)

// VerifyCfg holds verify-execution policy. Sandbox selects the isolation backend
// ("none"|"bwrap"|"container"); empty means the default unsandboxed shell runner.
type VerifyCfg struct {
	Sandbox string `json:"sandbox"`
}

type ReportCfg struct {
	Format             string `json:"format"`
	AutoRefreshSeconds int    `json:"autoRefreshSeconds"`
}

type RolesCfg struct {
	SubagentMode string `json:"subagentMode"`
}

type GatesCfg struct {
	Traceability string `json:"traceability"`
	Acceptance   string `json:"acceptance"`
	// Scope is the diff-scope gate severity: "off"/""/"*" = no-op, else
	// "warn"/"error". It flags verify-time changed files outside a task's
	// declared `files:` contract.
	Scope string `json:"scope"`
	// Custom lists external, declarative custom gates run after the core
	// pipeline. Each is an ordinary subprocess (no Go plugin, no network).
	Custom []CustomGateCfg `json:"custom"`
}

// CustomGateCfg declares one external custom gate. Command is run via the verify
// shell with a scrubbed env and bounded timeout; Severity ("warn"|"error", default
// "error") decides how its findings map into the check result.
type CustomGateCfg struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Severity string `json:"severity"`
}

var DefaultConfig = Config{
	Version:            1,
	DefaultVerify:      "npm test",
	Report:             ReportCfg{Format: "md", AutoRefreshSeconds: 0},
	Roles:              RolesCfg{SubagentMode: "inline"},
	PromotionThreshold: 3,
	Gates:              GatesCfg{Traceability: "warn", Acceptance: "off", Scope: "off", Custom: []CustomGateCfg{}},
	Verify:             VerifyCfg{Sandbox: "none"},
	Orchestration: OrchestrationCfg{
		Enabled:                  false,
		ApprovalPolicy:           "manual",
		WorkerMode:               "host",
		MaxWorkers:               4,
		MaxRetries:               2,
		SessionTimeoutMinutes:    120,
		HostReportedCostLimitUSD: 0,
		Transport: TransportCfg{
			Kind:               "file",
			PollIntervalMillis: 500,
			MessageTTLSeconds:  3600,
			LeaseSeconds:       120,
			HeartbeatSeconds:   30,
		},
		Program: ProgramCfg{MaxConcurrentSpecs: 2},
	},
}

func LoadConfig(root string) Config {
	raw := ReadOrNull(ConfigPath(root))
	if raw == nil {
		return DefaultConfig
	}
	var partial struct {
		Version            *int       `json:"version"`
		DefaultVerify      *string    `json:"defaultVerify"`
		Report             *ReportCfg `json:"report"`
		Roles              *RolesCfg  `json:"roles"`
		PromotionThreshold *int       `json:"promotionThreshold"`
		Gates              *GatesCfg  `json:"gates"`
		Verify             *VerifyCfg `json:"verify"`
		Orchestration      *struct {
			Enabled                  *bool    `json:"enabled"`
			ApprovalPolicy           *string  `json:"approvalPolicy"`
			WorkerMode               *string  `json:"workerMode"`
			MaxWorkers               *int     `json:"maxWorkers"`
			MaxRetries               *int     `json:"maxRetries"`
			SessionTimeoutMinutes    *int     `json:"sessionTimeoutMinutes"`
			HostReportedCostLimitUSD *float64 `json:"hostReportedCostLimitUSD"`
			Transport                *struct {
				Kind               *string `json:"kind"`
				PollIntervalMillis *int    `json:"pollIntervalMillis"`
				MessageTTLSeconds  *int    `json:"messageTTLSeconds"`
				LeaseSeconds       *int    `json:"leaseSeconds"`
				HeartbeatSeconds   *int    `json:"heartbeatSeconds"`
			} `json:"transport"`
			Program *struct {
				MaxConcurrentSpecs *int `json:"maxConcurrentSpecs"`
			} `json:"program"`
		} `json:"orchestration"`
	}
	if err := json.Unmarshal([]byte(*raw), &partial); err != nil {
		return DefaultConfig
	}
	cfg := DefaultConfig
	if partial.Version != nil {
		cfg.Version = *partial.Version
	}
	if partial.DefaultVerify != nil {
		cfg.DefaultVerify = *partial.DefaultVerify
	}
	if partial.Report != nil {
		if partial.Report.Format != "" {
			cfg.Report.Format = partial.Report.Format
		}
		cfg.Report.AutoRefreshSeconds = partial.Report.AutoRefreshSeconds
	}
	if partial.Roles != nil && partial.Roles.SubagentMode != "" {
		cfg.Roles.SubagentMode = partial.Roles.SubagentMode
	}
	if partial.PromotionThreshold != nil {
		cfg.PromotionThreshold = *partial.PromotionThreshold
	}
	if partial.Gates != nil {
		if partial.Gates.Traceability != "" {
			cfg.Gates.Traceability = partial.Gates.Traceability
		}
		if partial.Gates.Acceptance != "" {
			cfg.Gates.Acceptance = partial.Gates.Acceptance
		}
		if partial.Gates.Scope != "" {
			cfg.Gates.Scope = partial.Gates.Scope
		}
		if partial.Gates.Custom != nil {
			cfg.Gates.Custom = partial.Gates.Custom
		}
	}
	if partial.Verify != nil && partial.Verify.Sandbox != "" {
		cfg.Verify.Sandbox = partial.Verify.Sandbox
	}
	if partial.Orchestration != nil {
		orchestration := partial.Orchestration
		if orchestration.Enabled != nil {
			cfg.Orchestration.Enabled = *orchestration.Enabled
		}
		if orchestration.ApprovalPolicy != nil {
			cfg.Orchestration.ApprovalPolicy = *orchestration.ApprovalPolicy
		}
		if orchestration.WorkerMode != nil {
			cfg.Orchestration.WorkerMode = *orchestration.WorkerMode
		}
		if orchestration.MaxWorkers != nil {
			cfg.Orchestration.MaxWorkers = *orchestration.MaxWorkers
		}
		if orchestration.MaxRetries != nil {
			cfg.Orchestration.MaxRetries = *orchestration.MaxRetries
		}
		if orchestration.SessionTimeoutMinutes != nil {
			cfg.Orchestration.SessionTimeoutMinutes = *orchestration.SessionTimeoutMinutes
		}
		if orchestration.HostReportedCostLimitUSD != nil {
			cfg.Orchestration.HostReportedCostLimitUSD = *orchestration.HostReportedCostLimitUSD
		}
		if orchestration.Transport != nil {
			transport := orchestration.Transport
			if transport.Kind != nil {
				cfg.Orchestration.Transport.Kind = *transport.Kind
			}
			if transport.PollIntervalMillis != nil {
				cfg.Orchestration.Transport.PollIntervalMillis = *transport.PollIntervalMillis
			}
			if transport.MessageTTLSeconds != nil {
				cfg.Orchestration.Transport.MessageTTLSeconds = *transport.MessageTTLSeconds
			}
			if transport.LeaseSeconds != nil {
				cfg.Orchestration.Transport.LeaseSeconds = *transport.LeaseSeconds
			}
			if transport.HeartbeatSeconds != nil {
				cfg.Orchestration.Transport.HeartbeatSeconds = *transport.HeartbeatSeconds
			}
		}
		if orchestration.Program != nil && orchestration.Program.MaxConcurrentSpecs != nil {
			cfg.Orchestration.Program.MaxConcurrentSpecs = *orchestration.Program.MaxConcurrentSpecs
		}
		if err := rejectSecretBearingOrchestration([]byte(*raw)); err != nil {
			Warn("orchestration config rejected: " + err.Error() + " — using disabled defaults")
			cfg.Orchestration = DefaultConfig.Orchestration
			return cfg
		}
		if err := ValidateOrchestrationConfig(&cfg.Orchestration); err != nil {
			Warn("orchestration config rejected: " + err.Error() + " — using disabled defaults")
			cfg.Orchestration = DefaultConfig.Orchestration
		}
	}
	return cfg
}

// ValidateOrchestrationConfig validates authority-bearing values and normalizes
// resource limits. Callers receive an error for ambiguous or unsafe policy;
// bounded integers are clamped through clampOrchestrationInt.
func ValidateOrchestrationConfig(cfg *OrchestrationCfg) error {
	if cfg == nil {
		return fmt.Errorf("policy is missing")
	}
	if !oneOf(cfg.ApprovalPolicy, "manual", "planning", "session") {
		return fmt.Errorf("unsupported approvalPolicy %q", cfg.ApprovalPolicy)
	}
	if cfg.WorkerMode != "host" {
		return fmt.Errorf("unsupported workerMode %q", cfg.WorkerMode)
	}
	if cfg.Transport.Kind != "file" {
		return fmt.Errorf("unsupported transport kind %q", cfg.Transport.Kind)
	}
	if math.IsNaN(cfg.HostReportedCostLimitUSD) ||
		math.IsInf(cfg.HostReportedCostLimitUSD, 0) ||
		cfg.HostReportedCostLimitUSD < 0 {
		return fmt.Errorf("hostReportedCostLimitUSD must be a finite non-negative number")
	}

	cfg.MaxWorkers = clampOrchestrationInt("maxWorkers", cfg.MaxWorkers, minMaxWorkers, maxMaxWorkers)
	cfg.MaxRetries = clampOrchestrationInt("maxRetries", cfg.MaxRetries, minMaxRetries, maxMaxRetries)
	cfg.SessionTimeoutMinutes = clampOrchestrationInt(
		"sessionTimeoutMinutes",
		cfg.SessionTimeoutMinutes,
		minSessionTimeoutMinutes,
		maxSessionTimeoutMinutes,
	)
	cfg.Transport.PollIntervalMillis = clampOrchestrationInt(
		"transport.pollIntervalMillis",
		cfg.Transport.PollIntervalMillis,
		minPollIntervalMillis,
		maxPollIntervalMillis,
	)
	cfg.Transport.MessageTTLSeconds = clampOrchestrationInt(
		"transport.messageTTLSeconds",
		cfg.Transport.MessageTTLSeconds,
		minMessageTTLSeconds,
		maxMessageTTLSeconds,
	)
	cfg.Transport.LeaseSeconds = clampOrchestrationInt(
		"transport.leaseSeconds",
		cfg.Transport.LeaseSeconds,
		minLeaseSeconds,
		maxLeaseSeconds,
	)
	cfg.Transport.HeartbeatSeconds = clampOrchestrationInt(
		"transport.heartbeatSeconds",
		cfg.Transport.HeartbeatSeconds,
		minHeartbeatSeconds,
		maxHeartbeatSeconds,
	)
	cfg.Program.MaxConcurrentSpecs = clampOrchestrationInt(
		"program.maxConcurrentSpecs",
		cfg.Program.MaxConcurrentSpecs,
		minMaxConcurrentSpecs,
		maxMaxConcurrentSpecs,
	)

	if cfg.Transport.HeartbeatSeconds >= cfg.Transport.LeaseSeconds {
		return fmt.Errorf("transport.heartbeatSeconds must be less than transport.leaseSeconds")
	}
	if cfg.Transport.LeaseSeconds > cfg.Transport.MessageTTLSeconds {
		return fmt.Errorf("transport.leaseSeconds must not exceed transport.messageTTLSeconds")
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func clampOrchestrationInt(name string, value, min, max int) int {
	clamped := value
	if clamped < min {
		clamped = min
	}
	if clamped > max {
		clamped = max
	}
	if clamped != value {
		Warn(fmt.Sprintf("orchestration.%s: %d outside [%d,%d] — using %d", name, value, min, max, clamped))
	}
	return clamped
}

func rejectSecretBearingOrchestration(raw []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil
	}
	orchestration, ok := root["orchestration"]
	if !ok {
		return nil
	}
	var value any
	if err := json.Unmarshal(orchestration, &value); err != nil {
		return nil
	}
	return findSecretBearingKey(value, "orchestration")
}

func findSecretBearingKey(value any, path string) error {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if isSecretBearingKey(key) {
				return fmt.Errorf("%s.%s is not an allowed policy field", path, key)
			}
			if err := findSecretBearingKey(typed[key], path+"."+key); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range typed {
			if err := findSecretBearingKey(child, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func isSecretBearingKey(key string) bool {
	normalized := strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.ToLower(key))
	for _, fragment := range []string{
		"apikey",
		"credential",
		"password",
		"provider",
		"secret",
		"shell",
		"command",
		"script",
		"token",
		"model",
	} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

var Artifacts = []string{
	"requirements.md", "design.md", "tasks.md",
	"decisions.md", "memory.md", "mid-requirements.md",
}

func ArtifactPath(root, slug, name string) string {
	return filepath.Join(SpecDir(root, slug), name)
}

func ReadArtifact(root, slug, name string) *string {
	return ReadOrNull(ArtifactPath(root, slug, name))
}

func ReadRole(root, role string) *string {
	return ReadOrNull(filepath.Join(RolesDir(root), role+".md"))
}

func SpecExists(root, slug string) bool {
	_, err := os.Stat(filepath.Join(SpecDir(root, slug), "state.json"))
	return err == nil
}

func RequireSpec(root, slug string) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}
	if !SpecExists(root, slug) {
		return NotFoundError("spec '" + slug + "' not found under .specd/specs/")
	}
	return nil
}

func ListSpecs(root string) []string {
	dir := filepath.Join(root, ".specd", "specs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, e.Name(), "state.json")); err == nil {
				out = append(out, e.Name())
			}
		}
	}
	sort.Strings(out)
	return out
}

func Reconcile(state *State, doc ParsedTasks) {
	next := make(map[string]TaskState, len(doc.Tasks))
	for _, t := range doc.Tasks {
		prev, hasPrev := state.Tasks[t.ID]
		depends := ParseDepends(t.Meta["depends"])
		var reqs []int
		if _, ok := t.Meta["requirements"]; ok {
			reqs = ParseRequirements(t.Meta["requirements"])
		} else if hasPrev {
			reqs = prev.Requirements
		}
		ts := TaskState{
			ID:           t.ID,
			Title:        t.Title,
			Wave:         t.Wave,
			Depends:      depends,
			Requirements: reqs,
			Status:       TaskPending,
		}
		if t.Meta["role"] != "" {
			ts.Role = t.Meta["role"]
		} else if hasPrev {
			ts.Role = prev.Role
		}
		if ts.Role == "" {
			ts.Role = "builder"
		}
		if hasPrev {
			ts.Status = prev.Status
			ts.StartedAt = prev.StartedAt
			ts.FinishedAt = prev.FinishedAt
			ts.Evidence = prev.Evidence
			ts.Verification = prev.Verification
			ts.Blocker = prev.Blocker
			ts.Telemetry = prev.Telemetry
		}
		if ts.Depends == nil {
			ts.Depends = []string{}
		}
		if ts.Requirements == nil {
			ts.Requirements = []int{}
		}
		next[t.ID] = ts
	}
	state.Tasks = next
	var blockers []Blocker
	for _, b := range state.Blockers {
		if _, ok := next[b.Task]; ok {
			blockers = append(blockers, b)
		}
	}
	if blockers == nil {
		blockers = []Blocker{}
	}
	state.Blockers = blockers
}

func ParseTasksMd(root, slug string) (ParsedTasks, error) {
	raw := ReadArtifact(root, slug, "tasks.md")
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return ParsedTasks{Title: slug, Tasks: nil}, nil
	}
	return ParseTasks(*raw)
}

type LoadedSpec struct {
	State *State
	Doc   ParsedTasks
}

func LoadSpec(root, slug string) (LoadedSpec, error) {
	if err := RequireSpec(root, slug); err != nil {
		return LoadedSpec{}, err
	}
	return WithSpecLock[LoadedSpec](root, slug, func() (LoadedSpec, error) {
		state, err := LoadState(root, slug)
		if err != nil {
			return LoadedSpec{}, err
		}
		doc, err := ParseTasksMd(root, slug)
		if err != nil {
			return LoadedSpec{}, err
		}
		before, _ := json.Marshal(state.Tasks)
		Reconcile(state, doc)
		after, _ := json.Marshal(state.Tasks)
		if string(before) != string(after) {
			if err := SaveState(root, slug, state); err != nil {
				return LoadedSpec{}, err
			}
		}
		return LoadedSpec{State: state, Doc: doc}, nil
	})
}
