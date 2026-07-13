package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type DriftStatus string

const (
	DriftHolds        DriftStatus = "holds"
	Drifted           DriftStatus = "drifted"
	DriftNotEvaluable DriftStatus = "not-evaluable"
	DriftNone         DriftStatus = "none"
)

type DriftSeverity string

const (
	DriftSeverityUnknown  DriftSeverity = "unknown"
	DriftSeverityLow      DriftSeverity = "low"
	DriftSeverityMedium   DriftSeverity = "medium"
	DriftSeverityHigh     DriftSeverity = "high"
	DriftSeverityCritical DriftSeverity = "critical"
)

type DriftInvariantV1 struct {
	ID           string        `json:"id"`
	Path         string        `json:"path"`
	EvidenceTask string        `json:"evidence_task"`
	Severity     DriftSeverity `json:"severity"`
}

func (d DriftInvariantV1) Validate() error {
	if strings.TrimSpace(d.ID) == "" || strings.TrimSpace(d.EvidenceTask) == "" {
		return errors.New("drift invariant id and evidence_task are required")
	}
	if strings.TrimSpace(d.Path) == "" {
		return fmt.Errorf("drift invariant %s path is required", d.ID)
	}
	clean := filepath.ToSlash(filepath.Clean(d.Path))
	if filepath.IsAbs(d.Path) || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("drift invariant %s path must be workspace-relative", d.ID)
	}
	switch d.Severity {
	case DriftSeverityUnknown, DriftSeverityLow, DriftSeverityMedium, DriftSeverityHigh, DriftSeverityCritical:
	default:
		return fmt.Errorf("drift invariant %s has invalid severity %q", d.ID, d.Severity)
	}
	return nil
}

type DriftDeclarationsV1 struct {
	SchemaVersion int                `json:"schema_version"`
	Invariants    []DriftInvariantV1 `json:"invariants"`
}

func DriftPath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "drift.json")
}

func LoadDriftDeclarations(path string) ([]DriftInvariantV1, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc DriftDeclarationsV1
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode drift declarations: %w", err)
	}
	if doc.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported drift schema_version %d", doc.SchemaVersion)
	}
	seen := map[string]bool{}
	for _, invariant := range doc.Invariants {
		if err := invariant.Validate(); err != nil {
			return nil, err
		}
		if seen[invariant.ID] {
			return nil, fmt.Errorf("duplicate drift invariant %q", invariant.ID)
		}
		seen[invariant.ID] = true
	}
	return append([]DriftInvariantV1(nil), doc.Invariants...), nil
}

type DriftFinding struct {
	Source           string        `json:"source"`
	Path             string        `json:"path,omitempty"`
	Severity         DriftSeverity `json:"severity"`
	Status           DriftStatus   `json:"status"`
	LastPassingHead  string        `json:"last_passing_head,omitempty"`
	SuggestedCommand string        `json:"suggested_command,omitempty"`
}

// ProjectDrift is a pure, read-only projection. Evidence order is append order;
// its last record supplies current status while the most recent pass remains
// auditable. Caller supplies evaluation time so identical inputs stay stable.
func ProjectDrift(invariants []DriftInvariantV1, decisions []DecisionV1, evidence []EvidenceRecord, asOf time.Time, slug string) []DriftFinding {
	if len(invariants) == 0 {
		return []DriftFinding{{Source: "none", Severity: DriftSeverityUnknown, Status: DriftNone}}
	}
	active := map[string][]string{}
	for _, d := range decisions {
		if d.ActiveAt(asOf) {
			for _, id := range d.AffectedInvariants {
				active[id] = append(active[id], d.ID)
			}
		}
	}
	for id := range active {
		sort.Strings(active[id])
	}
	type evidenceState struct {
		latest   *EvidenceRecord
		lastPass string
	}
	states := map[string]evidenceState{}
	for i := range evidence {
		r := &evidence[i]
		s := states[r.TaskID]
		s.latest = r
		if r.ExitCode == 0 && HeadPinned(r.GitHead) {
			s.lastPass = r.GitHead
		}
		states[r.TaskID] = s
	}
	items := append([]DriftInvariantV1(nil), invariants...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	findings := make([]DriftFinding, 0, len(items))
	for _, inv := range items {
		source := "invariant:" + inv.ID
		if ids := active[inv.ID]; len(ids) > 0 {
			source = "decision:" + strings.Join(ids, ",") + "/" + source
		}
		state := states[inv.EvidenceTask]
		status := DriftNotEvaluable
		lastPassing := state.lastPass
		if lastPassing == "" {
			lastPassing = "unknown"
		}
		if state.latest != nil && HeadPinned(state.latest.GitHead) {
			if state.latest.ExitCode == 0 {
				status = DriftHolds
			} else {
				status = Drifted
			}
		}
		findings = append(findings, DriftFinding{Source: source, Path: inv.Path, Severity: inv.Severity, Status: status, LastPassingHead: lastPassing, SuggestedCommand: "specd new " + slug + "-drift"})
	}
	return findings
}
