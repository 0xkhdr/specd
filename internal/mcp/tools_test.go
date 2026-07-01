package mcp

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	contextpkg "github.com/0xkhdr/specd/internal/context"
	"github.com/0xkhdr/specd/internal/core"
)

// updateGolden regenerates the schema golden instead of asserting against it:
//
//	go test ./internal/mcp/ -run Schema -update
var updateGolden = flag.Bool("update", false, "rewrite golden schema snapshot")

// TestToolsList asserts the generated tool list mirrors the command schema:
// one tool per non-meta command, correct namespacing, and read-only annotations
// matching the spec's R4 classification.
func TestHiddenExcluded(t *testing.T) {
	tools := toolNames(buildTools(nil))
	for _, name := range []string{
		"specd_doctor", "specd_migrate", "specd_mode", "specd_dispatch",
		"specd_validate", "specd_schema", "specd_serve", "specd_replay",
		"specd_diff", "specd_watch", "specd_program", "specd_update",
		"specd_uninstall", "specd_fusion", "specd_version", "specd_help", "specd_mcp",
	} {
		if tools[name] {
			t.Fatalf("hidden/retired command %s exposed in MCP default tool list", name)
		}
	}
}

func TestToolsList(t *testing.T) {
	tools := buildTools(nil)

	// Parity: one command-mirror tool per non-hidden, non-meta core.Commands entry
	// (R2) plus the intent-level tools (GAP-5). Adding a surviving command/intent
	// tool without it surfacing here fails the test.
	wantCount := len(intentTools)
	for _, c := range core.Commands {
		if !metaCommands[c.Command] && !c.Hidden {
			wantCount++
		}
	}
	if len(tools) != wantCount {
		t.Fatalf("tool count = %d, want %d (one per non-meta command + %d intent tools)", len(tools), wantCount, len(intentTools))
	}

	byName := map[string]toolDef{}
	for _, tl := range tools {
		byName[tl.Name] = tl
	}

	// Meta, hidden, and retired commands are never exposed.
	for _, name := range []string{"specd_help", "specd_version", "specd_mcp", "specd_doctor", "specd_dispatch", "specd_update", "specd_uninstall"} {
		if _, ok := byName[name]; ok {
			t.Errorf("meta/hidden command exposed as tool: %s", name)
		}
	}

	// Read-only annotation (R4).
	if tl := byName["specd_status"]; !tl.Annotations.ReadOnlyHint {
		t.Error("specd_status should be readOnlyHint:true")
	}
	if tl := byName["specd_verify"]; tl.Annotations.ReadOnlyHint {
		t.Error("specd_verify should be readOnlyHint:false")
	}

	// Each command-mirror tool carries an object input schema with an ordered
	// args array; intent tools model named properties instead (no args array).
	for _, tl := range tools {
		if tl.InputSchema.Type != "object" {
			t.Errorf("%s inputSchema.type = %q, want object", tl.Name, tl.InputSchema.Type)
		}
		if tl.intent {
			if _, ok := tl.InputSchema.Properties["args"]; ok {
				t.Errorf("intent tool %s should not expose a positional args array", tl.Name)
			}
			continue
		}
		if p, ok := tl.InputSchema.Properties["args"]; !ok || p.Type != "array" {
			t.Errorf("%s missing args array property", tl.Name)
		}
	}

	// Intent tools (GAP-5) are present with their semantic argument surface.
	orch := byName["brain_orchestrate"]
	if orch.Name == "" || !orch.intent {
		t.Fatal("brain_orchestrate intent tool missing")
	}
	for _, prop := range []string{"spec", "goal", "worker_cmd", "approval_policy", "max_steps"} {
		if _, ok := orch.InputSchema.Properties[prop]; !ok {
			t.Errorf("brain_orchestrate missing argument %q", prop)
		}
	}
	if _, ok := orch.InputSchema.Properties["args"]; ok {
		t.Error("brain_orchestrate must not carry a positional args array")
	}
	if !byName["brain_status"].Annotations.ReadOnlyHint {
		t.Error("brain_status should be readOnlyHint:true")
	}
	if byName["brain_orchestrate"].Annotations.ReadOnlyHint {
		t.Error("brain_orchestrate mutates state; readOnlyHint must be false")
	}
	for _, name := range []string{"brain_orchestrate", "brain_status", "brain_approve", "brain_pause", "brain_resume", "brain_cancel"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("intent tool %s not exposed", name)
		}
	}

	// Verify's --status flag is surfaced as a typed property.
	if tl, ok := byName["specd_verify"]; ok {
		if p, ok := tl.InputSchema.Properties["status"]; !ok || p.Type != "string" {
			t.Errorf("specd_verify missing string 'status' flag prop")
		}
	}

	brain := byName["specd_brain"]
	if brain.Annotations.ReadOnlyHint || brain.Annotations.DestructiveHint {
		t.Errorf("specd_brain annotations = %+v, want mutating non-destructive", brain.Annotations)
	}
	for _, flag := range []string{"program", "session", "approval-policy", "max-workers", "max-retries", "timeout-seconds", "cost-limit", "json"} {
		if _, ok := brain.InputSchema.Properties[flag]; !ok {
			t.Errorf("specd_brain missing orchestration flag %q", flag)
		}
	}
	pinky := byName["specd_pinky"]
	if pinky.Annotations.ReadOnlyHint || pinky.Annotations.DestructiveHint {
		t.Errorf("specd_pinky annotations = %+v, want mutating non-destructive", pinky.Annotations)
	}
	for _, flag := range []string{"mission", "session", "worker", "spec", "task", "attempt", "percent", "message", "reason", "verification-ref", "summary", "changed-files", "git-head", "duration-ms", "host-tokens", "host-cost", "json"} {
		if _, ok := pinky.InputSchema.Properties[flag]; !ok {
			t.Errorf("specd_pinky missing worker flag %q", flag)
		}
	}
}

