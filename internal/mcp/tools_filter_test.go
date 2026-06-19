package mcp

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

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
	// The composite read tools must be in the default essential surface (AC7).
	for _, c := range []string{"specd_inspect", "specd_read", "specd_query"} {
		if !names[c] {
			t.Errorf("essential set missing composite %s", c)
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

// TestIncludeMetaGate covers AC3/R4: meta-risk tools hidden by default, shown
// only when includeMeta is true.
func TestIncludeMetaGate(t *testing.T) {
	hidden := toolNames(buildTools(&core.Config{MCP: core.MCPConfig{Expose: "all"}}))
	for cmd := range metaRiskCommands {
		if hidden[toolPrefix+cmd] {
			t.Errorf("%s%s exposed with includeMeta:false", toolPrefix, cmd)
		}
	}
	shown := toolNames(buildTools(&core.Config{MCP: core.MCPConfig{Expose: "all", IncludeMeta: true}}))
	for cmd := range metaRiskCommands {
		if !shown[toolPrefix+cmd] {
			t.Errorf("%s%s missing with includeMeta:true", toolPrefix, cmd)
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
