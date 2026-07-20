package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

func runStatus(root string, args []string, flags map[string]string) error {
	if flagEnabled(flags, "program") {
		if len(args) != 0 {
			return errors.New("usage: specd status --program (takes no spec)")
		}
		view, err := renderProgram(root)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, view)
		return nil
	}
	if flagEnabled(flags, "guide") {
		if len(args) != 1 {
			return errors.New("usage: specd status <spec> --guide [--json]")
		}
		return emitGuidance(root, args[0], flagEnabled(flags, "json"))
	}
	if len(args) != 1 {
		return errors.New("usage: status slug [--json]")
	}
	model, err := reportModel(root, args[0])
	if err != nil {
		return err
	}
	coverage, err := criterionCoverage(root, args[0])
	if err != nil {
		return err
	}
	spec, err := loadSpec(root, args[0])
	if err != nil {
		return err
	}
	escalated, ratchetActive, err := escalatedAdvisory(root, args[0], spec.Tasks)
	if err != nil {
		return err
	}
	if flagEnabled(flags, "json") {
		// Records are projected verbatim (RawMessage), never re-synthesized, so
		// decision/midreq text/scope/actor/timestamp round-trip exactly (R3.4).
		state, err := core.LoadState(core.StatePath(root, args[0]))
		if err != nil {
			return err
		}
		guidance, err := guidanceForSpec(root, args[0])
		if err != nil {
			return err
		}
		return writeJSON(struct {
			core.ReportModel
			Records   map[string]json.RawMessage `json:"records,omitempty"`
			Criteria  []requirementCoverage      `json:"criteria,omitempty"`
			Escalated map[string]int             `json:"escalated,omitempty"`
			Locator   core.Locator               `json:"locator"`
		}{model, state.Records, coverage, escalated,
			core.NewLocator(args[0], state.Revision, guidance, core.ActorAgent, core.AuthorityNone, core.HostCapabilities{})})
	}
	fmt.Fprint(os.Stdout, core.RenderStatus(model))
	fmt.Fprint(os.Stdout, renderCriterionCoverage(coverage))
	fmt.Fprint(os.Stdout, renderEscalated(escalated, ratchetActive))
	return nil
}

// renderEscalated formats the escalated-task section for `status` text output.
// When the ratchet is active the tasks are genuinely blocked; when disabled the
// section is advisory (repeated failures still surfaced, spec 06 R2/R6).
func renderEscalated(escalated map[string]int, ratchetActive bool) string {
	if len(escalated) == 0 {
		return ""
	}
	header := "Escalated (advisory; ratchet disabled):"
	if ratchetActive {
		header = "Escalated (blocked — clear with `specd task <id> --override --reason <text>`):"
	}
	var b strings.Builder
	b.WriteString("\n" + header + "\n")
	for _, id := range slices.Sorted(maps.Keys(escalated)) {
		fmt.Fprintf(&b, "  %s — %d consecutive verify failures\n", id, escalated[id])
	}
	return b.String()
}

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
		// Additive: the Guidance fields stay at the top level exactly where they
		// were, and `locator` is a new sibling key. A consumer that predates it
		// still parses this response unchanged (R5.1).
		state, err := core.LoadState(core.StatePath(root, slug))
		if err != nil {
			return err
		}
		return writeJSON(struct {
			core.Guidance
			Locator core.Locator `json:"locator"`
		}{g, core.NewLocator(slug, state.Revision, g, core.ActorAgent, core.AuthorityNone, core.HostCapabilities{})})
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
