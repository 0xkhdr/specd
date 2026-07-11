package core_test

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestDesignParse(t *testing.T) {
	raw := []byte("# Design\n\n- references: R1, R2.1\n- boundaries: core vs cmd\n- owner: alice\n")
	doc := core.ParseDesign(raw)
	if len(doc.Refs) != 2 || doc.Refs[0] != "R1" || doc.Refs[1] != "R2.1" {
		t.Fatalf("refs = %v", doc.Refs)
	}
	if doc.Fields["boundaries"] != "core vs cmd" || doc.Fields["owner"] != "alice" {
		t.Fatalf("fields = %v", doc.Fields)
	}
	if string(doc.Raw) != string(raw) {
		t.Fatal("parser mutated author bytes")
	}
}

func TestDesignValidateUnknownRef(t *testing.T) {
	known := map[string]bool{"R1": true}
	doc := core.ParseDesign([]byte("- references: R1, R9\n"))
	f := core.ValidateDesign(doc, known, false)
	if len(f) != 1 || f[0].Ref != "R9" {
		t.Fatalf("expected only R9 unknown, got %+v", f)
	}
}

func TestDesignValidateContractProfile(t *testing.T) {
	known := map[string]bool{"R1": true, "R1.1": true}
	partial := core.ParseDesign([]byte("- references: R1\n- boundaries: x\n"))
	// Default profile: an incomplete contract still approves (R7.1 compatibility).
	if f := core.ValidateDesign(partial, known, false); len(f) != 0 {
		t.Fatalf("default profile should not require contract fields, got %+v", f)
	}
	// Production profile: missing decision metadata is refused (R2.1).
	if f := core.ValidateDesign(partial, known, true); len(f) == 0 {
		t.Fatal("production profile should require the full contract")
	}
	// A complete contract passes under production profile.
	complete := core.ParseDesign([]byte(
		"- references: R1.1\n- boundaries: b\n- interfaces: i\n- invariants: inv\n" +
			"- failure: f\n- integration: ig\n- alternatives: a\n- disposition: d\n- owner: o\n"))
	if f := core.ValidateDesign(complete, known, true); len(f) != 0 {
		t.Fatalf("complete contract should pass, got %+v", f)
	}
}

func TestDesignRequirementIDSet(t *testing.T) {
	set := core.RequirementIDSet("### R1 — Title\n\n- R1.1: When x, the system shall y.\n")
	if !set["R1"] || !set["R1.1"] {
		t.Fatalf("id set missing entries: %v", set)
	}
}
