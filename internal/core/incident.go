package core

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const maxIncidentRefs = 16

// IncidentSeed contains bounded identity and reference facts only. Raw external
// payloads are intentionally absent so they cannot enter standing context.
type IncidentSeed struct {
	SourceSpec   string
	ReleaseID    string
	DeploymentID string
	CriterionID  string
	EvidenceRefs []string
}

func ValidateIncidentSeed(seed IncidentSeed) error {
	if err := ValidateSlug(seed.SourceSpec); err != nil {
		return fmt.Errorf("invalid source spec: %w", err)
	}
	if firstEmpty("release", seed.ReleaseID, "deployment", seed.DeploymentID, "criterion", seed.CriterionID) != "" {
		return fmt.Errorf("incident source release, deployment, and criterion are required")
	}
	if len(seed.EvidenceRefs) == 0 || len(seed.EvidenceRefs) > maxIncidentRefs {
		return fmt.Errorf("incident requires 1-%d evidence references", maxIncidentRefs)
	}
	seen := map[string]bool{}
	for _, ref := range seed.EvidenceRefs {
		if len(ref) > 256 {
			return fmt.Errorf("incident evidence reference exceeds 256 bytes")
		}
		u, err := url.Parse(ref)
		if err != nil || u.Scheme == "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return fmt.Errorf("unsafe incident evidence reference %q", ref)
		}
		if seen[ref] {
			return fmt.Errorf("duplicate incident evidence reference %q", ref)
		}
		seen[ref] = true
	}
	return nil
}

func IncidentSpecDocuments(slug string, seed IncidentSeed) (requirements, design, tasks, memory string, err error) {
	if err = ValidateSlug(slug); err != nil {
		return "", "", "", "", err
	}
	if err = ValidateIncidentSeed(seed); err != nil {
		return "", "", "", "", err
	}
	refs := append([]string(nil), seed.EvidenceRefs...)
	sort.Strings(refs)
	var bullets strings.Builder
	for _, ref := range refs {
		fmt.Fprintf(&bullets, "  - `%s`\n", ref)
	}
	requirements = fmt.Sprintf(`# Requirements — %s

## Incident source

- Source spec: %s
- Source release: %s
- Source deployment: %s
- Failed criterion: %s
- Bounded evidence references:
%s
### R1 — Prevent recurrence

- R1.1: When this incident is analyzed, the system shall preserve source identity and prove a deterministic prevention check without loading raw external payloads.
`, slug, seed.SourceSpec, seed.ReleaseID, seed.DeploymentID, seed.CriterionID, bullets.String())
	design = fmt.Sprintf("# Design — %s\n\n## Decision\n\nAnalyze bounded references from `%s` offline. Preserve source delivery ledgers unchanged.\n", slug, seed.SourceSpec)
	tasks = fmt.Sprintf("# Tasks — %s\n\n| id | role | files | deps | verify | req |\n|---|---|---|---|---|---|\n| T1 | craftsman | requirements.md,design.md,tasks.md | - | printf ok | R1.1 |\n", slug)
	memory = fmt.Sprintf("# Memory — %s\n\nIncident source: %s/%s/%s. References only; raw payload remains external.\n", slug, seed.SourceSpec, seed.ReleaseID, seed.DeploymentID)
	return requirements, design, tasks, memory, nil
}
