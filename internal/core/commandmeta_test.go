package core_test

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestCommandMeta pins spec 03 R1/R6: every verb declares allowed phases
// (explicitly, never by silent default), an exit-code table containing at least
// codes 0 and 2, and at least one usage example.
func TestCommandMeta(t *testing.T) {
	for _, cmd := range core.Commands {
		if len(cmd.AllowedPhases) == 0 {
			t.Errorf("%s: no AllowedPhases declared (must declare PhaseAny explicitly if unrestricted)", cmd.Name)
		}
		for _, phase := range cmd.AllowedPhases {
			if phase != core.PhaseAny && !core.ValidPhase(phase) {
				t.Errorf("%s: AllowedPhases has invalid phase %q", cmd.Name, phase)
			}
		}
		codes := map[int]bool{}
		for _, ec := range cmd.ExitCodes {
			if ec.Meaning == "" {
				t.Errorf("%s: exit code %d has no meaning", cmd.Name, ec.Code)
			}
			codes[ec.Code] = true
		}
		if !codes[0] || !codes[2] {
			t.Errorf("%s: must declare exit codes 0 and 2, got %v", cmd.Name, codes)
		}
		if len(cmd.Examples) == 0 {
			t.Errorf("%s: no usage example declared", cmd.Name)
		}
		for _, flag := range cmd.Flags {
			if len(flag.Enum) > 0 && flag.Type != "string" {
				t.Errorf("%s: flag --%s has enum but type %q (enum flags take a string value)", cmd.Name, flag.Name, flag.Type)
			}
		}
	}
}

// TestCommandByName exercises the metadata lookup helpers dispatch relies on.
func TestCommandByName(t *testing.T) {
	verify, ok := core.CommandByName("verify")
	if !ok {
		t.Fatal("verify command not found")
	}
	if verify.AllowsPhase(core.PhaseAny) {
		t.Error("verify should be phase-restricted, not any")
	}
	if !verify.AllowsPhase(core.PhaseExecute) {
		t.Error("verify should allow execute phase")
	}
	if verify.AllowsPhase(core.PhasePerceive) {
		t.Error("verify must not allow perceive phase")
	}
	if _, ok := core.CommandByName("nonesuch"); ok {
		t.Error("unknown command reported as found")
	}
}

func TestOperationMetadataIsVersionedCompleteAndUnique(t *testing.T) {
	if core.OperationSchemaVersion != 1 {
		t.Fatalf("operation schema version = %d, want 1", core.OperationSchemaVersion)
	}
	seen := map[string]bool{}
	covered := map[string]bool{}
	for _, op := range core.Operations {
		if op.ID == "" || op.Command == "" || op.Usage == "" || op.Actor == "" || op.Effect == "" || len(op.AllowedPhases) == 0 || op.ScopeSource == "" || op.NetworkClass == "" || len(op.ExitCodes) == 0 || len(op.Examples) == 0 {
			t.Fatalf("incomplete operation metadata: %#v", op)
		}
		if seen[op.ID] {
			t.Fatalf("duplicate operation id %q", op.ID)
		}
		seen[op.ID] = true
		covered[op.Command] = true
		if _, ok := core.CommandByName(op.Command); !ok {
			t.Fatalf("operation %q names unknown command %q", op.ID, op.Command)
		}
	}
	for _, command := range core.Commands {
		if !covered[command.Name] {
			t.Errorf("command %q has no operation", command.Name)
		}
	}
}

func TestOperationMixedCommandEffects(t *testing.T) {
	tests := []struct {
		id     string
		effect core.OperationEffect
		actor  core.OperationActor
	}{
		{"eval.import", core.EffectStateWrite, core.ActorAgent},
		{"eval.status", core.EffectRead, core.ActorAgent},
		{"task.show", core.EffectRead, core.ActorAgent},
		{"task.override", core.EffectStateWrite, core.ActorHuman},
		{"complete-task", core.EffectStateWrite, core.ActorAgent},
		{"report.render", core.EffectRead, core.ActorAgent},
	}
	for _, tt := range tests {
		op, ok := core.OperationByID(tt.id)
		if !ok {
			t.Errorf("operation %q missing", tt.id)
			continue
		}
		if op.Effect != tt.effect || op.Actor != tt.actor {
			t.Errorf("%s = effect %q actor %q, want %q/%q", tt.id, op.Effect, op.Actor, tt.effect, tt.actor)
		}
	}
}

// TestGuideModel pins spec 01 R6.1: driving guidance for a phase separates the
// machine-legal commands from the human-only actions (so an agent never treats
// approval as self-serve), and names the artifact the phase must produce.
func TestGuideModel(t *testing.T) {
	g := core.GuidanceForPhase(core.StatusRequirements, nil)
	if g.Phase != core.PhasePerceive {
		t.Fatalf("phase = %q", g.Phase)
	}
	if g.RequiredArtifact != "requirements.md" {
		t.Fatalf("required artifact = %q", g.RequiredArtifact)
	}
	if !contains(g.HumanOnly, "approve") {
		t.Fatalf("approve must be a human-only action, got %v", g.HumanOnly)
	}
	if contains(g.LegalCommands, "approve") {
		t.Fatalf("approve must not appear as a machine-legal command, got %v", g.LegalCommands)
	}
	if !contains(g.LegalCommands, "status") {
		t.Fatalf("status should be machine-legal, got %v", g.LegalCommands)
	}
	if core.NextStatus(core.StatusRequirements) != core.StatusDesign {
		t.Fatalf("next status after requirements = %q", core.NextStatus(core.StatusRequirements))
	}
	if core.NextStatus(core.StatusComplete) != "" {
		t.Fatalf("final status should have no successor")
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
