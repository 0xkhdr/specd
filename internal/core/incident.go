package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxIncidentRefs = 16

const IncidentPreventionSchemaV1 = 1

var incidentIdentity = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)

type PreventionKind string

const (
	PreventionRegressionTest PreventionKind = "regression_test"
	PreventionEval           PreventionKind = "eval"
)

// IncidentPreventionV1 is append-only closure evidence. Owner is always a
// human/team identity; EvidenceRef and WhyCaughtRef point to durable evidence
// rather than copying potentially sensitive payloads into context.
type IncidentPreventionV1 struct {
	SchemaVersion int            `json:"schema_version"`
	Kind          PreventionKind `json:"kind"`
	Owner         string         `json:"owner"`
	EvidenceRef   string         `json:"evidence_ref"`
	WhyCaughtRef  string         `json:"why_caught_ref"`
}

func ValidatePreventiveEvidence(record IncidentPreventionV1, required bool) error {
	if record.SchemaVersion != 0 && record.SchemaVersion != IncidentPreventionSchemaV1 {
		return fmt.Errorf("unsupported incident prevention schema_version %d", record.SchemaVersion)
	}
	if record.Kind != "" && record.Kind != PreventionRegressionTest && record.Kind != PreventionEval {
		return fmt.Errorf("unknown preventive evidence kind %q", record.Kind)
	}
	if !required {
		return nil
	}
	if record.Kind == "" || !incidentIdentity.MatchString(record.Owner) {
		return errors.New("incident closure requires prevention kind and human/team owner")
	}
	if err := validateIncidentRef("preventive evidence", record.EvidenceRef); err != nil {
		return err
	}
	if err := validateIncidentRef("why recurrence is now caught", record.WhyCaughtRef); err != nil {
		return err
	}
	return nil
}

func IncidentPreventionPath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "incident-prevention.jsonl")
}

func RecordIncidentPrevention(root, slug string, record IncidentPreventionV1) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}
	if err := ValidatePreventiveEvidence(record, true); err != nil {
		return err
	}
	record.SchemaVersion = IncidentPreventionSchemaV1
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = WithSpecLock(root, func() (struct{}, error) {
		return struct{}{}, AppendFile(IncidentPreventionPath(root, slug), string(raw)+"\n")
	})
	return err
}

func LoadIncidentPrevention(path string) ([]IncidentPreventionV1, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var records []IncidentPreventionV1
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var record IncidentPreventionV1
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("decode incident prevention: %w", err)
		}
		if err := ValidatePreventiveEvidence(record, true); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, scanner.Err()
}

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
	for _, identity := range []struct{ label, value string }{
		{"release", seed.ReleaseID},
		{"deployment", seed.DeploymentID},
		{"criterion", seed.CriterionID},
	} {
		if !incidentIdentity.MatchString(identity.value) {
			return fmt.Errorf("incident %s identity is invalid or exceeds 128 bytes", identity.label)
		}
	}
	if len(seed.EvidenceRefs) == 0 || len(seed.EvidenceRefs) > maxIncidentRefs {
		return fmt.Errorf("incident requires 1-%d evidence references", maxIncidentRefs)
	}
	seen := map[string]bool{}
	for _, ref := range seed.EvidenceRefs {
		if err := validateIncidentRef("incident evidence", ref); err != nil {
			return err
		}
		if seen[ref] {
			return fmt.Errorf("duplicate incident evidence reference %q", ref)
		}
		seen[ref] = true
	}
	return nil
}

func validateIncidentRef(label, ref string) error {
	if len(ref) == 0 || len(ref) > 256 || strings.ContainsAny(ref, "\x00\r\n") {
		return fmt.Errorf("%s reference must be 1-256 bytes", label)
	}
	u, err := url.Parse(ref)
	if err != nil || u.Scheme == "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return fmt.Errorf("unsafe %s reference %q", label, ref)
	}
	return nil
}

type IncidentSuccessorPlan struct {
	Provenance ProvenanceV1 `json:"provenance"`
	Link       ProgramLink  `json:"link"`
}

// PlanIncidentSuccessor is a deterministic projection. Caller persists only
// successor-owned artifacts and program link; source spec remains untouched.
func PlanIncidentSuccessor(successor string, seed IncidentSeed) (IncidentSuccessorPlan, error) {
	if err := ValidateSlug(successor); err != nil {
		return IncidentSuccessorPlan{}, err
	}
	if err := ValidateIncidentSeed(seed); err != nil {
		return IncidentSuccessorPlan{}, err
	}
	reason := fmt.Sprintf("incident %s/%s criterion %s", seed.ReleaseID, seed.DeploymentID, seed.CriterionID)
	link := ProgramLink{From: successor, To: seed.SourceSpec, Kind: LinkKindRegresses, Reason: reason}
	provenance := ProvenanceV1{
		SchemaVersion: ProvenanceSchemaV1, SourceType: SourceIncident,
		SourceRef:     fmt.Sprintf("incident:%s/%s/%s", seed.ReleaseID, seed.DeploymentID, seed.CriterionID),
		AffectedSpecs: []string{seed.SourceSpec},
		PriorLinks:    []ProvenanceLink{{From: successor, To: seed.SourceSpec, Kind: LinkKindRegresses, Reason: reason}},
	}
	return IncidentSuccessorPlan{Provenance: provenance, Link: link}, nil
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
