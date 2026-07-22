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

// ReviewContract is the deterministic review envelope. Hard risks are always
// visible to the auditor; required test evidence remains an independent gate.
type ReviewContract struct {
	TaskID          string
	SubjectRevision string
	RequiredTests   []string
	HardRisks       []string
}

var hardReviewRisks = []string{"integration", "error", "concurrency", "rollback"}

func BuildReviewContract(quality QualityContract, subjectRevision string, risks []string) ReviewContract {
	contract := ReviewContract{TaskID: quality.TaskID, SubjectRevision: subjectRevision, HardRisks: append([]string(nil), hardReviewRisks...)}
	for _, req := range quality.Required {
		if req.EvidenceClass == EvidenceTest {
			contract.RequiredTests = append(contract.RequiredTests, req.CheckID)
		}
	}
	seen := map[string]bool{}
	for _, risk := range contract.HardRisks {
		seen[risk] = true
	}
	for _, risk := range risks {
		if risk != "" && !seen[risk] {
			contract.HardRisks = append(contract.HardRisks, risk)
			seen[risk] = true
		}
	}
	return contract
}

// ValidateReviewContract enforces test proof separately from human review.
func ValidateReviewContract(contract ReviewContract, status QualityStatus) error {
	missing := map[string]bool{}
	for _, req := range status.Missing {
		if req.EvidenceClass == EvidenceTest {
			missing[req.CheckID] = true
		}
	}
	for _, req := range status.Stale {
		if req.EvidenceClass == EvidenceTest {
			missing[req.CheckID] = true
		}
	}
	for _, check := range contract.RequiredTests {
		if missing[check] {
			return fmt.Errorf("REVIEW_TEST_REQUIRED: %s", check)
		}
	}
	return nil
}

// ReviewReport is the field-extraction result of a review_report.md. The report
// is human-edited, so — unlike the tasks parser — the parser does not require
// byte-stability or round-tripping; it extracts three load-bearing fields and
// the findings prose. The Head field is what makes an approval a *fact about
// this code* (spec 09 R3), mirroring evidence pinning. R5.3: Verdict is a strict
// token (approve|reject|needs-changes); any following note is stored separately.
type ReviewReport struct {
	Verdict  string
	Note     string
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
		// Declared files come from the canonical projection (spec 05 R1.1) so the
		// auditor audits the same normalized, de-duplicated path set the diff-scope
		// check enforces, whatever delimiter the author used.
		files := task.Files
		if paths, err := TaskDeclaredPaths(task); err == nil && len(paths) != 0 {
			files = strings.Join(paths, ", ")
		}
		fmt.Fprintf(&b, "### %s\n\n- files: %s\n- acceptance: %s\n\n", task.ID, files, task.Acceptance)
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
// R5.3: The verdict line is parsed to extract the strict token and any following
// note separately.
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
			// R5.3: Parse verdict line as "token note" where token is strict
			parts := strings.Fields(v)
			if len(parts) > 0 {
				report.Verdict = strings.ToLower(parts[0])
				if len(parts) > 1 {
					// The token is normalized because it is matched; the note is
					// human prose and is kept exactly as written (R5.2/R5.3).
					report.Note = strings.Join(parts[1:], " ")
				}
			}
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

// RestampReviewReport updates a review report to a new git HEAD while preserving
// all human-authored body bytes. R5.2: Only machine-owned provenance (Git HEAD) is
// updated; the human findings are preserved exactly. Returns an error if the
// report cannot be parsed.
func RestampReviewReport(raw string, newHead string) (string, error) {
	// Validate the new HEAD is resolvable
	if !HeadPinned(newHead) {
		return "", fmt.Errorf("new git HEAD %q is not resolvable", newHead)
	}

	// Parse once to validate structure; extract HEAD line format
	_, err := ParseReviewReport(raw)
	if err != nil {
		return "", fmt.Errorf("existing report cannot be restamped: %v", err)
	}

	// Replace only the Git HEAD line; preserve everything else byte-for-byte
	var result []string
	for _, line := range strings.Split(raw, "\n") {
		if v, ok := fieldValue(strings.TrimSpace(line), "Git HEAD"); ok && v != "" {
			// Reconstruct the HEAD line with the same format as the original
			// Keep the original indentation and markup
			result = append(result, "- **Git HEAD:** "+newHead)
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n"), nil
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
