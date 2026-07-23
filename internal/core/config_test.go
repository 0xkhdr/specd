package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestConfigSourceResolution(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, "project.yml")
	canonical := filepath.Join(root, ".specd", "config.yaml")
	policy := []byte("agent: codex\ncontext:\n  max_tokens: 2000\n")
	if err := os.WriteFile(legacy, policy, 0o600); err != nil {
		t.Fatal(err)
	}

	resolution, err := ResolveConfigSource(filepath.Join(root, ".specd"))
	if err != nil || resolution.SelectedPath != legacy || resolution.SelectedKind != "legacy" || resolution.SourceDigest == "" || resolution.EffectiveDigest == "" || len(resolution.Deprecations) != 1 {
		t.Fatalf("legacy resolution = %#v, err=%v", resolution, err)
	}
	if err := os.WriteFile(canonical, []byte("context:\n  max_tokens: 2000\nagent: codex\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolution, err = ResolveConfigSource(root)
	if err != nil || resolution.SelectedPath != canonical || len(resolution.DuplicatePaths) != 1 {
		t.Fatalf("equal canonical resolution = %#v, err=%v", resolution, err)
	}
	if err := os.WriteFile(canonical, []byte("agent: weaker\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ResolveConfigSource(root)
	var conflict ConfigConflictError
	if !errors.As(err, &conflict) || !slices.Contains(conflict.Keys, "agent") {
		t.Fatalf("conflict = %#v, err=%v", conflict, err)
	}
	if err := os.WriteFile(canonical, []byte("agent: [unsupported]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = ResolveConfigSource(root); err == nil || !strings.Contains(err.Error(), "config line 1") {
		t.Fatalf("malformed canonical fell back: %v", err)
	}
}

func TestConfigSourceResolutionSymlinkRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".specd", "config.yaml"), []byte("agent: codex\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "project")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	fromRoot, err := ResolveConfigSource(root)
	if err != nil {
		t.Fatal(err)
	}
	fromLink, err := ResolveConfigSource(filepath.Join(link, ".specd", "nested"))
	if err != nil || fromLink.Root != fromRoot.Root || fromLink.EffectiveDigest != fromRoot.EffectiveDigest {
		t.Fatalf("root=%#v link=%#v err=%v", fromRoot, fromLink, err)
	}
}

func TestConfigStrictUnsupportedSyntax(t *testing.T) {
	for _, raw := range []string{"---\nagent: codex\n", "items:\n  - one\n", "agent: &base codex\n", "agent: codex\nagent: other\n", "agent: [codex]\n"} {
		if _, err := parseSimpleYAML(raw); err == nil || !strings.Contains(err.Error(), "config line") {
			t.Fatalf("accepted unsupported YAML %q: %v", raw, err)
		}
	}
}

func TestConfigListAndCommentConformance(t *testing.T) {
	clean := strings.Join([]string{
		"agent: codex",
		"verify:",
		`  trivial: "printf ok,true,:"`,
		"routing:",
		"  classes: basic,reviewed",
		"  default_class: basic",
		"  fallback: reviewed,basic",
		"  class_capabilities: basic=context;reviewed=context+review",
		"  recommendations: low=basic,high=reviewed",
		"environments:",
		"  staging: strategy=rolling;criteria=health+latency;window=5m;freshness=2m;rollback=previous",
		"orchestration:",
		`  model: "model#release"`,
		"",
	}, "\n")
	commented := strings.Join([]string{
		"# whole-line comments are ignored, including unsupported-looking [] & * syntax",
		"agent: codex # unquoted trailing comment",
		"verify:",
		`  trivial: "printf ok,true,:" # comma-separated commands`,
		"routing:",
		"  classes: basic,reviewed # comma-separated classes",
		"  default_class: basic",
		"  fallback: reviewed,basic # comma-separated fallback",
		"  class_capabilities: basic=context;reviewed=context+review # semicolon entries, plus capabilities",
		"  recommendations: low=basic,high=reviewed # comma-separated entries",
		"environments:",
		"  staging: strategy=rolling;criteria=health+latency;window=5m;freshness=2m;rollback=previous # semicolon fields, plus criteria",
		"orchestration:",
		`  model: "model#release" # quoted hash is data`,
		"",
	}, "\n")

	load := func(name, raw string) Config {
		t.Helper()
		path := filepath.Join(t.TempDir(), name+".yaml")
		if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, diagnostics := LoadConfig(ConfigPaths{Project: path}, nil)
		if len(diagnostics) != 0 {
			t.Fatalf("%s diagnostics = %#v", name, diagnostics)
		}
		return cfg
	}
	want, got := load("clean", clean), load("commented", commented)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commented config differs from documented form:\n got: %#v\nwant: %#v", got, want)
	}
	if !reflect.DeepEqual(got.Verify.Trivial, []string{"printf ok", "true", ":"}) ||
		!reflect.DeepEqual(got.Routing.Classes, []string{"basic", "reviewed"}) ||
		!reflect.DeepEqual(got.Routing.Fallback, []string{"reviewed", "basic"}) ||
		!reflect.DeepEqual(got.Routing.ClassCapabilities["reviewed"], []string{"context", "review"}) ||
		!reflect.DeepEqual(got.Routing.Recommendations, map[string]string{"low": "basic", "high": "reviewed"}) ||
		!reflect.DeepEqual(got.Environments[EnvironmentStaging].HealthCriteria, []string{"health", "latency"}) {
		t.Fatalf("documented list separators parsed incorrectly: %#v", got)
	}

	values, err := parseSimpleYAML("orchestration:\n  model: 'single#hash' # comment\nsubmit:\n  command: 'echo \"double#hash\"' # comment\n")
	if err != nil {
		t.Fatal(err)
	}
	if values["orchestration.model"] != "single#hash" || values["submit.command"] != `echo "double#hash"` {
		t.Fatalf("quoted hashes were stripped: %#v", values)
	}
}

func TestConfigCascade(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project.yml")
	if err := os.WriteFile(project, []byte(strings.Join([]string{
		"agent: codex",
		"gates:",
		"  verify: warn",
		"context:",
		"  max_tokens: 2000",
		"orchestration:",
		"  enabled: true",
		"  api_key: should-not-apply",
		"  model: project-model",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, diagnostics := LoadConfig(ConfigPaths{Project: project}, map[string]string{
		"SPECD_GATES_VERIFY":       "error",
		"SPECD_CONTEXT_MAX_TOKENS": "3000",
	})

	if cfg.Agent != "codex" {
		t.Fatalf("agent = %q, want project codex", cfg.Agent)
	}
	if cfg.Gates.Verify != "error" {
		t.Fatalf("gates.verify = %q, want env override error", cfg.Gates.Verify)
	}
	if cfg.Context.MaxTokens != 3000 {
		t.Fatalf("context.max_tokens = %d, want env override 3000", cfg.Context.MaxTokens)
	}
	if !cfg.Orchestration.Enabled {
		t.Fatal("orchestration.enabled = false, want project true")
	}
	if cfg.Orchestration.Model != "project-model" {
		t.Fatalf("orchestration.model = %q, want project-model", cfg.Orchestration.Model)
	}
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "secret value not allowed") {
		t.Fatalf("diagnostics = %#v, want secret scrub diagnostic", diagnostics)
	}

	bad := filepath.Join(dir, "bad.yml")
	if err := os.WriteFile(bad, []byte("agent codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diagnostics = LoadConfig(ConfigPaths{Project: bad}, nil)
	if len(diagnostics) != 1 || diagnostics[0].Severity != "error" {
		t.Fatalf("bad yaml diagnostics = %#v, want fail-loud error", diagnostics)
	}
}

func TestConfigIgnoresStrayJSON(t *testing.T) {
	dir := t.TempDir()
	stray := filepath.Join(dir, "config.json")
	if err := os.WriteFile(stray, []byte(`{"agent":"stray"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, diagnostics := LoadConfig(ConfigPaths{Project: stray}, nil)
	if len(diagnostics) != 0 {
		t.Fatalf("stray json diagnostics = %#v, want ignored", diagnostics)
	}
	if !reflect.DeepEqual(cfg, DefaultConfig) {
		t.Fatalf("stray config changed cfg = %#v, want default %#v", cfg, DefaultConfig)
	}
}

// TestVerifyTimeoutConfig pins the verify.timeout_seconds key (gap 4.2): a valid
// value parses onto Verify.TimeoutSecs, a negative value is a loud error, and env
// overrides project.
func TestVerifyTimeoutConfig(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project.yml")
	if err := os.WriteFile(project, []byte("verify:\n  timeout_seconds: 30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, diags := LoadConfig(ConfigPaths{Project: project}, nil)
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diags)
	}
	if cfg.Verify.TimeoutSecs != 30 {
		t.Fatalf("verify.timeout_seconds = %d, want 30", cfg.Verify.TimeoutSecs)
	}
	cfg, _ = LoadConfig(ConfigPaths{Project: project}, map[string]string{"SPECD_VERIFY_TIMEOUT_SECONDS": "5"})
	if cfg.Verify.TimeoutSecs != 5 {
		t.Fatalf("env override verify.timeout_seconds = %d, want 5", cfg.Verify.TimeoutSecs)
	}

	bad := filepath.Join(dir, "bad.yml")
	if err := os.WriteFile(bad, []byte("verify:\n  timeout_seconds: -1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diags = LoadConfig(ConfigPaths{Project: bad}, nil)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "verify.timeout_seconds") {
		t.Fatalf("negative timeout diagnostics = %#v, want one verify.timeout_seconds error", diags)
	}
}

func TestConfigSecurityProfile(t *testing.T) {
	if DefaultConfig.Security.Profile != "prototype" {
		t.Fatalf("default profile = %q", DefaultConfig.Security.Profile)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yml")
	if err := os.WriteFile(path, []byte("security:\n  profile: production\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, diags := LoadConfig(ConfigPaths{Project: path}, nil)
	if len(diags) != 0 || cfg.Security.Profile != "production" {
		t.Fatalf("cfg=%+v diags=%v", cfg.Security, diags)
	}
	if err := os.WriteFile(path, []byte("security:\n  profile: unsafe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diags = LoadConfig(ConfigPaths{Project: path}, nil)
	if len(diags) != 1 {
		t.Fatalf("invalid profile diagnostics=%v", diags)
	}
}

func TestConfigRoutingPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yml")
	raw := strings.Join([]string{
		"routing:",
		"  version: 1",
		"  classes: basic,reviewed,sandboxed",
		"  fallback: sandboxed,reviewed,basic",
		"  class_capabilities: basic=context;reviewed=context+review;sandboxed=context+review+sandbox+eval",
		"  max_tokens: 50000",
		"  max_cost_micros: 250000",
		"  deadline_seconds: 900",
		"  max_retries: 2",
		"  allow_unknown_telemetry: false",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, diags := LoadConfig(ConfigPaths{Project: path}, nil)
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %#v", diags)
	}
	if cfg.Routing.Version != "1" || cfg.Routing.MaxTokens != 50000 || cfg.Routing.MaxCostMicros != 250000 || cfg.Routing.DeadlineSeconds != 900 || cfg.Routing.MaxRetries != 2 || cfg.Routing.AllowUnknownTelemetry {
		t.Fatalf("routing = %#v", cfg.Routing)
	}
	if got := cfg.Routing.ClassCapabilities["sandboxed"]; !reflect.DeepEqual(got, []string{"context", "eval", "review", "sandbox"}) {
		t.Fatalf("sandboxed capabilities = %#v", got)
	}
}

func TestConfigRoutingRecommendationDeterministic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.yml")
	if err := os.WriteFile(path, []byte("routing:\n  classes: standard,reasoning\n  default_class: standard\n  fallback: standard,reasoning\n  class_capabilities: standard=context;reasoning=context+eval\n  recommendations: low=standard,high=reasoning\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, diagnostics := LoadConfig(ConfigPaths{Project: path}, nil)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	task := TaskRow{ID: "T7", Complexity: "high"}
	a, err := RecommendRouting(task, cfg.Routing)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RecommendRouting(task, cfg.Routing)
	if err != nil || a != b || a.Class != "reasoning" {
		t.Fatalf("recommendations = %#v %#v, err=%v", a, b, err)
	}
	if a.Provider != "" || a.Model != "" {
		t.Fatalf("core selected provider/model: %#v", a)
	}
}

// TestEnvPolicy pins R7.1: closed environment policy loads per-environment
// strategy/approver/authority/criteria/window/freshness/rollback, and an unknown
// environment name or a missing/invalid required field fails closed.
func TestEnvPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yml")
	good := strings.Join([]string{
		"environments:",
		"  staging: strategy=rolling;criteria=health;window=5m;freshness=2m;rollback=previous",
		"  production: strategy=canary;approver=release-manager;authority=oncall;criteria=health+latency;window=10m;freshness=5m;rollback=previous",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, diags := LoadConfig(ConfigPaths{Project: path}, nil)
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %#v", diags)
	}
	prod, ok := cfg.Environments[EnvironmentProduction]
	if !ok {
		t.Fatal("production policy missing")
	}
	if prod.Schema != EnvironmentSchemaV1 || prod.Name != EnvironmentProduction {
		t.Fatalf("schema/name = %q/%q", prod.Schema, prod.Name)
	}
	if prod.Strategy != "canary" || prod.RequiredApprover != "release-manager" || prod.RequiredAuthority != "oncall" ||
		prod.ObservationWindow != "10m" || prod.Freshness != "5m" || prod.RollbackTarget != "previous" {
		t.Fatalf("production policy = %#v", prod)
	}
	if !reflect.DeepEqual(prod.HealthCriteria, []string{"health", "latency"}) {
		t.Fatalf("criteria = %#v", prod.HealthCriteria)
	}

	for _, raw := range []string{
		"environments:\n  qa: strategy=rolling;criteria=health;window=5m;freshness=2m;rollback=previous\n",         // unknown env
		"environments:\n  production: strategy=canary;criteria=health;window=10m;freshness=5m\n",                   // missing rollback
		"environments:\n  production: strategy=canary;criteria=health;window=nope;freshness=5m;rollback=prev\n",    // bad duration
		"environments:\n  production: strategy=canary;window=10m;freshness=5m;rollback=prev\n",                     // missing criteria
		"environments:\n  production: strategy=canary;criteria=health;window=10m;freshness=5m;rollback=prev;x=y\n", // unknown field
	} {
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, diags := LoadConfig(ConfigPaths{Project: path}, nil); len(diags) == 0 {
			t.Fatalf("policy %q accepted, want fail-closed diagnostic", raw)
		}
	}
}

func TestConfigRoutingSafeDefaultsAndValidation(t *testing.T) {
	if DefaultConfig.Routing.Version != "1" || !DefaultConfig.Routing.AllowUnknownTelemetry || DefaultConfig.Routing.DefaultClass == "" {
		t.Fatalf("unsafe routing defaults = %#v", DefaultConfig.Routing)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yml")
	for _, raw := range []string{
		"routing:\n  version: 2\n",
		"routing:\n  classes: basic,basic\n",
		"routing:\n  fallback: missing\n",
		"routing:\n  max_cost_micros: -1\n",
		"routing:\n  class_capabilities: malformed\n",
	} {
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
		_, diags := LoadConfig(ConfigPaths{Project: path}, nil)
		if len(diags) == 0 {
			t.Fatalf("config %q accepted, want diagnostic", raw)
		}
	}
}

// TestConfigProfile pins spec 01 R7: the default lifecycle profile keeps every
// completeness gate opt-in (R7.1); the production profile arms the criterion,
// review, and integration/negative-path gates together (R7.2); an unknown
// profile fails closed.
func TestConfigProfile(t *testing.T) {
	if DefaultConfig.Profile != ProfileDefault {
		t.Fatalf("default profile = %q, want %q", DefaultConfig.Profile, ProfileDefault)
	}
	if DefaultConfig.ProductionProfile() || DefaultConfig.CriteriaGateArmed() ||
		DefaultConfig.ReviewGateArmed() || DefaultConfig.IntegrationPolicyArmed() {
		t.Fatalf("default profile arms a gate: %#v", DefaultConfig)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "project.yml")
	if err := os.WriteFile(path, []byte("profile: production\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, diags := LoadConfig(ConfigPaths{Project: path}, nil)
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %#v", diags)
	}
	if cfg.Profile != ProfileProduction || !cfg.ProductionProfile() {
		t.Fatalf("profile = %q, want production", cfg.Profile)
	}
	if !cfg.CriteriaGateArmed() || !cfg.ReviewGateArmed() || !cfg.IntegrationPolicyArmed() {
		t.Fatalf("production profile did not arm all gates: criteria=%v review=%v integration=%v",
			cfg.CriteriaGateArmed(), cfg.ReviewGateArmed(), cfg.IntegrationPolicyArmed())
	}

	if err := os.WriteFile(path, []byte("profile: staging\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, diags = LoadConfig(ConfigPaths{Project: path}, nil); len(diags) != 1 ||
		!strings.Contains(diags[0].Message, "profile must be default|production") {
		t.Fatalf("invalid profile diagnostics = %#v, want one profile error", diags)
	}
}

// TestConfigProfileArmsPerSwitch pins R7.1: under the default profile each gate
// still arms from its own explicit switch, independently of the profile.
func TestConfigProfileArmsPerSwitch(t *testing.T) {
	cfg := DefaultConfig
	cfg.Criteria.Required = true
	if !cfg.CriteriaGateArmed() || cfg.ReviewGateArmed() || cfg.ProductionProfile() {
		t.Fatalf("criteria switch alone: %#v", cfg)
	}
	cfg = DefaultConfig
	cfg.Review.Required = true
	if !cfg.ReviewGateArmed() || cfg.CriteriaGateArmed() {
		t.Fatalf("review switch alone: %#v", cfg)
	}
}

// TestHandshakePolicyDigest pins R7.2 "policy digest shall pin judgment": the
// digest is stable for an unchanged policy and changes when the profile flips,
// so an approval pinned to it goes stale exactly when the policy moves.
func TestHandshakePolicyDigest(t *testing.T) {
	base := DefaultConfig
	baseDigest, again := PolicyDigest(base), PolicyDigest(base)
	if baseDigest != again {
		t.Fatal("policy digest not stable for identical config")
	}
	prod := DefaultConfig
	prod.Profile = ProfileProduction
	if PolicyDigest(base) == PolicyDigest(prod) {
		t.Fatal("policy digest unchanged when profile flipped to production")
	}
	hs := BootstrapHandshake(prod)
	if hs.PolicyDigest != PolicyDigest(prod) {
		t.Fatalf("handshake policy digest = %q, want %q", hs.PolicyDigest, PolicyDigest(prod))
	}
	if hs.PolicyDigest == PolicyDigest(base) {
		t.Fatal("handshake did not surface the production policy digest")
	}
}
