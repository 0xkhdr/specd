package core

import (
	"encoding/json"
	"strings"
	"testing"
)

// fakeArtifacts builds a reader closure over an in-memory artifact map so the
// engine stays pure in tests (no temp files, no IO).
func fakeArtifacts(m map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := m[name]
		return v, ok
	}
}

func kindsOf(manifest MissionContextManifest) []string {
	out := make([]string, len(manifest.Items))
	for i, item := range manifest.Items {
		out[i] = item.Kind
	}
	return out
}

func itemByPathSuffix(manifest MissionContextManifest, suffix string) (MissionContextItem, bool) {
	for _, item := range manifest.Items {
		if strings.HasSuffix(item.Path, suffix) {
			return item, true
		}
	}
	return MissionContextItem{}, false
}

func TestContextManifestOrderingAndKinds(t *testing.T) {
	req := ContextRequest{
		Slug:           "demo",
		Status:         StatusExecuting,
		TaskID:         "T1",
		Role:           "builder",
		Files:          []string{"internal/core/demo.go"},
		Mode:           ContextModeMission,
		ContextCommand: "specd context demo",
		Requirements:   []int{1},
	}
	m := BuildContextManifest(req)

	wantKinds := []string{"role", "skill", "phase-skill", "spec-context", "scope-file", "source-artifact", "source-artifact", "source-artifact"}
	got := kindsOf(m)
	if strings.Join(got, ",") != strings.Join(wantKinds, ",") {
		t.Fatalf("kinds = %v, want %v", got, wantKinds)
	}
	for i, item := range m.Items {
		if item.Order != i+1 {
			t.Fatalf("order not contiguous at %d: %d", i, item.Order)
		}
	}
	if m.Version != missionContextManifestVersion || m.SoftTokenCeiling != missionContextSoftCeiling {
		t.Fatalf("unexpected version/ceiling: %+v", m)
	}
}

func TestContextManifestMeasuredHints(t *testing.T) {
	big := strings.Repeat("design body line\n", 500)
	req := ContextRequest{
		Slug:         "demo",
		Status:       StatusDesign,
		TaskID:       "A2",
		Role:         "builder",
		Mode:         ContextModeBriefing,
		ReadArtifact: fakeArtifacts(map[string]string{"design.md": big}),
	}
	m := BuildContextManifest(req)
	item, ok := itemByPathSuffix(m, "specs/demo/design.md")
	if !ok {
		t.Fatal("design.md source-artifact missing")
	}
	want := EstimateTokensString(big)
	if item.TokenHint != want {
		t.Fatalf("design hint = %d, want measured %d (not a constant)", item.TokenHint, want)
	}
	if item.TokenHint == ctxHintArtifact {
		t.Fatalf("hint still equals the hardcoded constant %d", ctxHintArtifact)
	}
}

func TestContextManifestTargetedRequirementSlice(t *testing.T) {
	req := `# Requirements

## Requirement 1
- R1 line.

## Requirement 2
- R2 line, must not leak.

## Requirement 3
- R3 line.
`
	r := ContextRequest{
		Slug:         "demo",
		Status:       StatusExecuting,
		TaskID:       "T1",
		Role:         "builder",
		Requirements: []int{1, 3},
		ReadArtifact: fakeArtifacts(map[string]string{"requirements.md": req}),
	}
	m := BuildContextManifest(r)
	item, ok := itemByPathSuffix(m, "requirements.md")
	if !ok {
		t.Fatal("requirements.md missing")
	}
	if item.Mode != "read-targeted" {
		t.Fatalf("mode = %q, want read-targeted", item.Mode)
	}
	// The slice's measured hint should be smaller than the whole-file estimate.
	if item.TokenHint >= EstimateTokensString(req) {
		t.Fatalf("targeted hint %d not smaller than whole-file %d", item.TokenHint, EstimateTokensString(req))
	}
}

func TestContextManifestSourceArtifactsPhaseFiltered(t *testing.T) {
	cases := map[SpecStatus][]string{
		StatusRequirements: {"requirements.md"},
		StatusDesign:       {"requirements.md", "design.md"},
		StatusVerifying:    {"requirements.md", "tasks.md"},
		StatusComplete:     {"tasks.md"},
	}
	for status, want := range cases {
		m := BuildContextManifest(ContextRequest{Slug: "demo", Status: status, Role: "builder"})
		var got []string
		for _, item := range m.Items {
			if item.Kind == "source-artifact" {
				got = append(got, item.Path[strings.LastIndex(item.Path, "/")+1:])
			}
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("status %s source artifacts = %v, want %v", status, got, want)
		}
	}
}

