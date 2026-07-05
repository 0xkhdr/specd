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