// TestToolSchemaGolden snapshots every tool's name → input-schema mapping to a
// golden file (R2.3). Any change to a tool's input schema — a renamed flag, a
// changed type, an added/removed tool — diffs against the golden and fails,
// forcing the change to be deliberate. Regenerate with `-update` after an
// intentional schema change. The snapshot is keyed by tool name (stable, not
// help-display order) so reordering commands is not a spurious failure.
func TestToolSchemaGolden(t *testing.T) {
	schemas := map[string]jsonSchema{}
	for _, tl := range buildTools(nil) {
		schemas[tl.Name] = tl.InputSchema
	}
	got, err := json.MarshalIndent(schemas, "", "  ")
	if err != nil {
		t.Fatalf("marshal schemas: %v", err)
	}
	got = append(got, '\n')

	golden := filepath.Join("testdata", "tool_schemas.golden.json")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s (%d tools)", golden, len(schemas))
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("tool input schemas drifted from golden.\n"+
			"If intentional, regenerate: go test ./internal/mcp/ -run Schema -update\n"+
			"--- got ---\n%s", got)
	}
}

// TestToolSchemaValidity asserts each generated tool input schema is a
// structurally valid JSON-Schema object (R2.1): an object type, a non-nil
// properties map, an ordered string-array `args`, and a concrete type on every
// flag property. A host must be able to consume every advertised schema.
func TestToolSchemaValidity(t *testing.T) {
	for _, tl := range buildTools(nil) {
		if tl.Name == "" {
			t.Error("tool with empty name")
		}
		s := tl.InputSchema
		if s.AdditionalProperties {
			t.Errorf("%s: additionalProperties = true, want false", tl.Name)
		}
		if s.Type != "object" {
			t.Errorf("%s: inputSchema.type = %q, want object", tl.Name, s.Type)
		}
		if s.Properties == nil {
			t.Errorf("%s: nil properties", tl.Name)
			continue
		}
		if !tl.intent {
			args, ok := s.Properties["args"]
			if !ok || args.Type != "array" || args.Items == nil || args.Items.Type != "string" {
				t.Errorf("%s: args must be an array of strings, got %+v", tl.Name, args)
			}
		}
		for name, p := range s.Properties {
			if p.Type != "string" && p.Type != "boolean" && p.Type != "array" {
				t.Errorf("%s: property %q has unsupported type %q", tl.Name, name, p.Type)
			}
		}
	}
}

// --- C1: context-manifest tool filtering -----------------------------------

