package cmd

import (
	"fmt"
	"os"
	"strings"
)

// emitGuidance writes a spec's machine driving guidance (spec 01 R6). With
// asJSON the guidance round-trips as the core.Guidance contract; otherwise it is
// a compact human summary. The separation of legal commands from human-only
// actions (R6.1) and the suppression of task verify without an executable task
// (R6.2) are computed by guidanceForSpec — this function only renders.
func emitGuidance(root, slug string, asJSON bool) error {
	g, err := guidanceForSpec(root, slug)
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(g)
	}
	fmt.Fprintf(os.Stdout, "phase: %s (status %s)\n", g.Phase, g.Status)
	if g.RequiredArtifact != "" {
		fmt.Fprintf(os.Stdout, "required artifact: %s\n", g.RequiredArtifact)
	}
	if g.NextGate != "" {
		fmt.Fprintf(os.Stdout, "next gate (human approval): %s\n", g.NextGate)
	}
	fmt.Fprintf(os.Stdout, "legal commands: %s\n", strings.Join(g.LegalCommands, ", "))
	fmt.Fprintf(os.Stdout, "human-only: %s\n", strings.Join(g.HumanOnly, ", "))
	for _, blocker := range g.Blockers {
		fmt.Fprintf(os.Stdout, "blocker: %s\n", blocker)
	}
	return nil
}
