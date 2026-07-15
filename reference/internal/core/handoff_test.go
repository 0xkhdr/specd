package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateHandoff(t *testing.T) {
	cases := []struct {
		name string
		h    *ACPHandoff
		ok   bool
	}{
		{"nil is fresh dispatch", nil, true},
		{"scout to craftsman", &ACPHandoff{From: "scout", Reason: "scan complete", Artifacts: []string{"notes.md"}}, true},
		{"unknown role", &ACPHandoff{From: "wizard", Reason: "x"}, false},
		{"empty reason", &ACPHandoff{From: "scout", Reason: "  "}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateHandoff(c.h)
			if c.ok && err != nil {
				t.Fatalf("want valid, got %v", err)
			}
			if !c.ok && err == nil {
				t.Fatal("want invalid, got nil")
			}
		})
	}
}

// TestHandoffDigestStable proves adding a handoff/tier to a mission does not
// change its dispatch digest — the digest covers the work contract, not routing
// or handoff provenance.
func TestHandoffDigestStable(t *testing.T) {
	base := PinkyMission{
		Spec:          "demo",
		TaskID:        "T1",
		Role:          "craftsman",
		Contract:      "do the thing",
		Acceptance:    "it is done",
		VerifyCommand: "true",
	}
	plain := pinkyMissionDigest(base)

	withHandoff := base
	withHandoff.Tier = "premium"
	withHandoff.Handoff = &ACPHandoff{From: "scout", Reason: "scan complete", Artifacts: []string{"a.md"}}
	if got := pinkyMissionDigest(withHandoff); got != plain {
		t.Fatalf("digest changed with handoff/tier: %q != %q", got, plain)
	}
}

// TestScoutToCraftsmanHandoffRoundTrip: a scout->craftsman handoff survives the
// ACP mission payload JSON round-trip and validates.
func TestScoutToCraftsmanHandoffRoundTrip(t *testing.T) {
	payload := ACPMissionPayload{
		DispatchDigest: strings.Repeat("a", 64),
		Role:           "craftsman",
		Contract:       "implement parser",
		Acceptance:     "tests pass",
		VerifyCommand:  "go test ./...",
		Authority:      ACPAuthority{ReadOnly: false, AllowedActions: []string{"edit"}},
		Tier:           "standard",
		Handoff: &ACPHandoff{
			From:      "scout",
			Reason:    "located the parser and its callers",
			Artifacts: []string{".specd/specs/demo/scout-notes.md"},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var got ACPMissionPayload
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Handoff == nil || got.Handoff.From != "scout" {
		t.Fatalf("handoff lost in round-trip: %+v", got.Handoff)
	}
	if got.Tier != "standard" {
		t.Fatalf("tier lost: %q", got.Tier)
	}
	if err := ValidateHandoff(got.Handoff); err != nil {
		t.Fatalf("round-tripped handoff invalid: %v", err)
	}
}

// TestBriefRendersHandoff: the mission brief surfaces the handoff provenance so
// the receiving worker sees where its inputs came from.
func TestBriefRendersHandoff(t *testing.T) {
	var sb strings.Builder
	renderHandoff(&sb, &ACPHandoff{From: "scout", Reason: "scan done", Artifacts: []string{"notes.md"}})
	out := sb.String()
	if !strings.Contains(out, "from `scout`") || !strings.Contains(out, "scan done") {
		t.Fatalf("handoff not rendered: %q", out)
	}
	if !strings.Contains(out, "notes.md") {
		t.Fatalf("artifacts not rendered: %q", out)
	}

	// Nil handoff renders nothing (byte-stable with pre-handoff briefs).
	var empty strings.Builder
	renderHandoff(&empty, nil)
	if empty.Len() != 0 {
		t.Fatalf("nil handoff rendered %q", empty.String())
	}
}
