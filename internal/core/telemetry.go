package core

import (
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Telemetry envelope schema versions (spec 07 R1.1). An empty EnvelopeVersion
// marks a legacy, pre-envelope record: it decodes and re-encodes byte-for-byte
// unchanged and is validated leniently for backward compatibility. A canonical
// record carries TelemetryEnvelopeV1 and is held to the strict v1 contract
// (R1.2/R1.3). Any other, unrecognized version is a *required* schema mismatch
// and fails closed.
const TelemetryEnvelopeV1 = "v1"

// Telemetry provenance (spec 07 R1.3): who reported the measured values. specd
// stores worker numbers verbatim and never presents them as independently
// measured — every report renders them "as reported" against this label.
const (
	TelemetrySourceWorker   = "worker"
	TelemetrySourceAdapter  = "provider_adapter"
	TelemetrySourceOperator = "operator"
)

// Annotations is worker-reported cost telemetry attached verbatim to a record.
// The doctrine is "stored, never computed" (spec 10 R1): specd accepts these
// values from the host worker and never estimates, derives, or counts tokens
// itself. Every field is optional (R5) — a worker that cannot report cost still
// produces valid records. Cost is a decimal string with currency-agnostic
// semantics; aggregation uses exact rational math, never float64 (R6).
//
// The envelope fields (spec 07 R1) are all omitempty so a legacy record (bare
// tokens/cost/duration) decodes unchanged and re-encodes byte-identically,
// while a canonical record round-trips its version and provenance byte-stably.
type Annotations struct {
	Tokens       int    `json:"tokens,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
	CachedTokens int    `json:"cached_tokens,omitempty"`
	Cost         string `json:"cost,omitempty"`
	DurationMs   int    `json:"duration_ms,omitempty"`

	// Source is the trust provenance (worker|provider_adapter|operator, R1.3).
	// Currency pairs with Cost on canonical records (R1.2). AttestationRef is an
	// external, always-optional pointer to a provider attestation (R1.3).
	// EnvelopeVersion marks the schema; empty means legacy (grandfathered).
	Source          string `json:"telemetry_source,omitempty"`
	Currency        string `json:"currency,omitempty"`
	PricingRef      string `json:"pricing_ref,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	AttestationRef  string `json:"attestation_ref,omitempty"`
	EnvelopeVersion string `json:"envelope_version,omitempty"`
}

// ValidateAnnotations enforces the telemetry envelope's decode/persist rules
// (spec 07 R1.2). A nil record is valid (absence is honest, never zero). A
// legacy record (no EnvelopeVersion) is grandfathered so old ledgers keep
// decoding (R1.1) — only an outright malformed decimal or negative unit is
// rejected. A canonical v1 record is held to the full contract: a known
// provenance, and cost paired with a currency. Any unrecognized required
// version fails closed. Optional fields left absent stay absent.
func ValidateAnnotations(a *Annotations) error {
	if a == nil {
		return nil
	}
	if a.EnvelopeVersion != "" && a.EnvelopeVersion != TelemetryEnvelopeV1 {
		return fmt.Errorf("unknown telemetry envelope version %q", a.EnvelopeVersion)
	}
	if a.Cost != "" && !decimalPattern.MatchString(a.Cost) {
		return fmt.Errorf("telemetry cost %q is not a non-negative decimal", a.Cost)
	}
	if a.Tokens < 0 {
		return fmt.Errorf("telemetry tokens %d is negative", a.Tokens)
	}
	if a.InputTokens < 0 || a.OutputTokens < 0 || a.CachedTokens < 0 {
		return fmt.Errorf("telemetry token categories must be non-negative")
	}
	categoryTotal := a.InputTokens + a.OutputTokens + a.CachedTokens
	if categoryTotal > 0 && a.Tokens > 0 && categoryTotal != a.Tokens {
		return fmt.Errorf("telemetry tokens %d contradict category total %d", a.Tokens, categoryTotal)
	}
	if a.DurationMs < 0 {
		return fmt.Errorf("telemetry duration_ms %d is negative", a.DurationMs)
	}
	if a.EnvelopeVersion == "" {
		return nil // legacy: grandfathered, no v1 strictness
	}
	switch a.Source {
	case TelemetrySourceWorker, TelemetrySourceAdapter, TelemetrySourceOperator:
	case "":
		return fmt.Errorf("canonical telemetry envelope requires telemetry_source")
	default:
		return fmt.Errorf("unknown telemetry_source %q", a.Source)
	}
	if a.Cost != "" && a.Currency == "" {
		return fmt.Errorf("telemetry cost present without currency")
	}
	if a.Cost != "" && a.PricingRef == "" {
		return fmt.Errorf("telemetry cost present without pricing_ref")
	}
	for name, value := range map[string]string{"provider": a.Provider, "model": a.Model} {
		if value != "" && !boundedIdentifier.MatchString(value) {
			return fmt.Errorf("telemetry %s %q is not a bounded identifier", name, value)
		}
	}
	return nil
}

// decimalPattern is a non-negative decimal: digits, optional fractional part.
// It deliberately rejects big.Rat's looser forms (fractions, exponents, signs)
// so cost stays an honest money string.
var decimalPattern = regexp.MustCompile(`^\d+(\.\d+)?$`)
var boundedIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,63}$`)

// ParseAnnotationFlags parses the additive provider-neutral flag set. New
// fields opt into canonical v1 validation; legacy-only flags retain old shape.
func ParseAnnotationFlags(values map[string]string, present func(string) bool) (*Annotations, error) {
	legacy, err := ParseAnnotations(values["tokens"], values["cost"], values["duration-ms"], present)
	if err != nil {
		return nil, err
	}
	newFields := []string{"input-tokens", "output-tokens", "cached-tokens", "provider", "model", "currency", "pricing-ref", "telemetry-source", "attestation-ref"}
	hasNew := false
	for _, name := range newFields {
		if present(name) {
			hasNew = true
			break
		}
	}
	if legacy == nil && !hasNew {
		return nil, nil
	}
	if legacy == nil {
		legacy = &Annotations{}
	}
	for flag, target := range map[string]*int{"input-tokens": &legacy.InputTokens, "output-tokens": &legacy.OutputTokens, "cached-tokens": &legacy.CachedTokens} {
		if !present(flag) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(values[flag]))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("--%s must be a non-negative integer, got %q", flag, values[flag])
		}
		*target = n
	}
	if hasNew {
		legacy.EnvelopeVersion = TelemetryEnvelopeV1
		legacy.Source = strings.TrimSpace(values["telemetry-source"])
		if legacy.Source == "" {
			legacy.Source = TelemetrySourceWorker
		}
		legacy.Currency = strings.TrimSpace(values["currency"])
		legacy.PricingRef = strings.TrimSpace(values["pricing-ref"])
		legacy.Provider = strings.TrimSpace(values["provider"])
		legacy.Model = strings.TrimSpace(values["model"])
		legacy.AttestationRef = strings.TrimSpace(values["attestation-ref"])
	}
	if err := ValidateAnnotations(legacy); err != nil {
		return nil, err
	}
	return legacy, nil
}

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
	InputTokens  int           `json:"input_tokens"`
	OutputTokens int           `json:"output_tokens"`
	CachedTokens int           `json:"cached_tokens"`
	Cost         string        `json:"cost"`
	// Source is the telemetry provenance surfaced so a report renders the values
	// as reported, never as independently measured (spec 07 R1.3). A legacy
	// attempt carries no explicit source and is reported as worker-reported.
	Source       string `json:"telemetry_source,omitempty"`
	DurationMs   int    `json:"duration_ms"`
	HasTelemetry bool   `json:"has_telemetry"`
}

// TelemetryReport aggregates annotations per spec and per task. Totals are exact
// (integer sums for tokens/duration, rational sum for cost). Missing lists the
// tasks with no telemetry at all.
type TelemetryReport struct {
	Tokens       int             `json:"tokens"`
	InputTokens  int             `json:"input_tokens"`
	OutputTokens int             `json:"output_tokens"`
	CachedTokens int             `json:"cached_tokens"`
	Cost         string          `json:"cost"`
	DurationMs   int             `json:"duration_ms"`
	Tasks        []TaskTelemetry `json:"tasks,omitempty"`
	Missing      []string        `json:"missing,omitempty"`
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
		task.Source = telemetrySource(attempts)
		taskCost := new(big.Rat)
		for _, ann := range attempts {
			task.Tokens += ann.Tokens
			task.InputTokens += ann.InputTokens
			task.OutputTokens += ann.OutputTokens
			task.CachedTokens += ann.CachedTokens
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
		report.InputTokens += task.InputTokens
		report.OutputTokens += task.OutputTokens
		report.CachedTokens += task.CachedTokens
		report.DurationMs += task.DurationMs
		report.Tasks = append(report.Tasks, task)
	}
	report.Cost = formatRat(total)
	return report
}

// telemetrySource reports the provenance of a task's telemetry (spec 07 R1.3):
// the explicit source when every annotated attempt agrees, "mixed" when they
// diverge, and "worker" when none is set (legacy attempts are worker-reported).
func telemetrySource(attempts []Annotations) string {
	src := ""
	for _, a := range attempts {
		s := a.Source
		if s == "" {
			s = TelemetrySourceWorker
		}
		if src == "" {
			src = s
		} else if src != s {
			return "mixed"
		}
	}
	if src == "" {
		return TelemetrySourceWorker
	}
	return src
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
	// Provenance is a comment, never a metric label — the label allowlist stays
	// spec/task/status only (spec 07 R5 cardinality). Values are rendered as
	// reported by the worker, not as independently measured (R1.3).
	fmt.Fprintf(&b, "# specd telemetry is worker-reported, not independently measured\n")
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
