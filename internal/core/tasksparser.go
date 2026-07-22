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
	// columns; absent columns yield zero values so a minimal 6-column tasks.md
	// file parses unchanged. The task-trace gate requires
	// them only under the production planning profile.
	Refs         []string // requirement/design references this task implements
	Kind         string   // work kind (e.g. feature, fix, refactor, docs)
	Risk         string   // risk tier
	Complexity   string   // operator-declared routing complexity
	Capabilities []string // required provider-neutral capability ids
	Context      string   // required context declaration
	Evidence     string   // evidence classes planned
	Checks       string   // negative/edge checks planned
}

type TaskRunStatus string

const (
	TaskPending  TaskRunStatus = "pending"
	TaskRunning  TaskRunStatus = "running"
	TaskComplete TaskRunStatus = "complete"
	TaskBlocked  TaskRunStatus = "blocked"
)

// TaskActivity is what a task *is* — accepted, attempted, or terminally
// disposed — as distinct from whether it may start (Readiness, spec 03 R3.1).
// The legacy tasks.md marker stays the activity view; states no marker can
// express are carried by TaskFacts.Activity, so tasks.md remains byte-stable.
type TaskActivity string

const (
	ActivityDraft      TaskActivity = "draft"
	ActivityPending    TaskActivity = "pending"
	ActivityInProgress TaskActivity = "in_progress"
	ActivityPaused     TaskActivity = "paused"
	ActivityBlocked    TaskActivity = "blocked"
	ActivityFailed     TaskActivity = "failed"
	ActivityCompleted  TaskActivity = "completed"
	ActivityCancelled  TaskActivity = "cancelled"
	ActivitySuperseded TaskActivity = "superseded"
)

// ActivityFromStatus projects the legacy run status (itself the marker view)
// onto the canonical activity. Unknown or empty status is an accepted task with
// no attempt and no disposition: pending.
func ActivityFromStatus(status TaskRunStatus) TaskActivity {
	switch status {
	case TaskComplete:
		return ActivityCompleted
	case TaskRunning:
		return ActivityInProgress
	case TaskBlocked:
		return ActivityBlocked
	default:
		return ActivityPending
	}
}

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
				DependsOn:     splitCanonical(cell(cells, indexes["depends-on"])),
				Verify:        strings.Trim(cell(cells, indexes["verify"]), "`"),
				Acceptance:    cell(cells, indexes["acceptance"]),
				// Optional trace/risk columns (spec 01 R3.1). headerIndex returns
				// -1 for a column the header omits, which cell() reads as empty.
				Refs:         splitCanonical(cell(cells, headerIndex(header, "refs"))),
				Kind:         cell(cells, headerIndex(header, "kind")),
				Risk:         cell(cells, headerIndex(header, "risk")),
				Complexity:   cell(cells, headerIndex(header, "complexity")),
				Capabilities: sortedUnique(splitCanonical(cell(cells, headerIndex(header, "capabilities")))),
				Context:      cell(cells, headerIndex(header, "context")),
				Evidence:     cell(cells, headerIndex(header, "evidence")),
				Checks:       cell(cells, headerIndex(header, "checks")),
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

