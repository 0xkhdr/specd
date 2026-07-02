package core

import (
	"fmt"
	"sort"
	"strings"
)

// escalation.go is the deterministic auto-escalation engine (V7/P3.2). Like every
// other decision surface in the binary it is pure over countable facts — no LLM,
// no interpretation. When a task's recorded facts cross a configured threshold the
// engine fires a *named rule*; the caller then pauses the task, records the
// EscalationRecord, and flips the mode recommendation to conductor. The engine
// NEVER switches mode itself — a human resolves via `specd mode --set conductor`
// or `specd orchestrate resume --override`. The Foundational Split holds: the
// harness supplies a reproducible signal, the human decides.

// EscalationFacts are the countable inputs the rules evaluate. Every field is a
// plain count/measure taken from state, ledgers, or the router policy — nothing
// derived from prose. Two callers assembling the same facts always get the same
// verdict.
type EscalationFacts struct {
	Task            string  `json:"task"`
	VerifyFailCount int     `json:"verifyFailCount"`
	RetryCount      int     `json:"retryCount"`
	MaxRetries      int     `json:"maxRetries"`
	BlockerCount    int     `json:"blockerCount"`
	CostUSD         float64 `json:"costUSD"`
	TierBudgetUSD   float64 `json:"tierBudgetUSD"`
	ComplexityScore int     `json:"complexityScore"`
}

// EscalationConfig holds the tunable thresholds. All are additive with
// backward-compatible zero values: the whole engine is opt-in via Enabled, so a
// migrated repo (Enabled=false) never escalates — the documented rollback path
// (spec §5) is "disable escalation via config thresholds".
type EscalationConfig struct {
	// Enabled is the master switch. Off by default: escalation is opt-in per repo
	// (invariant 9 — migrated repos default-off for new gates).
	Enabled bool `json:"enabled,omitempty"`
	// VerifyFailThreshold fires the "verify-fail" rule when a task has this many
	// or more failed verify records. 0 falls back to defaultVerifyFailThreshold.
	VerifyFailThreshold int `json:"verifyFailThreshold,omitempty"`
	// BlockerThreshold fires the "blocker" rule at this many open blockers. 0
	// falls back to defaultBlockerThreshold.
	BlockerThreshold int `json:"blockerThreshold,omitempty"`
	// ComplexityThreshold fires the "complexity" rule at this score. 0 disables
	// the rule (there is no meaningful default complexity ceiling).
	ComplexityThreshold int `json:"complexityThreshold,omitempty"`
}

const (
	defaultVerifyFailThreshold = 2
	defaultBlockerThreshold    = 1
)

// EscalationRuleName identifies a single escalation trigger. The set is closed;
// callers switch on it and it appears verbatim in the EscalationRecord.
type EscalationRuleName = string

const (
	RuleVerifyFail     EscalationRuleName = "verify-fail"
	RuleRetryExhausted EscalationRuleName = "retry-exhausted"
	RuleBlocker        EscalationRuleName = "blocker"
	RuleCostOverBudget EscalationRuleName = "cost-over-budget"
	RuleComplexity     EscalationRuleName = "complexity"
)

// escalationRuleOrder is the fixed evaluation/priority order. The first firing
// rule becomes the record's Rule; the order is a user-visible contract (it
// decides which rule "wins" when several fire) and must not change without
// intent.
var escalationRuleOrder = []EscalationRuleName{
	RuleVerifyFail,
	RuleRetryExhausted,
	RuleBlocker,
	RuleCostOverBudget,
	RuleComplexity,
}

// EscalationVerdict is the engine's output: which rule (if any) fired first, the
// full set of rules that fired, and a deterministic, human-readable rendering of
// the facts that justified it.
type EscalationVerdict struct {
	Triggered bool                 `json:"triggered"`
	Rule      EscalationRuleName   `json:"rule,omitempty"`
	Fired     []EscalationRuleName `json:"fired,omitempty"`
	Facts     string               `json:"facts,omitempty"`
}

// verifyFailThreshold resolves the effective verify-fail threshold.
func (c EscalationConfig) verifyFailThreshold() int {
	if c.VerifyFailThreshold > 0 {
		return c.VerifyFailThreshold
	}
	return defaultVerifyFailThreshold
}

// blockerThreshold resolves the effective blocker threshold.
func (c EscalationConfig) blockerThreshold() int {
	if c.BlockerThreshold > 0 {
		return c.BlockerThreshold
	}
	return defaultBlockerThreshold
}

