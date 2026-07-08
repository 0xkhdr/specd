package core

import (
	"strings"
	"testing"
)

// TestRenderConfigYAML exercises the canonical YAML renderer across its optional
// branches (resilience block, MCP block with essential tools + orchestration
// override, compaction knobs) so the migration output path stays covered.
func TestRenderConfigYAML(t *testing.T) {
	cfg := DefaultConfig
	out := RenderConfigYAML(cfg)
	if !strings.Contains(out, "# specd configuration (YAML v2)") {
		t.Fatalf("missing header:\n%s", out)
	}
	for _, want := range []string{"defaults:", "report:", "roles:", "gates:", "verify:", "orchestration:", "  transport:", "  program:"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered YAML missing %q", want)
		}
	}

	// Populate optional blocks so the resilience + MCP + compaction branches render.
	incOrch := true
	cfg.Orchestration.CompactionPolicy = CompactionBoth
	cfg.Orchestration.CompactionBudgetThreshold = 0.8
	cfg.Orchestration.Resilience = &ResilienceCfg{
		CheckpointEnabled:      true,
		MaxSuspendSeconds:      120,
		ContextSnapshotEnabled: true,
		ProgressTimeoutSeconds: 60,
		AutoResume:             AutoResumeCfg{Enabled: true, OnHostStart: true, MaxAgeMinutes: 30},
	}
	cfg.MCP.Expose = "essential"
	cfg.MCP.IncludeMeta = true
	cfg.MCP.IncludeOrchestration = &incOrch
	cfg.MCP.EssentialTools = []string{"specd_next", "specd_check"}

	out = RenderConfigYAML(cfg)
	for _, want := range []string{
		"  resilience:",
		"    auto_resume:",
		"compaction_policy:",
		"compaction_budget_threshold:",
		"mcp:",
		"  essential_tools: [",
		"specd_next",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered YAML missing optional %q\n%s", want, out)
		}
	}

	// Rendering is deterministic.
	if RenderConfigYAML(cfg) != out {
		t.Error("RenderConfigYAML not deterministic")
	}
}