func TestContextManifestBudgetHostCap(t *testing.T) {
	base := BuildContextManifest(ContextRequest{Slug: "demo", Status: StatusExecuting, Role: "builder", Files: []string{"a.go", "b.go"}})
	if base.Budget < minMissionContextSoftCeiling || base.Budget > maxMissionContextSoftCeiling {
		t.Fatalf("budget out of bounds: %d", base.Budget)
	}
	capped := BuildContextManifest(ContextRequest{Slug: "demo", Status: StatusExecuting, Role: "builder", Files: []string{"a.go", "b.go"}, HostBudget: 2000})
	if capped.Budget != 2000 {
		t.Fatalf("host budget not honored: got %d want 2000", capped.Budget)
	}
	// A garbage-ish tiny host budget is clamped to the minimum, not below.
	floor := BuildContextManifest(ContextRequest{Slug: "demo", Status: StatusExecuting, Role: "builder", HostBudget: 1})
	if floor.Budget != minMissionContextSoftCeiling {
		t.Fatalf("budget floor = %d, want %d", floor.Budget, minMissionContextSoftCeiling)
	}
}

// TestContextManifestSurfaceParity proves the single-engine invariant (AC-1):
// the same task built for Surface A (briefing), B (dispatch), and C (mission)
// references byte-identical items/kinds/order, and a small HostBudget caps the
// derived Budget identically across all three. Surfaces differ only in how they
// populate the ContextRequest, never in the engine output for shared inputs.
func TestContextManifestSurfaceParity(t *testing.T) {
	reader := fakeArtifacts(map[string]string{
		"tasks.md":        "## Wave 1\n\n- [ ] T1 — build the thing\n  - files: x.go\n\n- [ ] T2 — other\n",
		"requirements.md": "- R1: the system SHALL do X.\n- R2: the system SHALL do Y.\n",
	})
	mk := func(mode ContextMode) ContextRequest {
		return ContextRequest{
			Slug:           "demo",
			Status:         StatusExecuting,
			TaskID:         "T1",
			Role:           "builder",
			Files:          []string{"x.go"},
			Mode:           mode,
			ContextCommand: "specd context demo",
			Requirements:   []int{1},
			HostBudget:     3000,
			ReadArtifact:   reader,
		}
	}
	a := BuildContextManifest(mk(ContextModeBriefing))
	b := BuildContextManifest(mk(ContextModeDispatch))
	c := BuildContextManifest(mk(ContextModeMission))

	itemsJSON := func(m MissionContextManifest) string {
		raw, err := json.Marshal(m.Items)
		if err != nil {
			t.Fatalf("marshal items: %v", err)
		}
		return string(raw)
	}
	wantItems := itemsJSON(a)
	if itemsJSON(b) != wantItems {
		t.Fatalf("dispatch items diverge from briefing:\n A=%s\n B=%s", wantItems, itemsJSON(b))
	}
	if itemsJSON(c) != wantItems {
		t.Fatalf("mission items diverge from briefing:\n A=%s\n C=%s", wantItems, itemsJSON(c))
	}
	for _, m := range []MissionContextManifest{a, b, c} {
		if m.Budget != 3000 {
			t.Fatalf("host budget not honored across surfaces: got %d want 3000", m.Budget)
		}
		if m.Version != missionContextManifestVersion {
			t.Fatalf("version drifted: %d", m.Version)
		}
	}
}

func TestContextManifestEstimatedTokensSumsRequired(t *testing.T) {
	m := BuildContextManifest(ContextRequest{Slug: "demo", Status: StatusExecuting, TaskID: "T1", Role: "builder", Files: []string{"x.go"}, ContextCommand: "specd context demo"})
	sum := 0
	for _, item := range m.Items {
		if item.Required {
			sum += item.TokenHint
		}
	}
	if m.EstimatedTokens != sum {
		t.Fatalf("estimatedTokens = %d, want sum of required %d", m.EstimatedTokens, sum)
	}
}

// TestContextManifestNoReaderBackCompat proves AC-7: a reader-less request keeps
// default hints and whole-file reference modes for source artifacts.
func TestContextManifestNoReaderBackCompat(t *testing.T) {
	m := BuildContextManifest(ContextRequest{Slug: "demo", Status: StatusExecuting, TaskID: "T1", Role: "builder", Files: []string{"x.go"}, ContextCommand: "specd context demo"})
	for _, item := range m.Items {
		if item.Kind == "source-artifact" {
			if item.Mode != "reference-if-needed" || item.TokenHint != ctxHintArtifact {
				t.Fatalf("no-reader source artifact changed: %+v", item)
			}
		}
	}
}

func TestContextManifestValidates(t *testing.T) {
	m := BuildContextManifest(ContextRequest{
		Slug:           "demo",
		Status:         StatusExecuting,
		TaskID:         "T1",
		Role:           "builder",
		Files:          []string{"x.go"},
		ContextCommand: "specd context demo",
		ReadArtifact:   fakeArtifacts(map[string]string{"tasks.md": "## Wave 1\n\n- [ ] T1 — Demo\n  - role: builder\n"}),
	})
	if err := validateMissionContextManifest(m, true); err != nil {
		t.Fatalf("engine output failed validation: %v", err)
	}
	// Round-trips through JSON unchanged (additive fields present, version 1).
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var back MissionContextManifest
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Version != 1 {
		t.Fatalf("version drifted: %d", back.Version)
	}
}
