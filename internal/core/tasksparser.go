package core

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"
)

type TaskRow struct {
	ID     string
	Marker string
	Role   string
	Files  string
	// DeclaredFiles is the canonical, sorted, de-duplicated projection of Files.
	// Files remains untouched so tasks.md round-trips byte-for-byte.
	DeclaredFiles []string
	DependsOn     []string
	Verify        string
	Acceptance    string
	// Trace/risk planning metadata (spec 01 R3.1). Parsed from optional named
	// columns; absent columns yield zero values so legacy 6-column tasks.md
	// files parse unchanged (backward compatible). The task-trace gate requires
	// them only under the production planning profile.
	Refs     []string // requirement/design references this task implements
	Kind     string   // work kind (e.g. feature, fix, refactor, docs)
	Risk     string   // risk tier
	Context  string   // required context declaration
	Evidence string   // evidence classes planned
	Checks   string   // negative/edge checks planned
}

type TaskRunStatus string

const (
	TaskPending  TaskRunStatus = "pending"
	TaskRunning  TaskRunStatus = "running"
	TaskComplete TaskRunStatus = "complete"
	TaskBlocked  TaskRunStatus = "blocked"
)

func ParseTasksMd(raw []byte) (TasksMd, error) {
	doc := TasksMd{Raw: append([]byte(nil), raw...)}
	seen := map[string]bool{}
	tables, err := parseMarkdownTables(raw, func(header []string, rows [][]string) error {
		indexes, ok := validateTaskHeader(header)
		if !ok {
			return nil
		}
		for _, cells := range rows {
			marker, id := splitMarkedTaskID(cell(cells, indexes["id"]))
			if id == "" {
				continue
			}
			if seen[id] {
				return formatDuplicateTask(id)
			}
			seen[id] = true
			files := cell(cells, indexes["files"])
			declaredFiles, err := normalizeDeclaredFiles(files)
			if err != nil {
				return fmt.Errorf("task %s files: %w", id, err)
			}
			doc.Tasks = append(doc.Tasks, TaskRow{
				ID:            id,
				Marker:        marker,
				Role:          cell(cells, indexes["role"]),
				Files:         files,
				DeclaredFiles: declaredFiles,
				DependsOn:     splitTaskList(cell(cells, indexes["depends-on"])),
				Verify:        strings.Trim(cell(cells, indexes["verify"]), "`"),
				Acceptance:    cell(cells, indexes["acceptance"]),
				// Optional trace/risk columns (spec 01 R3.1). headerIndex returns
				// -1 for a column the header omits, which cell() reads as empty.
				Refs:     splitTaskList(cell(cells, headerIndex(header, "refs"))),
				Kind:     cell(cells, headerIndex(header, "kind")),
				Risk:     cell(cells, headerIndex(header, "risk")),
				Context:  cell(cells, headerIndex(header, "context")),
				Evidence: cell(cells, headerIndex(header, "evidence")),
				Checks:   cell(cells, headerIndex(header, "checks")),
			})
		}
		return nil
	})
	if err != nil {
		return TasksMd{}, err
	}
	doc.Tables = tables
	return doc, nil
}

func normalizeDeclaredFiles(raw string) ([]string, error) {
	seen := map[string]bool{}
	for _, value := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' }) {
		value = strings.TrimSpace(value)
		if value == "" || value == "-" {
			continue
		}
		value = strings.ReplaceAll(value, "\\", "/")
		clean := path.Clean(value)
		if path.IsAbs(clean) || (len(clean) >= 2 && clean[1] == ':') || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("declared path %q escapes repository base", value)
		}
		seen[clean] = true
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out, nil
}