func TestApplyManifestFilterRequiredOptional(t *testing.T) {
	// AC1: required+optional define the allowlist; everything else is dropped.
	cand := []toolDef{td("specd_inspect"), td("specd_verify"), td("specd_task"), td("specd_status")}
	m := core.ContextManifestTools{
		RequiredTools: []string{"specd_inspect", "specd_verify"},
		OptionalTools: []string{"specd_task"},
	}
	got := names(applyManifestFilter(cand, m))
	want := []string{"specd_inspect", "specd_verify", "specd_task"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("allowlist = %v, want %v", got, want)
	}
}

func TestApplyManifestForbiddenWins(t *testing.T) {
	// AC2/R3: forbidden excludes a tool even when required/optional would allow it.
	cand := []toolDef{td("specd_inspect"), td("specd_approve")}
	m := core.ContextManifestTools{
		RequiredTools:  []string{"specd_inspect", "specd_approve"},
		ForbiddenTools: []string{"specd_approve"},
	}
	got := names(applyManifestFilter(cand, m))
	if strings.Join(got, ",") != "specd_inspect" {
		t.Fatalf("forbidden not excluded: %v", got)
	}
}

func TestApplyManifestNoManifestUnchanged(t *testing.T) {
	// AC4: an empty manifest is a no-op.
	cand := []toolDef{td("specd_status"), td("specd_verify")}
	got := applyManifestFilter(cand, core.ContextManifestTools{})
	if len(got) != len(cand) {
		t.Fatalf("empty manifest altered list: %v", names(got))
	}
}

func TestApplyManifestRequiredGatedOffDiagnostic(t *testing.T) {
	// AC3/R4: a required tool missing from the (config-gated) candidate set stays
	// excluded and emits a diagnostic — config safety wins over manifest required.
	cand := []toolDef{td("specd_status")}
	m := core.ContextManifestTools{RequiredTools: []string{"specd_status", "specd_brain"}}
	var got []toolDef
	diag := captureStderr(t, func() { got = applyManifestFilter(cand, m) })
	if strings.Join(names(got), ",") != "specd_status" {
		t.Fatalf("gated required leaked into list: %v", names(got))
	}
	if !strings.Contains(diag, "specd_brain") || !strings.Contains(diag, "config gate") {
		t.Fatalf("missing R4 diagnostic, got: %q", diag)
	}
}

func TestApplyManifestUnknownNameIgnored(t *testing.T) {
	cand := []toolDef{td("specd_status")}
	m := core.ContextManifestTools{RequiredTools: []string{"specd_status", "specd_bogus"}}
	var got []toolDef
	diag := captureStderr(t, func() { got = applyManifestFilter(cand, m) })
	if strings.Join(names(got), ",") != "specd_status" {
		t.Fatalf("unknown name affected output: %v", names(got))
	}
	if !strings.Contains(diag, "specd_bogus") || !strings.Contains(diag, "unknown tool") {
		t.Fatalf("missing unknown-name warning, got: %q", diag)
	}
}

// --- C2: host capability negotiation ---------------------------------------

func TestApplyHostPrefsMaxToolsCap(t *testing.T) {
	// AC1: maxTools caps the emitted count.
	cand := []toolDef{td("a"), td("b"), td("c"), td("d"), td("e"), td("f")}
	got := applyHostPrefs(cand, hostPrefs{maxTools: 5}, nil)
	if len(got) != 5 {
		t.Fatalf("maxTools=5 emitted %d tools", len(got))
	}
}

func TestApplyHostPrefsPreferredNamespaceOrder(t *testing.T) {
	// AC2: a preferred namespace (named by a member tool) orders its tools first,
	// preserving relative order within and outside the bucket.
	cand := []toolDef{td("specd_status"), td("specd_inspect"), td("specd_verify"), td("specd_read")}
	got := names(applyHostPrefs(cand, hostPrefs{preferredNamespaces: []string{"specd_read"}}, nil))
	want := []string{"specd_inspect", "specd_read", "specd_status", "specd_verify"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("namespace order = %v, want %v", got, want)
	}
}

