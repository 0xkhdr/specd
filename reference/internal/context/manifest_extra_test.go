package contextpkg

import (
	"testing"

	"github.com/0xkhdr/specd/internal/spec"
)

func TestManifestHelperBranches(t *testing.T) {
	if got := statusSourceArtifacts(spec.StatusRequirements); len(got) != 1 || got[0] != "requirements.md" {
		t.Fatalf("requirements artifacts = %#v", got)
	}
	if got := statusSourceArtifacts(spec.StatusComplete); len(got) != 1 || got[0] != "tasks.md" {
		t.Fatalf("complete artifacts = %#v", got)
	}
	if got := statusSourceArtifacts("unknown"); len(got) != 3 {
		t.Fatalf("unknown artifacts = %#v", got)
	}

	cases := []struct {
		task  string
		files []string
		want  string
	}{
		{"A1", []string{".specd/specs/demo/requirements.md"}, ".specd/skills/specd-requirements/SKILL.md"},
		{"A2", []string{"design.md"}, ".specd/skills/specd-design/SKILL.md"},
		{"A3", []string{"tasks.md"}, ".specd/skills/specd-tasks/SKILL.md"},
		{"A4", []string{"notes.md"}, ""},
		{"T1", nil, ".specd/skills/specd-execute/SKILL.md"},
	}
	for _, tc := range cases {
		if got := contextPhaseSkillPath(tc.task, tc.files); got != tc.want {
			t.Fatalf("contextPhaseSkillPath(%q,%v)=%q want %q", tc.task, tc.files, got, tc.want)
		}
	}

	roles := []struct {
		req  ContextRequest
		want string
	}{
		{ContextRequest{Status: spec.StatusRequirements}, "architect"},
		{ContextRequest{Status: spec.StatusVerifying}, "validator"},
		{ContextRequest{Status: spec.StatusComplete}, "documenter"},
		{ContextRequest{TaskID: "T1"}, "craftsman"},
	}
	for _, tc := range roles {
		if got := defaultContextRole(tc.req); got != tc.want {
			t.Fatalf("defaultContextRole(%+v)=%q want %q", tc.req, got, tc.want)
		}
	}

	if got := effectivePhase(ContextRequest{TaskID: "A1"}); got != spec.PhasePlan {
		t.Fatalf("authoring effective phase = %q", got)
	}
	if got := deriveContextBudget(ContextRequest{Status: spec.StatusDesign, Role: "architect", Files: []string{"a", "b"}}); got <= missionContextSoftCeiling {
		t.Fatalf("planning architect budget too small: %d", got)
	}
	if got := deriveContextBudget(ContextRequest{Status: spec.StatusVerifying, Role: "auditor", HostBudget: 500}); got != MinSoftCeiling {
		t.Fatalf("host budget should clamp to min: %d", got)
	}
	if got := clampContextBudget(MaxSoftCeiling + 1); got != MaxSoftCeiling {
		t.Fatalf("max clamp = %d", got)
	}
}
