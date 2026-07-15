package core

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// review.go implements the AI-first review workflow gate (V8/P4.1) under the
// "demand the artifact" pattern: the binary never judges code, it makes skipping
// the judgment impossible. `specd review` scaffolds a structured review_report.md;
// the review gate then checks that the report exists, is structurally valid,
// carries a verdict, and is *fresh* (newer than the latest task completion) so an
// old rubber-stamp cannot pass. Human approval stays final — the report is
// evidence, not the decision.

// reviewReportName is the artifact the reviewer authors and the gate reads.
const reviewReportName = "review_report.md"

// reviewSections are the mandatory H2 sections a valid review report must carry,
// in canonical order. The Verdict section additionally must name a decision.
var reviewSections = []string{
	"Summary",
	"Bugs",
	"Security",
	"Hallucinated Dependencies",
	"Style",
	"Verdict",
}

// ReviewVerdict is the parsed decision. Only approve|revise are valid.
type ReviewVerdict string

const (
	ReviewApprove ReviewVerdict = "approve"
	ReviewRevise  ReviewVerdict = "revise"
)

// ReviewReport is the parsed, validated review artifact.
type ReviewReport struct {
	Verdict  ReviewVerdict `json:"verdict"`
	Sections []string      `json:"sections"`
}

var (
	reviewHeadingRe = regexp.MustCompile(`(?m)^##\s+(.+?)\s*$`)
	reviewVerdictRe = regexp.MustCompile(`(?i)\bverdict\b\s*[:\-]?\s*(approve|revise)\b`)
)

// ParseReviewReport parses and structurally validates the report body. It returns
// an error naming the first missing mandatory section or the absent/invalid
// verdict — so the gate message is actionable.
func ParseReviewReport(body string) (ReviewReport, error) {
	present := map[string]bool{}
	for _, m := range reviewHeadingRe.FindAllStringSubmatch(body, -1) {
		present[normalizeHeading(m[1])] = true
	}
	var missing []string
	for _, s := range reviewSections {
		if !present[normalizeHeading(s)] {
			missing = append(missing, s)
		}
	}
	if len(missing) > 0 {
		return ReviewReport{}, GateError(fmt.Sprintf("review report missing mandatory section(s): %s", strings.Join(missing, ", ")))
	}
	vm := reviewVerdictRe.FindStringSubmatch(body)
	if vm == nil {
		return ReviewReport{}, GateError("review report has no verdict — add a line like `Verdict: approve` or `Verdict: revise`")
	}
	report := ReviewReport{Verdict: ReviewVerdict(strings.ToLower(vm[1]))}
	report.Sections = append(report.Sections, reviewSections...)
	return report, nil
}

func normalizeHeading(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// LatestTaskCompletion returns the most recent task FinishedAt timestamp in
// state, or the zero time when no task has completed. Used by the freshness
// check: the review must post-date the last thing it is supposed to have
// reviewed.
func LatestTaskCompletion(state *State) time.Time {
	var latest time.Time
	if state == nil {
		return latest
	}
	for _, t := range state.Tasks {
		if t.FinishedAt == nil {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, *t.FinishedAt)
		if err != nil {
			continue
		}
		if ts.After(latest) {
			latest = ts
		}
	}
	return latest
}

// ReviewGateResult is the outcome of evaluating the review gate.
type ReviewGateResult struct {
	OK      bool
	Verdict ReviewVerdict
	Fresh   bool
	Problem string
}

// EvaluateReviewGate checks the review report for a spec: existence, structural
// validity, verdict presence, and freshness relative to the latest task
// completion. reportModTime is the report file's modification time (the staleness
// signal); pass the zero time when the report is absent.
func EvaluateReviewGate(state *State, reportBody *string, reportModTime time.Time) ReviewGateResult {
	if reportBody == nil {
		return ReviewGateResult{Problem: fmt.Sprintf("no %s — run `specd review <slug>` and complete the report before approving", reviewReportName)}
	}
	report, err := ParseReviewReport(*reportBody)
	if err != nil {
		return ReviewGateResult{Problem: err.Error()}
	}
	latest := LatestTaskCompletion(state)
	fresh := latest.IsZero() || reportModTime.After(latest)
	if !fresh {
		return ReviewGateResult{Verdict: report.Verdict, Problem: fmt.Sprintf("review report is stale (dated %s) — a task completed at %s afterwards; re-review and regenerate the report", reportModTime.UTC().Format(time.RFC3339), latest.UTC().Format(time.RFC3339))}
	}
	if report.Verdict != ReviewApprove {
		return ReviewGateResult{Verdict: report.Verdict, Fresh: true, Problem: fmt.Sprintf("review verdict is %q — resolve the review (verdict must be `approve`) before completing", report.Verdict)}
	}
	return ReviewGateResult{OK: true, Verdict: report.Verdict, Fresh: true}
}

// ReadReviewReport returns the review report body and its mod time, or (nil,
// zero) when absent.
func ReadReviewReport(root, slug string) (*string, time.Time) {
	path := ArtifactPath(root, slug, reviewReportName)
	info, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}
	}
	s := string(b)
	return &s, info.ModTime()
}

// ScaffoldReviewReport returns the mandatory-section skeleton the reviewer fills
// in. Deterministic — the same spec always yields the same skeleton.
func ScaffoldReviewReport(slug string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Review report — %s\n\n", slug)
	b.WriteString("<!-- Read-only, adversarial review. Demand the artifact: cite file:line for every claim. -->\n\n")
	for _, s := range reviewSections {
		fmt.Fprintf(&b, "## %s\n\n", s)
		switch s {
		case "Verdict":
			b.WriteString("Verdict: revise\n\n_(set to `approve` only when Bugs/Security/Hallucinated Dependencies are clear)_\n\n")
		case "Hallucinated Dependencies":
			b.WriteString("_List any imported/declared dependency you could not verify exists. None → \"none found\"._\n\n")
		default:
			b.WriteString("_..._\n\n")
		}
	}
	return b.String()
}

// ReviewChecklist deterministically extracts a human review checklist from a
// spec's design.md section headings and tasks.md task contracts (id + files +
// verify). It is extraction only — zero interpretation (V8/P4.3).
func ReviewChecklist(designMd string, doc *ParsedTasks) []string {
	var out []string
	for _, m := range reviewHeadingRe.FindAllStringSubmatch(designMd, -1) {
		out = append(out, fmt.Sprintf("[ ] design: reviewed section %q", strings.TrimSpace(m[1])))
	}
	if doc != nil {
		tasks := append([]ParsedTask(nil), doc.Tasks...)
		sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
		for _, t := range tasks {
			files := strings.TrimSpace(t.Meta["files"])
			if files == "" {
				files = "(no files contract)"
			}
			out = append(out, fmt.Sprintf("[ ] task %s: changes confined to %s; verify `%s`", t.ID, files, strings.TrimSpace(t.Meta["verify"])))
		}
	}
	return out
}
