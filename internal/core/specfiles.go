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

// Config is the root shape of .specd/config.yml: project-wide defaults for
// verification, reporting, role hints, promotion gating, and the
// orchestration/MCP subsystems.
type Config struct {
	Version            int              `json:"version"`
	DefaultVerify      string           `json:"defaultVerify"`
	Report             ReportCfg        `json:"report"`
	Roles              RolesCfg         `json:"roles"`
	PromotionThreshold int              `json:"promotionThreshold"`
	Gates              GatesCfg         `json:"gates"`
	Verify             VerifyCfg        `json:"verify"`
	Orchestration      OrchestrationCfg `json:"orchestration"`
	MCP                MCPConfig        `json:"mcp"`
}

// MCPConfig tunes which tools the native MCP server advertises on tools/list.
// Its zero value is the backward-compatible contract: an absent `mcp` block
// leaves every field zero, which the tool builder treats as "expose everything"
// — byte-identical to pre-config output. IncludeOrchestration is a *bool so an
// unset value (nil) is distinguishable from an explicit false: nil means "derive
// from orchestration.enabled".
type MCPConfig struct {
	Expose               string   `json:"expose"`               // "all" (default) | "essential"
	EssentialTools       []string `json:"essentialTools"`       // commands/intents kept under expose:"essential"
	IncludeMeta          bool     `json:"includeMeta"`          // include install-mutating tools (update/uninstall/schema)
	IncludeOrchestration *bool    `json:"includeOrchestration"` // nil => derive from orchestration.enabled
}

// Configured reports whether an `mcp` block was supplied (any field set). A
// fully zero-value MCPConfig means no block was present, so the tool builder
// falls back to full passthrough for strict backward compatibility.
func (m MCPConfig) Configured() bool {
	return m.Expose != "" || m.IncludeMeta || len(m.EssentialTools) > 0 || m.IncludeOrchestration != nil
}

// OrchestrationCfg configures the worker orchestration subsystem: whether it
// is enabled, approval and worker-mode policy, worker/retry limits, timeouts,
// cost caps, context-compaction policy, transport, program, and resilience settings.
type OrchestrationCfg struct {
	Enabled                  bool    `json:"enabled"`
	ApprovalPolicy           string  `json:"approvalPolicy"`
	WorkerMode               string  `json:"workerMode"`
	MaxWorkers               int     `json:"maxWorkers"`
	MaxRetries               int     `json:"maxRetries"`
	SessionTimeoutMinutes    int     `json:"sessionTimeoutMinutes"`
	HostReportedCostLimitUSD float64 `json:"hostReportedCostLimitUSD"`
	// CompactionPolicy / CompactionBudgetThreshold drive stage-aware context
	// compaction (none|phase|budget|both; threshold in [0,1]). omitempty keeps
	// pre-compaction config files byte-identical.
	CompactionPolicy          string       `json:"compactionPolicy,omitempty"`
	CompactionBudgetThreshold float64      `json:"compactionBudgetThreshold,omitempty"`
	Transport                 TransportCfg `json:"transport"`
	Program                   ProgramCfg   `json:"program"`
	// Resilience holds opt-in checkpoint/auto-resume policy. Pointer + omitempty
	// keeps existing config.json byte-identical when the block is absent.
	Resilience *ResilienceCfg `json:"resilience,omitempty"`
}

// TransportCfg configures the ACP message transport used between Brain and
// Pinky workers: its kind, poll interval, message TTL, lease duration, and
// heartbeat interval.
type TransportCfg struct {
	Kind               string `json:"kind"`
	PollIntervalMillis int    `json:"pollIntervalMillis"`
	MessageTTLSeconds  int    `json:"messageTTLSeconds"`
	LeaseSeconds       int    `json:"leaseSeconds"`
	HeartbeatSeconds   int    `json:"heartbeatSeconds"`
}

// ProgramCfg configures multi-spec program orchestration, currently just the
// cap on specs that may be running concurrently.
type ProgramCfg struct {
	MaxConcurrentSpecs int `json:"maxConcurrentSpecs"`
}

// ResilienceCfg groups the opt-in resilience knobs (checkpointing, auto-resume).
// It is a pointer on OrchestrationCfg with omitempty so a config without a
// `resilience` block marshals byte-identically to the pre-resilience shape; the
// whole feature set is therefore default-off and additive.
type ResilienceCfg struct {
	// CheckpointEnabled gates proactive checkpoint/resume behavior (R1, R4).
	CheckpointEnabled bool          `json:"checkpointEnabled,omitempty"`
	AutoResume        AutoResumeCfg `json:"autoResume,omitempty"`
	// MaxSuspendSeconds caps the cumulative time a worker may keep a task
	// suspended (rate-limited) before it is treated as dead (R3). 0 = use the
	// built-in default (600s); omitempty keeps configs without the field
	// byte-identical.
	MaxSuspendSeconds int `json:"maxSuspendSeconds,omitempty"`
	// ContextSnapshotEnabled gates per-turn context-snapshot writing (R2).
	// Default false; omitempty keeps configs without the field byte-identical.
	ContextSnapshotEnabled bool `json:"contextSnapshotEnabled,omitempty"`
	// ProgressTimeoutSeconds is the window within which an in-flight worker's
	// last progress report keeps a driver wait from counting toward the stall
	// limit (R6). Recommended 300. 0/unset disables progress weighting, keeping
	// today's behavior; omitempty keeps configs without the field byte-identical.
	ProgressTimeoutSeconds int `json:"progressTimeoutSeconds,omitempty"`
}