// EvaluateEscalation applies the rule set to a fact tuple under cfg. It returns a
// verdict naming the first firing rule (in escalationRuleOrder) plus every rule
// that fired. When cfg.Enabled is false it always returns an untriggered verdict
// — the single opt-in gate for the whole engine. Pure: no IO, no clock, no state
// mutation.
func EvaluateEscalation(facts EscalationFacts, cfg EscalationConfig) EscalationVerdict {
	if !cfg.Enabled {
		return EscalationVerdict{}
	}
	fired := map[EscalationRuleName]bool{}

	if facts.VerifyFailCount >= cfg.verifyFailThreshold() {
		fired[RuleVerifyFail] = true
	}
	// A retry budget of 0 means "no retries configured"; the rule only fires once a
	// positive budget has been fully consumed, never on an unconfigured budget.
	if facts.MaxRetries > 0 && facts.RetryCount >= facts.MaxRetries {
		fired[RuleRetryExhausted] = true
	}
	if facts.BlockerCount >= cfg.blockerThreshold() {
		fired[RuleBlocker] = true
	}
	// Cost breach only counts against a positive tier budget (V4). A zero budget is
	// "no cap", so it can never be exceeded.
	if facts.TierBudgetUSD > 0 && facts.CostUSD > facts.TierBudgetUSD {
		fired[RuleCostOverBudget] = true
	}
	if cfg.ComplexityThreshold > 0 && facts.ComplexityScore >= cfg.ComplexityThreshold {
		fired[RuleComplexity] = true
	}

	verdict := EscalationVerdict{}
	for _, name := range escalationRuleOrder {
		if fired[name] {
			verdict.Fired = append(verdict.Fired, name)
		}
	}
	if len(verdict.Fired) == 0 {
		return verdict
	}
	verdict.Triggered = true
	verdict.Rule = verdict.Fired[0]
	verdict.Facts = renderEscalationFacts(facts, verdict.Fired)
	return verdict
}

// renderEscalationFacts produces a compact, deterministic "k=v k=v" string of the
// facts relevant to the fired rules. Deterministic key order (sorted) makes it
// byte-stable across runs (invariant 7), so it can be stored and diffed.
func renderEscalationFacts(facts EscalationFacts, fired []EscalationRuleName) string {
	kv := map[string]string{}
	for _, name := range fired {
		switch name {
		case RuleVerifyFail:
			kv["verifyFail"] = fmt.Sprintf("%d", facts.VerifyFailCount)
		case RuleRetryExhausted:
			kv["retry"] = fmt.Sprintf("%d/%d", facts.RetryCount, facts.MaxRetries)
		case RuleBlocker:
			kv["blockers"] = fmt.Sprintf("%d", facts.BlockerCount)
		case RuleCostOverBudget:
			kv["cost"] = fmt.Sprintf("%.4f/%.4f", facts.CostUSD, facts.TierBudgetUSD)
		case RuleComplexity:
			kv["complexity"] = fmt.Sprintf("%d", facts.ComplexityScore)
		}
	}
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+kv[k])
	}
	return strings.Join(parts, " ")
}

