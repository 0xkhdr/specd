package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/core"
)

func runDrift(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return fmt.Errorf("%w: specd drift <spec> [--json]", ErrUsage)
	}
	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return fmt.Errorf("%w: %v", ErrUsage, err)
	}
	invariants, err := core.LoadDriftDeclarations(core.DriftPath(root, slug))
	if err != nil {
		return err
	}
	decisions, err := core.LoadDecisions(core.DecisionPath(root, slug))
	if err != nil {
		return err
	}
	evidence, err := core.LoadEvidenceRecords(core.EvidencePath(root, slug))
	if err != nil {
		return err
	}
	findings := core.ProjectDrift(invariants, decisions, evidence, core.Clock(), slug)
	if flagEnabled(flags, "json") {
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		for _, finding := range findings {
			if err := enc.Encode(finding); err != nil {
				return err
			}
		}
		return nil
	}
	for _, finding := range findings {
		fmt.Fprintf(os.Stdout, "%s | %s | %s | %s | %s | %s\n", finding.Status, finding.Severity, finding.Source, dashValue(finding.Path), dashValue(finding.LastPassingHead), dashValue(finding.SuggestedCommand))
	}
	return nil
}

func dashValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