func sortedUnique(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
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
// minimal tasks.md files keep planning (R7.1). Design-component references (ids
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

// Canonical task-field vocabularies (spec 05 R1.4). One registry per typed
// column, shared by planning gates, routing, review, mission authority, and the
// tasks scaffold, so a value one consumer accepts cannot be refused by another.
var (
	knownTaskKinds = map[string]bool{
		"feature": true, "fix": true, "refactor": true, "docs": true,
		"test": true, "chore": true, "spike": true, "deferred": true,
	}
	// knownTaskCapabilities is the provider-neutral capability vocabulary a task
	// row may require. It is the same set the default routing class supplies and
	// the same set RouteTask escalates to for high/critical risk — parity is
	// pinned by TestTaskContractConformance.
	knownTaskCapabilities = map[string]bool{"context": true, "eval": true, "review": true, "sandbox": true}
)

// DeferredTaskKind marks a task row that records a deliberate deferral rather
// than work to dispatch. Coverage analysis treats its refs as intentionally
// uncovered, so it carries no evidence or edge-check obligation.
const DeferredTaskKind = "deferred"

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// CanonicalTaskCapabilities returns the sorted capability vocabulary shared by
// task rows, routing class capabilities, and the tasks scaffold example.
func CanonicalTaskCapabilities() []string { return sortedKeys(knownTaskCapabilities) }

// CanonicalTaskKinds returns the sorted work-kind vocabulary.
func CanonicalTaskKinds() []string { return sortedKeys(knownTaskKinds) }

// CanonicalRiskTiers returns the sorted risk-tier vocabulary.
func CanonicalRiskTiers() []string { return sortedKeys(knownRiskTiers) }

// SplitTaskField splits one list-shaped task cell into canonical tokens. The
// canonical delimiter is the comma; `;` is a legacy delimiter that is still
// normalized, and legacy reports whether it was used so the caller can attach a
// stable deprecation warning (spec 05 R1.3). Every list-shaped column goes
// through here so consumers cannot disagree about delimiters.
func SplitTaskField(value string) (items []string, legacy bool) {
	return splitCanonical(value), strings.ContainsRune(value, ';')
}

// splitCanonical normalizes the legacy `;` separator onto the canonical `,`
// before the shared list splitter runs, so every list-shaped column — declared
// files, refs, depends-on, capabilities, context, evidence, checks — agrees on
// one delimiter set.
func splitCanonical(value string) []string {
	return splitTaskList(strings.ReplaceAll(value, ";", ","))
}

// TaskDeclaredPaths is the canonical declared-file projection of a task row:
// the parsed DeclaredFiles when the row came from ParseTasksMd, otherwise the
// same normalization applied to the raw cell.
func TaskDeclaredPaths(task TaskRow) ([]string, error) {
	if len(task.DeclaredFiles) != 0 {
		return task.DeclaredFiles, nil
	}
	return normalizeDeclaredFiles(task.Files)
}

// TaskContract is the single typed projection of one task row (spec 05 R1.1).
// Planning gates, routing, evidence policy, review, and docs read these fields
// instead of splitting the raw cells themselves.
type TaskContract struct {
	TaskID       string          `json:"task_id"`
	Role         string          `json:"role,omitempty"`
	Kind         string          `json:"kind,omitempty"`
	Risk         string          `json:"risk,omitempty"`
	Complexity   string          `json:"complexity,omitempty"`
	Deferred     bool            `json:"deferred,omitempty"`
	WriteRole    bool            `json:"write_role"`
	OutputPaths  []string        `json:"output_paths,omitempty"`
	Context      []string        `json:"context,omitempty"`
	DependsOn    []string        `json:"depends_on,omitempty"`
	Capabilities []string        `json:"capabilities,omitempty"`
	Refs         []string        `json:"refs,omitempty"`
	Verify       string          `json:"verify,omitempty"`
	Acceptance   string          `json:"acceptance,omitempty"`
	Quality      QualityContract `json:"quality"`
	Checks       []string        `json:"checks,omitempty"`
	// Warnings carry deterministic deprecation notices (legacy delimiters). They
	// never block: an unambiguous legacy spelling is normalized, not refused.
	Warnings []string `json:"warnings,omitempty"`
}

// taskFieldUnknown is the one refusal shape for an unrecognized typed value: it
// names the task id, the column, the offending value, and the accepted set, so
// the author can repair the cell without a second lookup (spec 05 R1.3).
func taskFieldUnknown(id, column, value string, accepted []string) error {
	return fmt.Errorf("TASK_FIELD_UNKNOWN: task %s column %s value %q is not one of %s", id, column, value, strings.Join(accepted, ", "))
}

func taskFieldLegacy(id, column string) string {
	return fmt.Sprintf("TASK_FIELD_LEGACY_DELIMITER: task %s column %s uses ';'; the canonical delimiter is ','", id, column)
}

// ParseTaskContract parses every typed field of a task row exactly once. Pure:
// no disk, no clock. Unknown values in a closed vocabulary fail against the task
// id and column; unambiguous legacy delimiters are normalized with a warning.
func ParseTaskContract(task TaskRow) (TaskContract, error) {
	c := TaskContract{
		TaskID:     task.ID,
		Role:       task.Role,
		Kind:       strings.ToLower(strings.TrimSpace(task.Kind)),
		Risk:       strings.ToLower(strings.TrimSpace(task.Risk)),
		Complexity: strings.TrimSpace(task.Complexity),
		Verify:     task.Verify,
		Acceptance: task.Acceptance,
		WriteRole:  IsWriteRole(task.Role),
		DependsOn:  task.DependsOn,
		Refs:       task.Refs,
	}
	if c.Kind != "" && !knownTaskKinds[c.Kind] {
		return TaskContract{}, taskFieldUnknown(task.ID, "kind", task.Kind, CanonicalTaskKinds())
	}
	c.Deferred = c.Kind == DeferredTaskKind
	if c.Risk != "" && !knownRiskTiers[c.Risk] {
		return TaskContract{}, taskFieldUnknown(task.ID, "risk", task.Risk, CanonicalRiskTiers())
	}
	paths, err := TaskDeclaredPaths(task)
	if err != nil {
		return TaskContract{}, fmt.Errorf("task %s column files: %w", task.ID, err)
	}
	c.OutputPaths = paths
	if strings.ContainsRune(task.Files, ';') {
		c.Warnings = append(c.Warnings, taskFieldLegacy(task.ID, "files"))
	}
	for _, capability := range task.Capabilities {
		if !knownTaskCapabilities[capability] {
			return TaskContract{}, taskFieldUnknown(task.ID, "capabilities", capability, CanonicalTaskCapabilities())
		}
	}
	c.Capabilities = task.Capabilities
	context, legacyContext := SplitTaskField(task.Context)
	if legacyContext {
		c.Warnings = append(c.Warnings, taskFieldLegacy(task.ID, "context"))
	}
	c.Context = context
	quality, err := ParseQualityContract(task)
	if err != nil {
		return TaskContract{}, fmt.Errorf("task %s column evidence: %w", task.ID, err)
	}
	c.Quality = quality
	c.Checks = quality.Checks
	if strings.ContainsRune(task.Evidence, ';') {
		c.Warnings = append(c.Warnings, taskFieldLegacy(task.ID, "evidence"))
	}
	if strings.ContainsRune(task.Checks, ';') {
		c.Warnings = append(c.Warnings, taskFieldLegacy(task.ID, "checks"))
	}
	sort.Strings(c.Warnings)
	return c, nil
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