func TestApplyHostPrefsRequiredSurvivesCap(t *testing.T) {
	// AC3/R4: required tools beyond maxTools are still all emitted, with a stderr
	// diagnostic — safety gates win over the cap.
	cand := []toolDef{td("specd_inspect"), td("specd_verify"), td("specd_task"), td("specd_status")}
	required := map[string]bool{"specd_inspect": true, "specd_verify": true, "specd_task": true}
	var got []toolDef
	diag := captureStderr(t, func() {
		got = applyHostPrefs(cand, hostPrefs{maxTools: 2}, required)
	})
	gm := toolNames(got)
	for n := range required {
		if !gm[n] {
			t.Fatalf("required %s dropped by cap: %v", n, names(got))
		}
	}
	if !strings.Contains(diag, "maxTools=2") {
		t.Fatalf("missing over-cap diagnostic, got: %q", diag)
	}
}

func TestApplyHostPrefsNoHintsNoop(t *testing.T) {
	// AC4: no hints ⇒ identical slice (order + length).
	cand := []toolDef{td("a"), td("b"), td("c")}
	got := applyHostPrefs(cand, hostPrefs{}, nil)
	if strings.Join(names(got), ",") != "a,b,c" {
		t.Fatalf("no-hint path altered list: %v", names(got))
	}
}

func TestApplyHostPrefsGarbageIsSafe(t *testing.T) {
	// AC5: a negative maxTools (clamped at parse) and unknown namespaces no-op.
	cand := []toolDef{td("specd_status"), td("specd_inspect")}
	hp := parseHostPrefs([]byte(`{"capabilities":{"specd":{"maxTools":-3,"preferredNamespaces":["nope"]}}}`))
	if hp.maxTools != 0 {
		t.Fatalf("negative maxTools not clamped: %+v", hp)
	}
	// The unknown namespace is dropped at apply time, leaving the list untouched.
	got := applyHostPrefs(cand, hp, nil)
	if strings.Join(names(got), ",") != "specd_status,specd_inspect" {
		t.Fatalf("garbage altered list: %v", names(got))
	}
}

func TestParseHostPrefs(t *testing.T) {
	hp := parseHostPrefs([]byte(`{"capabilities":{"specd":{"maxTools":5,"preferredNamespaces":["read"]}}}`))
	if hp.maxTools != 5 || len(hp.preferredNamespaces) != 1 || hp.preferredNamespaces[0] != "read" {
		t.Fatalf("parsed prefs wrong: %+v", hp)
	}
}

// TestNegotiateMaxContextTokens proves capabilities.specd.maxContextTokens is
// parsed and persisted, and that it does not perturb tool-list negotiation
// (active() stays driven by maxTools/namespaces only). AC-6.
func TestNegotiateMaxContextTokens(t *testing.T) {
	hp := parseHostPrefs([]byte(`{"capabilities":{"specd":{"maxContextTokens":8000}}}`))
	if hp.maxContextTokens != 8000 {
		t.Fatalf("maxContextTokens not parsed: %+v", hp)
	}
	if hp.active() {
		t.Fatalf("context budget alone must not activate tool-list shaping: %+v", hp)
	}
}

// TestNegotiateMaxContextTokensGarbageIsSafe proves a non-positive or malformed
// hint clamps to 0 (ignored) instead of capping briefs to nothing, and never
// tears down the handshake. AC-6 / R6.
func TestNegotiateMaxContextTokensGarbageIsSafe(t *testing.T) {
	for _, raw := range []string{
		`{"capabilities":{"specd":{"maxContextTokens":-4000}}}`,
		`{"capabilities":{"specd":{"maxContextTokens":"lots"}}}`,
		`not json at all`,
	} {
		if hp := parseHostPrefs([]byte(raw)); hp.maxContextTokens != 0 {
			t.Fatalf("garbage budget %q not clamped: %+v", raw, hp)
		}
	}
}