// defaultMaxSuspendSeconds is the fallback cumulative-suspension cap used when
// resilience.maxSuspendSeconds is unset (0).
const defaultMaxSuspendSeconds = 600

// AutoResumeCfg declares how a host should rediscover and continue sessions on
// startup (R5). Enabled is the master switch; OnHostStart asks the host adapter
// to auto-invoke resume when it boots; MaxAgeMinutes bounds how stale a session
// may be and still be auto-resumed (0 = no age filter).
type AutoResumeCfg struct {
	Enabled       bool `json:"enabled,omitempty"`
	OnHostStart   bool `json:"onHostStart,omitempty"`
	MaxAgeMinutes int  `json:"maxAgeMinutes,omitempty"`
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

// ReportCfg configures the progress report's output format and optional
// auto-refresh interval.
type ReportCfg struct {
	Format             string `json:"format"`
	AutoRefreshSeconds int    `json:"autoRefreshSeconds"`
}

// RolesCfg configures how role guidance is delivered to subagents.
type RolesCfg struct {
	SubagentMode string `json:"subagentMode"`
}

// GatesCfg configures the severity of each verify-time quality gate
// (traceability, acceptance, diff scope, context budget, mode capability)
// plus any external custom gates to run after the core pipeline.
type GatesCfg struct {
	Traceability string `json:"traceability"`
	Acceptance   string `json:"acceptance"`
	// Scope is the diff-scope gate severity: "off"/""/"*" = no-op, else
	// "warn"/"error". It flags verify-time changed files outside a task's
	// declared `files:` contract.
	Scope string `json:"scope"`
	// ContextBudget is the opt-in context-budget gate severity:
	// "off"/""/"*" = no-op (default), else "warn"/"error". When enabled it builds
	// the active spec's context manifest and flags it when the required-item token
	// estimate exceeds the effective budget, naming the heaviest items.
	ContextBudget string `json:"contextBudget"`
	// MaxContextTokens optionally caps the gate's effective budget (mirrors the MCP
	// capabilities.specd.maxContextTokens host hint). 0 = use the derived budget.
	MaxContextTokens int `json:"maxContextTokens"`
	// ModeCapability is the opt-in mode-capability gate severity:
	// "off"/""/"*" = no-op (default), else "warn"/"error". When enabled it flags a
	// spec recorded as orchestrated while the project lacks orchestration
	// capability (orchestration.enabled absent/false). Off by default keeps Base
	// projects clean.
	ModeCapability string `json:"modeCapability"`
	// Custom lists external, declarative custom gates run after the core
	// pipeline. Each is an ordinary subprocess (no Go plugin, no network).
	Custom []CustomGateCfg `json:"custom"`
}

// CustomGateCfg declares one external custom gate. Command is run via the verify
// shell with a scrubbed env and bounded timeout; Severity ("warn"|"error", default
// "error") decides how its findings map into the check result.
//
// Sandbox is the opt-in isolation backend for this gate's command, reusing the
// verify sandbox runner ("none" (default), "bwrap", "container"). The gate
// command is trusted operator input (not agent-authored), so it runs on the host
// with a scrubbed env by default; an operator who wants parity with verify's
// fail-closed sandbox sets this. An unavailable backend fails the gate closed.
type CustomGateCfg struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Severity string `json:"severity"`
	Sandbox  string `json:"sandbox,omitempty"`
}

