package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	embedtemplates "github.com/0xkhdr/specd/internal/core/embed_templates"
)

// Review verdicts. Anything else — including an unedited placeholder — fails the
// review parse and is never treated as approve (spec 09 R5, fail closed).
const (
	ReviewApprove      = "approve"
	ReviewReject       = "reject"
	ReviewNeedsChanges = "needs-changes"
)

// ReviewReport is the field-extraction result of a review_report.md. The report
// is human-edited, so — unlike the tasks parser — the parser does not require
// byte-stability or round-tripping; it extracts three load-bearing fields and
// the findings prose. The Head field is what makes an approval a *fact about
// this code* (spec 09 R3), mirroring evidence pinning.
type ReviewReport struct {
	Verdict  string
	Head     string
	Findings string
}

// ReviewReportPath is the per-spec review report the auditor role fills.
func ReviewReportPath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "review_report.md")
}

// RenderReviewScaffold builds the review report from the embedded template,
// substituting the spec slug, the git HEAD under review, and a per-task section
// (id, files, acceptance) so the reviewer sees exactly what to audit (R1).
func RenderReviewScaffold(slug, head string, tasks []TaskRow) string {
	raw, err := embedtemplates.FS.ReadFile("reports/review_report.md")
	if err != nil {
		// Embedded asset is compiled in; a read failure is a build defect.
		panic("review_report.md template missing from embed FS: " + err.Error())
	}
	var b strings.Builder
	for _, task := range tasks {
		fmt.Fprintf(&b, "### %s\n\n- files: %s\n- acceptance: %s\n\n", task.ID, task.Files, task.Acceptance)
	}
	tasksSection := strings.TrimRight(b.String(), "\n")
	if tasksSection == "" {
		tasksSection = "_(no tasks)_"
	}
	out := string(raw)
	out = strings.ReplaceAll(out, "{{SLUG}}", slug)
	out = strings.ReplaceAll(out, "{{HEAD}}", head)
	out = strings.ReplaceAll(out, "{{TASKS}}", tasksSection)
	return out
}

// ReviewReportHead extracts just the recorded Git HEAD from a review report,
// or "" if there is no resolved HEAD line. The overwrite guard (spec 09 R2) uses
// it to detect a report already scaffolded for the current commit — even before
// the auditor has filled the verdict, so a re-scaffold never clobbers in-progress
// notes.
func ReviewReportHead(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		if v, ok := fieldValue(strings.TrimSpace(line), "Git HEAD"); ok {
			if strings.HasPrefix(v, "<") {
				return ""
			}
			return v
		}
	}
	return ""
}

// ParseReviewReport extracts the verdict, HEAD, and findings from a review
// report. It is strict (R5): a missing/unknown verdict or a missing HEAD line is
// an error, never a silent approve. It is tolerant of surrounding human edits —
// it scans for the labelled fields rather than requiring a fixed byte layout.
func ParseReviewReport(raw string) (ReviewReport, error) {
	var report ReviewReport
	inFindings := false
	var findings []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inFindings = strings.EqualFold(strings.TrimSpace(trimmed[3:]), "Findings")
			continue
		}
		if inFindings {
			if strings.HasPrefix(trimmed, "<") || trimmed == "" {
				continue // placeholder comment or blank
			}
			findings = append(findings, trimmed)
			continue
		}
		if v, ok := fieldValue(trimmed, "Verdict"); ok {
			report.Verdict = strings.ToLower(v)
		}
		if v, ok := fieldValue(trimmed, "Git HEAD"); ok {
			report.Head = v
		}
	}
	report.Findings = strings.Join(findings, "\n")

	if report.Head == "" || strings.HasPrefix(report.Head, "<") {
		return ReviewReport{}, errors.New("review report has no Git HEAD line")
	}
	switch report.Verdict {
	case ReviewApprove, ReviewReject, ReviewNeedsChanges:
	case "":
		return ReviewReport{}, errors.New("review report has no verdict")
	default:
		return ReviewReport{}, fmt.Errorf("review report verdict %q is not one of approve|reject|needs-changes", report.Verdict)
	}
	return report, nil
}

// fieldValue extracts the value of a "- **Label:** value" or "Label: value"
// bullet line, stripping list markers and bold markup. It reports whether the
// line carried the label at all.
func fieldValue(line, label string) (string, bool) {
	s := strings.TrimLeft(line, "-*+ ")
	s = strings.ReplaceAll(s, "**", "")
	prefix := label + ":"
	if !strings.HasPrefix(s, prefix) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(s, prefix)), true
}