// TestCapabilityContextBudgetCapsManifest proves the negotiated budget threads
// across the in-process dispatch boundary (setContextBudgetEnv ->
// SPECD_MAX_CONTEXT_TOKENS -> core.HostContextBudgetFromEnv) and caps the
// effective manifest Budget; omitting it leaves the engine default untouched
// (byte-identical path). AC-6.
func TestCapabilityContextBudgetCapsManifest(t *testing.T) {
	req := contextpkg.ContextRequest{Slug: "demo", Status: core.StatusExecuting, Role: "craftsman", Mode: contextpkg.ContextModeBriefing}
	baseline := contextpkg.BuildContextManifest(req).Budget

	restore := setContextBudgetEnv(2500)
	defer restore()
	if got := core.HostContextBudgetFromEnv(); got != 2500 {
		t.Fatalf("budget did not cross the env boundary: got %d", got)
	}
	req.HostBudget = core.HostContextBudgetFromEnv()
	if capped := contextpkg.BuildContextManifest(req).Budget; capped != 2500 {
		t.Fatalf("budget not capped to host hint: got %d (baseline %d)", capped, baseline)
	}

	restore()
	if got := core.HostContextBudgetFromEnv(); got != 0 {
		t.Fatalf("env not restored after tool call: got %d", got)
	}
}

// boolPtr is a tiny helper so table rows can set IncludeOrchestration to an
// explicit true/false distinct from the unset (nil) state (spec R5a).
func boolPtr(b bool) *bool { return &b }

func toolNames(tools []toolDef) map[string]bool {
	names := make(map[string]bool, len(tools))
	for _, tl := range tools {
		names[tl.Name] = true
	}
	return names
}

// TestResolveMCPExposure table-tests the pure allow-policy across the config
// matrix (spec §7). It asserts the resolved exposurePlan directly so the gating
// semantics are pinned independently of the buildTools loop.
func TestResolveMCPExposure(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *core.Config
		wantPass bool
		wantEss  bool
		wantMeta bool
		wantOrch bool
	}{
		{
			name:     "nil cfg is passthrough",
			cfg:      nil,
			wantPass: true,
		},
		{
			name:     "absent mcp block is passthrough",
			cfg:      &core.Config{},
			wantPass: true,
		},
		{
			name:     "expose all engages gating with default-off meta/orch",
			cfg:      &core.Config{MCP: core.MCPConfig{Expose: "all"}},
			wantPass: false,
		},
		{
			name:     "includeMeta true opens meta gate",
			cfg:      &core.Config{MCP: core.MCPConfig{Expose: "all", IncludeMeta: true}},
			wantMeta: true,
		},
		{
			name:     "includeOrchestration explicit true overrides disabled",
			cfg:      &core.Config{MCP: core.MCPConfig{Expose: "all", IncludeOrchestration: boolPtr(true)}},
			wantOrch: true,
		},
		{
			name: "unset includeOrchestration derives from enabled",
			cfg: &core.Config{
				Orchestration: core.OrchestrationCfg{Enabled: true},
				MCP:           core.MCPConfig{Expose: "all"},
			},
			wantOrch: true,
		},
		{
			name:    "essential mode",
			cfg:     &core.Config{MCP: core.MCPConfig{Expose: "essential"}},
			wantEss: true,
		},
		{
			name:     "unknown mode degrades to all",
			cfg:      &core.Config{MCP: core.MCPConfig{Expose: "bogus"}},
			wantPass: false,
			wantEss:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := resolveMCPExposure(tt.cfg)
			if plan.passthrough != tt.wantPass {
				t.Errorf("passthrough = %v, want %v", plan.passthrough, tt.wantPass)
			}
			if plan.essential != tt.wantEss {
				t.Errorf("essential = %v, want %v", plan.essential, tt.wantEss)
			}
			if plan.includeMeta != tt.wantMeta {
				t.Errorf("includeMeta = %v, want %v", plan.includeMeta, tt.wantMeta)
			}
			if plan.includeOrchestration != tt.wantOrch {
				t.Errorf("includeOrchestration = %v, want %v", plan.includeOrchestration, tt.wantOrch)
			}
		})
	}
}

// TestEssentialDefaultSet covers AC2/R3a: expose:"essential" with no explicit
// set yields exactly the default tools. The default set mixes composite tools
// (already namespaced, e.g. specd_inspect) with atomic command mirrors (bare
// command names, prefixed with specd_), so each entry is resolved by shape.
func TestEssentialDefaultSet(t *testing.T) {
	cfg := &core.Config{MCP: core.MCPConfig{Expose: "essential"}}
	tools := buildTools(cfg)
	if len(tools) != len(defaultEssentialTools) {
		t.Fatalf("essential tool count = %d, want %d", len(tools), len(defaultEssentialTools))
	}
	names := toolNames(tools)
	for _, entry := range defaultEssentialTools {
		want := entry
		if !strings.HasPrefix(entry, toolPrefix) {
			want = toolPrefix + entry
		}
		if !names[want] {
			t.Errorf("essential set missing %s", want)
		}
	}
	// Composite read tools must be in the default essential surface (AC7).
	for _, c := range []string{"specd_inspect", "specd_read", "specd_query"} {
		if !names[c] {
			t.Errorf("essential set missing read tool %s", c)
		}
	}
}

