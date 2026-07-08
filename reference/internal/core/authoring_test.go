package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAuthoringBrief(t *testing.T) {
	b := NewAuthoringBrief("")

	// Sourced from the gate vars, not re-listed literals.
	if len(b.EarsForms) != len(EarsForms()) || len(b.EarsForms) == 0 {
		t.Fatalf("EarsForms = %v", b.EarsForms)
	}
	if strings.Join(b.DesignSections, "|") != strings.Join(DesignSections, "|") {
		t.Errorf("DesignSections = %v, want %v", b.DesignSections, DesignSections)
	}
	if strings.Join(b.TaskKeys, "|") != strings.Join(MandatoryKeys, "|") {
		t.Errorf("TaskKeys = %v, want %v", b.TaskKeys, MandatoryKeys)
	}
	if strings.Join(b.Roles, "|") != strings.Join(ValidRoles, "|") {
		t.Errorf("Roles = %v, want %v", b.Roles, ValidRoles)
	}

	// One artifact brief per canonical file.
	arts := map[string]bool{}
	for _, a := range b.Artifacts {
		arts[a.Artifact] = true
		if len(a.Constraints) == 0 {
			t.Errorf("%s: no constraints", a.Artifact)
		}
	}
	for _, want := range []string{"requirements.md", "design.md", "tasks.md"} {
		if !arts[want] {
			t.Errorf("missing artifact brief for %s", want)
		}
	}

	// Text output mentions the EARS forms verbatim.
	text := b.Text()
	for _, f := range EarsForms() {
		if !strings.Contains(text, f) {
			t.Errorf("brief text missing EARS form %q", f)
		}
	}

	// JSON round-trips (SPECD_JSON output path) and carries the prompt.
	bp := NewAuthoringBrief("do the thing")
	raw, err := json.Marshal(bp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back AuthoringBrief
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Prompt != "do the thing" {
		t.Errorf("Prompt round-trip = %q", back.Prompt)
	}
	// Empty prompt is omitted from JSON.
	if strings.Contains(string(mustJSON(t, b)), "\"prompt\"") {
		t.Error("empty prompt should be omitempty in JSON")
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestAuthoringSync fails if a gate's constraint source changes but the brief
// does not — the brief must always reflect the live gate vars.
func TestAuthoringSync(t *testing.T) {
	b := NewAuthoringBrief("")
	if strings.Join(b.EarsForms, "\n") != strings.Join(EarsForms(), "\n") {
		t.Error("brief EarsForms drifted from EarsForms()")
	}
	if strings.Join(b.DesignSections, "\n") != strings.Join(DesignSections, "\n") {
		t.Error("brief DesignSections drifted from core.DesignSections")
	}
	if strings.Join(b.TaskKeys, "\n") != strings.Join(MandatoryKeys, "\n") {
		t.Error("brief TaskKeys drifted from core.MandatoryKeys")
	}
	if strings.Join(b.Roles, "\n") != strings.Join(ValidRoles, "\n") {
		t.Error("brief Roles drifted from core.ValidRoles")
	}
}
