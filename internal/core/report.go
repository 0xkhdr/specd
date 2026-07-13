package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type ReportTask struct {
	ID          string        `json:"id"`
	Status      TaskRunStatus `json:"status"`
	Role        string        `json:"role,omitempty"`
	Files       string        `json:"files,omitempty"`
	Verify      string        `json:"verify,omitempty"`
	Acceptance  string        `json:"acceptance,omitempty"`
	EvidenceRef string        `json:"evidence_ref,omitempty"`
}

type ReportModel struct {
	Slug     string       `json:"slug"`
	Total    int          `json:"total"`
	Complete int          `json:"complete"`
	Pending  int          `json:"pending"`
	Blocked  int          `json:"blocked"`
	Running  int          `json:"running"`
	Tasks    []ReportTask `json:"tasks"`
}

type QualityReport struct {
	Passed  []string           `json:"passed,omitempty"`
	Missing []string           `json:"missing,omitempty"`
	Stale   []string           `json:"stale,omitempty"`
	Scores  map[string]float64 `json:"scores,omitempty"`
	Review  string             `json:"review,omitempty"`
}

func RenderQualityReport(q QualityReport) string {
	var b strings.Builder
	b.WriteString("proof:\n")
	for _, value := range q.Passed {
		fmt.Fprintf(&b, "  passed: %s\n", value)
	}
	b.WriteString("gaps:\n")
	for _, value := range q.Missing {
		fmt.Fprintf(&b, "  missing: %s\n", value)
	}
	b.WriteString("stale:\n")
	for _, value := range q.Stale {
		fmt.Fprintf(&b, "  stale: %s\n", value)
	}
	b.WriteString("scores:\n")
	keys := make([]string, 0, len(q.Scores))
	for key := range q.Scores {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "  %s: %.3f\n", key, q.Scores[key])
	}
	fmt.Fprintf(&b, "review: %s\n", q.Review)
	return b.String()
}

func BuildReportModel(slug string, tasks []TaskRow, status map[string]TaskRunStatus, evidence map[string]EvidenceRecord) ReportModel {
	model := ReportModel{Slug: slug, Total: len(tasks), Tasks: make([]ReportTask, 0, len(tasks))}
	for _, task := range tasks {
		taskStatus := status[task.ID]
		if taskStatus == "" {
			taskStatus = statusFromMarker(task.Marker)
		}
		record := evidence[task.ID]
		model.Tasks = append(model.Tasks, ReportTask{
			ID:          task.ID,
			Status:      taskStatus,
			Role:        task.Role,
			Files:       task.Files,
			Verify:      task.Verify,
			Acceptance:  task.Acceptance,
			EvidenceRef: record.EvidenceRef,
		})
		switch taskStatus {
		case TaskComplete:
			model.Complete++
		case TaskBlocked:
			model.Blocked++
		case TaskRunning:
			model.Running++
		default:
			model.Pending++
		}
	}
	return model
}