// TestEssentialExplicitSet covers R3: only the named commands/intents survive.
func TestEssentialExplicitSet(t *testing.T) {
	cfg := &core.Config{
		Orchestration: core.OrchestrationCfg{Enabled: true},
		MCP: core.MCPConfig{
			Expose:         "essential",
			EssentialTools: []string{"status", "brain_orchestrate"},
		},
	}
	names := toolNames(buildTools(cfg))
	if len(names) != 2 {
		t.Fatalf("got %d tools, want 2: %v", len(names), names)
	}
	if !names["specd_status"] || !names["brain_orchestrate"] {
		t.Errorf("explicit essential set = %v, want specd_status + brain_orchestrate", names)
	}
}

// TestIncludeMetaGate covers AC3/R4: retired hidden tools stay absent even when
// includeMeta is true; the optimized MCP surface has no default meta-risk leaks.
func TestIncludeMetaGate(t *testing.T) {
	hidden := toolNames(buildTools(&core.Config{MCP: core.MCPConfig{Expose: "all"}}))
	shown := toolNames(buildTools(&core.Config{MCP: core.MCPConfig{Expose: "all", IncludeMeta: true}}))
	for cmd := range metaRiskCommands {
		if hidden[toolPrefix+cmd] || shown[toolPrefix+cmd] {
			t.Errorf("retired meta-risk tool %s%s exposed (hidden=%v shown=%v)", toolPrefix, cmd, hidden[toolPrefix+cmd], shown[toolPrefix+cmd])
		}
	}
}

// TestOrchestrationGate covers AC4/AC5/R5/R5a: with orchestration disabled the
// brain/pinky commands and every brain_* intent vanish; enabling brings them back.
func TestOrchestrationGate(t *testing.T) {
	off := toolNames(buildTools(&core.Config{MCP: core.MCPConfig{Expose: "all"}}))
	for cmd := range orchestrationCommands {
		if off[toolPrefix+cmd] {
			t.Errorf("%s%s exposed while orchestration excluded", toolPrefix, cmd)
		}
	}
	for _, it := range intentTools {
		if off[it.name] {
			t.Errorf("intent %s exposed while orchestration excluded", it.name)
		}
	}

	on := toolNames(buildTools(&core.Config{
		Orchestration: core.OrchestrationCfg{Enabled: true},
		MCP:           core.MCPConfig{Expose: "all"},
	}))
	if !on["specd_brain"] || !on["specd_pinky"] {
		t.Error("orchestration commands missing when enabled + expose:all")
	}
	for _, it := range intentTools {
		if !on[it.name] {
			t.Errorf("intent %s missing when orchestration enabled", it.name)
		}
	}

	offExplicit := toolNames(buildTools(&core.Config{
		Orchestration: core.OrchestrationCfg{Enabled: true},
		MCP:           core.MCPConfig{Expose: "all", IncludeOrchestration: boolPtr(false)},
	}))
	if offExplicit["specd_brain"] || offExplicit["specd_pinky"] || offExplicit["brain_orchestrate"] {
		t.Errorf("orchestration tools exposed with includeOrchestration=false: %v", offExplicit)
	}
}

func TestPhasePlanningExcludesExecutionMutations(t *testing.T) {
	cfg := &core.Config{MCP: core.MCPConfig{Expose: "phase"}}
	got := toolNames(buildPhaseTools(cfg, core.StatusDesign, ""))
	for _, name := range []string{"specd_next", "specd_verify", "specd_task", "specd_brain", "specd_pinky"} {
		if got[name] {
			t.Errorf("planning phase exposed incompatible tool %s: %v", name, got)
		}
	}
	for _, name := range []string{"specd_check", "specd_approve", "specd_context", "specd_status"} {
		if !got[name] {
			t.Errorf("planning phase missing expected tool %s: %v", name, got)
		}
	}
}