func RewriteTaskStatusLine(raw []byte, id string, marker string) ([]byte, error) {
	lines := bytes.SplitAfter(raw, []byte{'\n'})
	out := make([]byte, 0, len(raw)+len(marker)+1)
	changed := false
	for _, line := range lines {
		trimmed := strings.TrimRight(string(line), "\n")
		cells := parsePipeRow(trimmed)
		if cells == nil {
			out = append(out, line...)
			continue
		}
		_, taskID := splitMarkedTaskID(cells[0])
		if taskID != id {
			out = append(out, line...)
			continue
		}
		if changed {
			return nil, fmt.Errorf("duplicate task id %q", id)
		}
		cells[0] = strings.TrimSpace(marker + " " + id)
		newLine := "| " + strings.Join(cells, " | ") + " |"
		if bytes.HasSuffix(line, []byte{'\n'}) {
			newLine += "\n"
		}
		out = append(out, newLine...)
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("task %s not found", id)
	}
	return out, nil
}

// knownRiskTiers are the accepted risk-tier values a task may declare (spec 01
// R3.1). An unrecognized tier is always refused so a typo cannot silently pass.
var knownRiskTiers = map[string]bool{"low": true, "medium": true, "high": true, "critical": true}

// TaskTraceFinding is an addressable task-planning defect (spec 01 R3.1). TaskID
// names the offending task.
type TaskTraceFinding struct {
	TaskID  string
	Message string
}

// ValidateTaskTrace reports task trace/risk defects. A declared requirement
// reference (R<n>/R<n>.<m>) that does not resolve, and an unrecognized risk
// tier, are always refused (safety). When requireTrace is set — the production
// planning profile (spec 01 R7.2) — every task must additionally declare its
// references, work kind, risk tier, required context, evidence classes, and
// negative/edge checks (R3.1); under the default profile these are optional so
// legacy tasks.md files keep planning (R7.1). Design-component references (ids
// that are not R<n> shaped) are accepted here; resolving them against a design
// component registry is deferred to a later wave. Pure: no disk, no clock.
func ValidateTaskTrace(tasks []TaskRow, knownReqIDs map[string]bool, requireTrace bool) []TaskTraceFinding {
	var findings []TaskTraceFinding
	for _, task := range tasks {
		for _, ref := range task.Refs {
			if !reReqRefToken.MatchString(ref) {
				continue
			}
			if !knownReqIDs[ref] && !knownReqIDs[requirementOf(ref)] {
				findings = append(findings, TaskTraceFinding{TaskID: task.ID, Message: task.ID + " references unknown requirement " + ref})
			}
		}
		if task.Risk != "" && !knownRiskTiers[strings.ToLower(task.Risk)] {
			findings = append(findings, TaskTraceFinding{TaskID: task.ID, Message: task.ID + " has unknown risk tier " + task.Risk})
		}
		if requireTrace {
			for _, field := range missingTraceFields(task) {
				findings = append(findings, TaskTraceFinding{TaskID: task.ID, Message: task.ID + " must declare " + field})
			}
		}
	}
	return findings
}

// missingTraceFields lists the trace/risk fields a task has not declared (spec
// 01 R3.1). A read-only scout task is exempt from evidence-class and edge-check
// declaration — it produces a finding, not a change — so only work-bearing
// tasks must declare the full contract.
func missingTraceFields(task TaskRow) []string {
	var missing []string
	if len(task.Refs) == 0 {
		missing = append(missing, "refs")
	}
	if task.Kind == "" {
		missing = append(missing, "kind")
	}
	if task.Risk == "" {
		missing = append(missing, "risk")
	}
	if task.Context == "" {
		missing = append(missing, "context")
	}
	if IsWriteRole(task.Role) {
		if task.Evidence == "" {
			missing = append(missing, "evidence")
		}
		if task.Checks == "" {
			missing = append(missing, "checks")
		}
	}
	return missing
}

func splitMarkedTaskID(value string) (string, string) {
	value = strings.Trim(strings.TrimSpace(value), "`")
	if value == "" {
		return "", ""
	}
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return "", ""
	}
	for i, part := range parts {
		part = strings.Trim(part, "`")
		if strings.HasPrefix(part, "T") {
			marker := strings.Join(parts[:i], " ")
			return marker, part
		}
	}
	if strings.HasPrefix(parts[0], "T") {
		return "", parts[0]
	}
	return "", value
}
