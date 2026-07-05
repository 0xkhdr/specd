package core

import (
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Annotations is worker-reported cost telemetry attached verbatim to a record.
// The doctrine is "stored, never computed" (spec 10 R1): specd accepts these
// values from the host worker and never estimates, derives, or counts tokens
// itself. Every field is optional (R5) — a worker that cannot report cost still
// produces valid records. Cost is a decimal string with currency-agnostic
// semantics; aggregation uses exact rational math, never float64 (R6).
type Annotations struct {
	Tokens     int    `json:"tokens,omitempty"`
	Cost       string `json:"cost,omitempty"`
	DurationMs int    `json:"duration_ms,omitempty"`
}

// decimalPattern is a non-negative decimal: digits, optional fractional part.
// It deliberately rejects big.Rat's looser forms (fractions, exponents, signs)
// so cost stays an honest money string.
var decimalPattern = regexp.MustCompile(`^\d+(\.\d+)?$`)

// ParseAnnotations reads the optional --tokens/--cost/--duration-ms flags. It
// returns nil (no telemetry) when none are present, and a validation error when
// any is malformed — the caller maps that to a fail-closed exit 2 (R2). Values
// are stored verbatim; nothing here computes a cost.
func ParseAnnotations(tokens, cost, durationMs string, present func(string) bool) (*Annotations, error) {
	hasTokens := present("tokens")
	hasCost := present("cost")
	hasDuration := present("duration-ms")
	if !hasTokens && !hasCost && !hasDuration {
		return nil, nil
	}
	ann := &Annotations{}
	if hasTokens {
		n, err := strconv.Atoi(strings.TrimSpace(tokens))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("--tokens must be a non-negative integer, got %q", tokens)
		}
		ann.Tokens = n
	}
	if hasDuration {
		n, err := strconv.Atoi(strings.TrimSpace(durationMs))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("--duration-ms must be a non-negative integer, got %q", durationMs)
		}
		ann.DurationMs = n
	}
	if hasCost {
		c := strings.TrimSpace(cost)
		if !decimalPattern.MatchString(c) {
			return nil, fmt.Errorf("--cost must be a non-negative decimal, got %q", cost)
		}
		ann.Cost = c
	}
	return ann, nil
}

// TaskTelemetry is one task's telemetry across every recorded attempt. Absence
// is explicit: HasTelemetry is false when no attempt carried annotations, so the
// report shows missing data rather than imputing zero (R4).
type TaskTelemetry struct {
	TaskID       string        `json:"task_id"`
	Attempts     []Annotations `json:"attempts,omitempty"`
	Tokens       int           `json:"tokens"`
	Cost         string        `json:"cost"`
	DurationMs   int           `json:"duration_ms"`
	HasTelemetry bool          `json:"has_telemetry"`
}

// TelemetryReport aggregates annotations per spec and per task. Totals are exact
// (integer sums for tokens/duration, rational sum for cost). Missing lists the
// tasks with no telemetry at all.
type TelemetryReport struct {
	Tokens     int             `json:"tokens"`
	Cost       string          `json:"cost"`
	DurationMs int             `json:"duration_ms"`
	Tasks      []TaskTelemetry `json:"tasks,omitempty"`
	Missing    []string        `json:"missing,omitempty"`
}

// AggregateTelemetry folds every evidence record's annotations into per-task and
// per-spec totals. records is the full append-order evidence log (one entry per
// attempt), so per-attempt breakdown is preserved. taskOrder fixes the task
// ordering for deterministic output; a task with no annotated attempt is listed
// in Missing and never imputed a zero cost.
func AggregateTelemetry(records []EvidenceRecord, taskOrder []string) TelemetryReport {
	byTask := map[string][]Annotations{}
	for _, rec := range records {
		if rec.Telemetry != nil {
			byTask[rec.TaskID] = append(byTask[rec.TaskID], *rec.Telemetry)
		}
	}

	// Task ordering: the caller's order first, then any annotated task not in it.
	seen := map[string]bool{}
	order := make([]string, 0, len(taskOrder)+len(byTask))
	for _, id := range taskOrder {
		if !seen[id] {
			order = append(order, id)
			seen[id] = true
		}
	}
	extras := make([]string, 0, len(byTask))
	for id := range byTask {
		if !seen[id] {
			extras = append(extras, id)
		}
	}
	sort.Strings(extras)
	order = append(order, extras...)

	report := TelemetryReport{}
	total := new(big.Rat)
	for _, id := range order {
		attempts := byTask[id]
		task := TaskTelemetry{TaskID: id}
		if len(attempts) == 0 {
			report.Missing = append(report.Missing, id)
			report.Tasks = append(report.Tasks, task)
			continue
		}
		task.HasTelemetry = true
		task.Attempts = attempts
		taskCost := new(big.Rat)
		for _, ann := range attempts {
			task.Tokens += ann.Tokens
			task.DurationMs += ann.DurationMs
			if ann.Cost != "" {
				if r, ok := new(big.Rat).SetString(ann.Cost); ok {
					taskCost.Add(taskCost, r)
					total.Add(total, r)
				}
			}
		}
		task.Cost = formatRat(taskCost)
		report.Tokens += task.Tokens
		report.DurationMs += task.DurationMs
		report.Tasks = append(report.Tasks, task)
	}
	report.Cost = formatRat(total)
	return report
}

// formatRat renders a rational money value as an exact decimal string, trimming
// trailing zeros. Money is terminating decimal, so 10 places is lossless here;
// this never touches float64 (R6).
func formatRat(r *big.Rat) string {
	s := r.FloatString(10)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" {
		return "0"
	}
	return s
}

// RenderTelemetry formats a telemetry report for `report --metrics`, in the same
// Prometheus-flavored style as the task-count metrics. Tasks without telemetry
// are emitted as an explicit gauge so absence is visible, never imputed (R4).
func RenderTelemetry(slug string, report TelemetryReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "specd_cost_tokens_total{spec=%q} %d\n", slug, report.Tokens)
	fmt.Fprintf(&b, "specd_cost_duration_ms_total{spec=%q} %d\n", slug, report.DurationMs)
	fmt.Fprintf(&b, "specd_cost_total{spec=%q} %s\n", slug, report.Cost)
	for _, task := range report.Tasks {
		if !task.HasTelemetry {
			fmt.Fprintf(&b, "specd_task_telemetry_present{spec=%q,task=%q} 0\n", slug, task.TaskID)
			continue
		}
		fmt.Fprintf(&b, "specd_task_telemetry_present{spec=%q,task=%q} 1\n", slug, task.TaskID)
		fmt.Fprintf(&b, "specd_task_cost_tokens{spec=%q,task=%q} %d\n", slug, task.TaskID, task.Tokens)
		fmt.Fprintf(&b, "specd_task_cost_duration_ms{spec=%q,task=%q} %d\n", slug, task.TaskID, task.DurationMs)
		fmt.Fprintf(&b, "specd_task_cost{spec=%q,task=%q} %s\n", slug, task.TaskID, task.Cost)
		fmt.Fprintf(&b, "specd_task_attempts{spec=%q,task=%q} %d\n", slug, task.TaskID, len(task.Attempts))
	}
	return b.String()
}