func TestPhaseExecutingIncludesDriveLoopTools(t *testing.T) {
	cfg := &core.Config{MCP: core.MCPConfig{Expose: "phase"}}
	got := toolNames(buildPhaseTools(cfg, core.StatusExecuting, ""))
	for _, name := range []string{"specd_next", "specd_verify", "specd_task"} {
		if !got[name] {
			t.Errorf("executing phase missing drive-loop tool %s: %v", name, got)
		}
	}
}

// TestPassthroughEqualsToday covers R1/AC1: an absent mcp block produces the
// exact same tool list (names and order) as the pre-config nil build.
func TestPassthroughEqualsToday(t *testing.T) {
	nilBuild := buildTools(nil)
	zeroBuild := buildTools(&core.Config{})
	if len(nilBuild) != len(zeroBuild) {
		t.Fatalf("zero-config count = %d, nil count = %d", len(zeroBuild), len(nilBuild))
	}
	for i := range nilBuild {
		if nilBuild[i].Name != zeroBuild[i].Name {
			t.Errorf("order/name drift at %d: nil=%s zero=%s", i, nilBuild[i].Name, zeroBuild[i].Name)
		}
	}
}

func TestBuildToolsRoleFilterFromActiveSpec(t *testing.T) {
	root := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.MkdirAll(filepath.Join(root, ".specd", "specs", "auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	st := core.InitialState("auth", "Auth")
	st.Status = core.StatusVerifying
	st.Phase = core.PhaseForStatus(st.Status)
	if err := core.SaveState(root, "auth", &st); err != nil {
		t.Fatal(err)
	}

	got := toolNames(buildTools(&core.Config{MCP: core.MCPConfig{Expose: "all"}}))
	want := []string{"specd_check", "specd_status", "specd_state_read"}
	if len(got) != len(want) {
		t.Fatalf("validator tool count = %d, want %d: %v", len(got), len(want), got)
	}
	for _, name := range want {
		if !got[name] {
			t.Fatalf("validator tool list missing %s: %v", name, got)
		}
	}
}

// listToolsOverServer runs the real stdio Serve loop with cfg and returns the
// advertised tool names from tools/list, exercising the full route → buildTools
// plumbing rather than buildTools in isolation (spec §7 integration).
func listToolsOverServer(t *testing.T, cfg *core.Config) []string {
	t.Helper()
	noop := func(string, cli.Args) (int, bool) { return core.ExitOK, true }
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	}, "\n") + "\n"

	var out strings.Builder
	if err := Serve(strings.NewReader(input), &out, noop, cfg); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var resp struct {
			ID     int `json:"id"`
			Result struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("bad response %q: %v", line, err)
		}
		if resp.ID == 2 {
			for _, tl := range resp.Result.Tools {
				names = append(names, tl.Name)
			}
		}
	}
	return names
}

// TestServerToolsListFiltered asserts tools/list honours each config variant
// end-to-end over the wire (spec AC1–AC5).
func TestServerToolsListFiltered(t *testing.T) {
	t.Run("absent_block_matches_full_set", func(t *testing.T) {
		got := len(listToolsOverServer(t, &core.Config{}))
		want := len(buildTools(nil))
		if got != want {
			t.Errorf("absent-block tool count = %d, want %d", got, want)
		}
	})

	t.Run("essential_default_set_includes_bootstrap", func(t *testing.T) {
		got := listToolsOverServer(t, &core.Config{MCP: core.MCPConfig{Expose: "essential"}})
		if len(got) != len(defaultEssentialTools) {
			t.Errorf("essential count = %d, want %d (%v)", len(got), len(defaultEssentialTools), got)
		}
	})

	t.Run("orchestration_enabled_exposes_brain_surface", func(t *testing.T) {
		got := listToolsOverServer(t, &core.Config{
			Orchestration: core.OrchestrationCfg{Enabled: true},
			MCP:           core.MCPConfig{Expose: "all"},
		})
		has := map[string]bool{}
		for _, n := range got {
			has[n] = true
		}
		if !has["specd_brain"] || !has["brain_orchestrate"] {
			t.Errorf("orchestration tools missing over wire: %v", got)
		}
	})
}