func RenderStatus(model ReportModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "spec: %s\n", model.Slug)
	fmt.Fprintf(&b, "tasks: %d complete, %d running, %d blocked, %d pending, %d total\n", model.Complete, model.Running, model.Blocked, model.Pending, model.Total)
	for _, task := range model.Tasks {
		fmt.Fprintf(&b, "%s %s", task.ID, task.Status)
		if task.Verify != "" {
			fmt.Fprintf(&b, " verify=%q", task.Verify)
		}
		if task.EvidenceRef != "" {
			fmt.Fprintf(&b, " evidence=%s", task.EvidenceRef)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func RenderPRSummary(model ReportModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## specd report: %s\n\n", model.Slug)
	fmt.Fprintf(&b, "- complete: %d/%d\n", model.Complete, model.Total)
	fmt.Fprintf(&b, "- running: %d\n", model.Running)
	fmt.Fprintf(&b, "- blocked: %d\n", model.Blocked)
	fmt.Fprintf(&b, "- pending: %d\n\n", model.Pending)
	b.WriteString("| task | status | verify |\n|---|---|---|\n")
	for _, task := range model.Tasks {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", task.ID, task.Status, escapeTable(task.Verify))
	}
	return b.String()
}

func statusFromMarker(marker string) TaskRunStatus {
	switch marker {
	case "✅":
		return TaskComplete
	case "🚧":
		return TaskRunning
	case "⛔":
		return TaskBlocked
	default:
		return TaskPending
	}
}

func escapeTable(value string) string {
	return strings.ReplaceAll(value, "|", `\|`)
}

// ProofCoverage is one requirement's criterion tally, projected into the
// lifecycle proof without depending on the cmd package's private type.
type ProofCoverage struct {
	Req     int `json:"req"`
	Passing int `json:"passing"`
	Total   int `json:"total"`
}

// EscapedDefect links a corrective amendment to a requirement that already had
// passing evidence — a defect that escaped a green gate and its rechecks.
type EscapedDefect struct {
	AffectedID string   `json:"affected_id"`
	ChangeID   string   `json:"change_id"`
	Rechecks   []string `json:"rechecks"`
}

// LifecycleProof is the deterministic R8.2 report: requirement-to-evidence
// coverage, stale approval records, amendments, and escaped-defect links. It is
// a pure projection of on-disk state; identical inputs render identical bytes.
type LifecycleProof struct {
	Slug       string          `json:"slug"`
	Coverage   []ProofCoverage `json:"coverage"`
	Stale      []string        `json:"stale,omitempty"`
	Amendments []Amendment     `json:"amendments,omitempty"`
	Escaped    []EscapedDefect `json:"escaped_defects,omitempty"`
}

func BuildLifecycleProof(slug string, coverage []ProofCoverage, stale []string, amendments []Amendment) LifecycleProof {
	proof := LifecycleProof{Slug: slug, Coverage: coverage, Stale: stale, Amendments: amendments}
	passing := make(map[int]bool, len(coverage))
	for _, c := range coverage {
		if c.Passing > 0 {
			passing[c.Req] = true
		}
	}
	for _, a := range amendments {
		for _, id := range a.AffectedIDs {
			if req, ok := requirementNumber(id); ok && passing[req] {
				proof.Escaped = append(proof.Escaped, EscapedDefect{
					AffectedID: id, ChangeID: a.ChangeID, Rechecks: a.RequiredRechecks,
				})
			}
		}
	}
	return proof
}

// requirementNumber extracts the requirement index from a contract address like
// "R1" or "R1.2"; anything that is not an R-addressed id returns ok=false.
func requirementNumber(id string) (int, bool) {
	if len(id) < 2 || (id[0] != 'R' && id[0] != 'r') {
		return 0, false
	}
	digits := id[1:]
	if dot := strings.IndexByte(digits, '.'); dot >= 0 {
		digits = digits[:dot]
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	return n, true
}

func RenderLifecycleProof(p LifecycleProof) string {
	var b strings.Builder
	fmt.Fprintf(&b, "lifecycle proof: %s\n", p.Slug)
	b.WriteString("coverage:\n")
	for _, c := range p.Coverage {
		fmt.Fprintf(&b, "  R%d %d/%d\n", c.Req, c.Passing, c.Total)
	}
	b.WriteString("stale:\n")
	for _, key := range p.Stale {
		fmt.Fprintf(&b, "  stale: %s\n", key)
	}
	b.WriteString("amendments:\n")
	for _, a := range p.Amendments {
		fmt.Fprintf(&b, "  amendment %s affects=%s rechecks=%s\n", a.ChangeID, strings.Join(a.AffectedIDs, ","), strings.Join(a.RequiredRechecks, ","))
	}
	b.WriteString("escaped-defects:\n")
	for _, e := range p.Escaped {
		fmt.Fprintf(&b, "  escaped %s -> %s rechecks=%s\n", e.AffectedID, e.ChangeID, strings.Join(e.Rechecks, ","))
	}
	return b.String()
}

func RenderLifecycleProofJSON(p LifecycleProof) (string, error) {
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw) + "\n", nil
}

func SortedReportTaskIDs(model ReportModel) []string {
	ids := make([]string, 0, len(model.Tasks))
	for _, task := range model.Tasks {
		ids = append(ids, task.ID)
	}
	sort.Strings(ids)
	return ids
}