// DefaultConfig is the Config used when .specd/config.yml is absent or omits
// a field, providing safe out-of-the-box defaults for verify, gates, and
// orchestration.
var DefaultConfig = Config{
	Version:            1,
	DefaultVerify:      "echo 'specd: defaultVerify is unset — set it to your repo test command (see the specd-steering skill)' >&2; exit 1",
	Report:             ReportCfg{Format: "md", AutoRefreshSeconds: 0},
	Roles:              RolesCfg{SubagentMode: "inline"},
	PromotionThreshold: 3,
	Gates:              GatesCfg{Traceability: "warn", Acceptance: "off", Scope: "off", ContextBudget: "off", Custom: []CustomGateCfg{}},
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

// ValidateOrchestrationConfig validates authority-bearing values and normalizes
// resource limits. Callers receive an error for ambiguous or unsafe policy;
// bounded integers are clamped through clampOrchestrationInt.
//
//nolint:gocyclo // pre-existing complexity debt, out of scope for spec S3 — tracked for a future cleanup pass
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
	if cfg.CompactionPolicy != "" &&
		!oneOf(cfg.CompactionPolicy, CompactionNone, CompactionPhase, CompactionBudget, CompactionBoth) {
		return fmt.Errorf("unsupported compactionPolicy %q", cfg.CompactionPolicy)
	}
	if math.IsNaN(cfg.CompactionBudgetThreshold) ||
		math.IsInf(cfg.CompactionBudgetThreshold, 0) ||
		cfg.CompactionBudgetThreshold < 0 || cfg.CompactionBudgetThreshold > 1 {
		return fmt.Errorf("compactionBudgetThreshold must be finite and within [0,1]")
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
	if cfg.Resilience != nil && cfg.Resilience.AutoResume.MaxAgeMinutes < 0 {
		return fmt.Errorf("resilience.autoResume.maxAgeMinutes must be non-negative")
	}
	if cfg.Resilience != nil && cfg.Resilience.MaxSuspendSeconds != 0 &&
		(cfg.Resilience.MaxSuspendSeconds < 0 || cfg.Resilience.MaxSuspendSeconds > 3600) {
		return fmt.Errorf("resilience.maxSuspendSeconds must be within (0,3600]")
	}
	if cfg.Resilience != nil && cfg.Resilience.ProgressTimeoutSeconds != 0 &&
		(cfg.Resilience.ProgressTimeoutSeconds < 0 || cfg.Resilience.ProgressTimeoutSeconds > 3600) {
		return fmt.Errorf("resilience.progressTimeoutSeconds must be within (0,3600]")
	}
	return nil
}

// EffectiveMaxSuspendSeconds resolves the cumulative-suspension cap, applying the
// built-in default when the config block is absent or leaves the field unset.
func (cfg OrchestrationCfg) EffectiveMaxSuspendSeconds() int {
	if cfg.Resilience == nil || cfg.Resilience.MaxSuspendSeconds == 0 {
		return defaultMaxSuspendSeconds
	}
	return cfg.Resilience.MaxSuspendSeconds
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

// Artifacts lists the standard per-spec markdown filenames managed under a
// spec's directory.
var Artifacts = []string{
	"requirements.md", "design.md", "tasks.md",
	"decisions.md", "memory.md", "mid-requirements.md",
}

// ArtifactPath returns the on-disk path of a named artifact file within the
// given spec's directory.
func ArtifactPath(root, slug, name string) string {
	return filepath.Join(SpecDir(root, slug), name)
}

// ReadArtifact reads a named artifact file for a spec, returning nil if it
// does not exist.
func ReadArtifact(root, slug, name string) *string {
	return ReadOrNull(ArtifactPath(root, slug, name))
}

// ReadRole reads a role definition file (e.g. .specd/roles/<role>.md),
// returning nil if it does not exist.
func ReadRole(root, role string) *string {
	return ReadOrNull(filepath.Join(RolesDir(root), role+".md"))
}

// SpecExists reports whether a spec with the given slug has a state.json
// under .specd/specs/.
func SpecExists(root, slug string) bool {
	_, err := os.Stat(filepath.Join(SpecDir(root, slug), "state.json"))
	return err == nil
}

// RequireSpec validates slug and returns a NotFoundError if no spec with
// that slug exists under .specd/specs/.
func RequireSpec(root, slug string) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}
	if !SpecExists(root, slug) {
		return NotFoundError("spec '" + slug + "' not found under .specd/specs/")
	}
	return nil
}

// ListSpecs returns the slugs of every spec under .specd/specs that has a
// state.json, sorted alphabetically. It returns nil if the specs directory
// cannot be read.
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

// Reconcile rebuilds state.Tasks from doc's parsed tasks.md, carrying over
// each existing task's runtime fields (status, timestamps, evidence,
// verification, blocker, telemetry) and role when still present, and drops
// blockers whose task no longer exists.
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
			ts.Role = "craftsman"
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

// ParseTasksMd reads and parses a spec's tasks.md, returning an empty
// ParsedTasks (titled by slug) if the file is missing or blank.
func ParseTasksMd(root, slug string) (ParsedTasks, error) {
	raw := ReadArtifact(root, slug, "tasks.md")
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return ParsedTasks{Title: slug, Tasks: nil}, nil
	}
	return ParseTasks(*raw)
}

// LoadedSpec bundles a spec's persisted state with its parsed tasks.md.
type LoadedSpec struct {
	State *State
	Doc   ParsedTasks
}

// LoadSpec loads a spec's state and tasks.md under the spec lock,
// reconciling state against the parsed tasks and persisting the result if
// reconciliation changed anything.
func LoadSpec(root, slug string) (LoadedSpec, error) {
	if err := RequireSpec(root, slug); err != nil {
		return LoadedSpec{}, err
	}
	return WithSpecLock[LoadedSpec](root, slug, func() (LoadedSpec, error) {
		state, err := LoadState(root, slug)
		if err != nil {
			return LoadedSpec{}, err
		}
		if state == nil {
			return LoadedSpec{}, GateError(fmt.Sprintf("state.json for spec '%s' is missing — concurrent delete detected, reload and retry", slug))
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