// EscalationFactsForTask assembles the fact tuple for one task from recorded,
// countable evidence: verify failures and retries from the trajectory ledger
// (V3), open blockers from state, the retry budget from orchestration config, and
// the routed tier budget/cost from the caller (V4). It reads only recorded facts,
// so the verdict is reproducible from disk (invariant 6). tierBudgetUSD/costUSD
// are 0 when routing is not configured.
//
// Derivation from the trajectory: a "verify fail" is a verify tool/result event
// for the task carrying a non-zero exit code; RetryCount is the number of verify
// attempts beyond the first (attempts-1), since each re-attempt after an initial
// failure is a retry.
func EscalationFactsForTask(state *State, traj []TrajectoryEvent, taskID string, maxRetries int, tierBudgetUSD, costUSD float64) EscalationFacts {
	facts := EscalationFacts{Task: taskID, MaxRetries: maxRetries, TierBudgetUSD: tierBudgetUSD, CostUSD: costUSD}

	// Trajectory source (orchestrated/brain flow): per-event verify exit codes.
	trajFails, trajAttempts := 0, 0
	for _, ev := range traj {
		if ev.Tool != "verify" || ev.ExitCode == nil || !containsStr(ev.TaskIDs, taskID) {
			continue
		}
		trajAttempts++
		if *ev.ExitCode != 0 {
			trajFails++
		}
	}
	trajRetries := 0
	if trajAttempts > 1 {
		trajRetries = trajAttempts - 1
	}

	// State source (direct `specd verify` flow): persistent per-task telemetry.
	telFails, telRetries := 0, 0
	if state != nil {
		if ts, ok := state.Tasks[taskID]; ok && ts.Telemetry != nil {
			telFails = ts.Telemetry.VerifyFails
			if ts.Telemetry.Retries > 1 {
				telRetries = ts.Telemetry.Retries - 1
			}
		}
		for _, b := range state.Blockers {
			if b.Task == taskID {
				facts.BlockerCount++
			}
		}
	}

	// A task is exercised through exactly one flow, so max (not sum) reconciles the
	// two sources without double-counting when only one is populated.
	facts.VerifyFailCount = maxInt(trajFails, telFails)
	facts.RetryCount = maxInt(trajRetries, telRetries)
	return facts
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// NewEscalationRecord builds the EscalationRecord written to state when a verdict
// triggers. The clock is the injected FakeClock-compatible Clock so records are
// deterministic under test (invariant 7).
func NewEscalationRecord(verdict EscalationVerdict, task string) *EscalationRecord {
	return &EscalationRecord{
		Task:  task,
		Rule:  verdict.Rule,
		Facts: verdict.Facts,
		Time:  Clock().UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
}

// EscalateOnVerify is the wiring point invoked after a verify record is written
// (the "verify record" trigger, spec §3). It assembles the task's facts, applies
// the rules, and on a trigger writes state.Escalation and returns the record so
// the caller can surface it. It mutates only state.Escalation and is a no-op when
// escalation is disabled or nothing fires — so the default (Enabled=false) path
// is byte-identical to pre-escalation behaviour. The task is *not* auto-switched
// to conductor; that stays a human decision (invariant: never auto-switch).
func EscalateOnVerify(state *State, traj []TrajectoryEvent, taskID string, cfg EscalationConfig, maxRetries int, tierBudgetUSD, costUSD float64) *EscalationRecord {
	if !cfg.Enabled || state == nil {
		return nil
	}
	facts := EscalationFactsForTask(state, traj, taskID, maxRetries, tierBudgetUSD, costUSD)
	verdict := EvaluateEscalation(facts, cfg)
	if !verdict.Triggered {
		return nil
	}
	rec := NewEscalationRecord(verdict, taskID)
	state.Escalation = rec
	return rec
}

// ConductorHandoffRecommendation is the advisory mode flip a host reads when a
// task has been escalated: mode_recommend → conductor with the escalation facts
// as rationale. UserDecides is always true (never an automatic switch).
type ConductorHandoffRecommendation struct {
	Recommended string `json:"recommended"` // "conductor" when escalated, else ""
	Task        string `json:"task,omitempty"`
	Rule        string `json:"rule,omitempty"`
	Rationale   string `json:"rationale"`
	UserDecides bool   `json:"userDecides"`
}

// RecommendConductorForEscalation returns the conductor-handoff recommendation
// for the spec's current escalation record, or a zero recommendation when the
// spec is not escalated. Deterministic from the record.
func RecommendConductorForEscalation(state *State) ConductorHandoffRecommendation {
	rec := ConductorHandoffRecommendation{UserDecides: true}
	if state == nil || state.Escalation == nil {
		return rec
	}
	e := state.Escalation
	rec.Recommended = "conductor"
	rec.Task = e.Task
	rec.Rule = e.Rule
	rec.Rationale = fmt.Sprintf("task %s escalated by rule %q (%s) — resolve in conductor mode (`specd mode --set conductor`) or override (`specd orchestrate %s resume --override`)",
		e.Task, e.Rule, e.Facts, "<spec>")
	return rec
}

// EscalationRationale renders the conductor-handoff rationale for a triggered
// verdict on a named task: the deterministic sentence a host shows when
// mode_recommend flips to conductor. It is phrasing over facts, never a decision.
func EscalationRationale(verdict EscalationVerdict, task string) string {
	if !verdict.Triggered {
		return ""
	}
	return fmt.Sprintf("auto-escalation rule %q fired for task %s (%s) — hand off to conductor for human-paced resolution",
		verdict.Rule, task, verdict.Facts)
}
