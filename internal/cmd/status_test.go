package cmd

import (
	"encoding/json"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestStatusGuideJSON pins spec 01 R6.1: `status --guide --json` emits the
// machine driving guidance with the phase, the required artifact, the
// machine-legal commands, and approval kept in the human-only set.
func TestStatusGuideJSON(t *testing.T) {
	root := newDemoSpec(t)
	out, err := captureStdout(t, func() error {
		return Run(root, "status", []string{"demo"}, map[string]string{"guide": "", "json": ""})
	})
	if err != nil {
		t.Fatalf("status --guide --json: %v", err)
	}
	var g core.Guidance
	if err := json.Unmarshal([]byte(out), &g); err != nil {
		t.Fatalf("guide json: %v (out=%q)", err, out)
	}
	if g.Phase != core.PhasePerceive || g.RequiredArtifact != "requirements.md" {
		t.Fatalf("guidance = %+v", g)
	}
	if !containsStr(g.HumanOnly, "approve") || containsStr(g.LegalCommands, "approve") {
		t.Fatalf("approve must be human-only, never machine-legal: %+v", g)
	}
}

// TestStatusGuideSuppressesTaskVerify pins spec 01 R6.2: with no executable
// task, the guidance does not suggest task verify (nor agent self-approval).
func TestStatusGuideSuppressesTaskVerify(t *testing.T) {
	root := newDemoSpec(t)
	g, err := guidanceForSpec(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if containsStr(g.LegalCommands, "verify") {
		t.Fatalf("verify must not be suggested without an executable task: %v", g.LegalCommands)
	}
}
