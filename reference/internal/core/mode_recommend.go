package core

import (
	"fmt"
	"strings"
)

// mode_recommend.go computes a deterministic, advisory execution-mode
// recommendation from on-disk, countable spec facts. No LLM is ever consulted —
// any host that runs this against the same files gets the same verdict, which is
// what makes the recommendation reproducible (the Foundational Split: the
// harness supplies a deterministic signal, the agent phrases it, the user
// decides). The harness never flips a spec's mode on the strength of this; it
// only advises.

// ModeSignals are the raw countable facts the recommendation is derived from.
type ModeSignals struct {
	TaskCount       int `json:"taskCount"`
	MaxWaveWidth    int `json:"maxWaveWidth"`
	DistinctRoles   int `json:"distinctRoles"`
	CrossSpecEdges  int `json:"crossSpecEdges"`
	EstimatedTokens int `json:"estimatedTokens"`
}

// ModeRecommendation is the advisory verdict. UserDecides is always true: the
// recommendation is input to a human choice, never an automatic action.
type ModeRecommendation struct {
	Recommended string      `json:"recommended"` // "simple" | "orchestrated"
	Confidence  string      `json:"confidence"`  // "neutral" | "suggest" | "strong"
	Signals     ModeSignals `json:"signals"`
	Rationale   string      `json:"rationale"`
	UserDecides bool        `json:"userDecides"`
}

// Confidence levels for a mode recommendation.
const (
	ConfidenceNeutral = "neutral"
	ConfidenceSuggest = "suggest"
	ConfidenceStrong  = "strong"
)

// RecommendMode computes the advisory recommendation for a spec. It reads the
// task DAG from state, cross-spec edges from the program manifest (if any), and
// a token estimate from the spec's planning artifacts. Before tasks.md has been
// parsed into the DAG (TaskCount == 0) the verdict is neutral — there is nothing
// to measure, so the host should reason from the prose instead.
func RecommendMode(root, slug string) (ModeRecommendation, error) {
	state, err := LoadState(root, slug)
	if err != nil {
		return ModeRecommendation{}, err
	}
	if state == nil {
		return ModeRecommendation{}, NotFoundError(fmt.Sprintf("spec '%s' not found", slug))
	}

	sig := computeModeSignals(root, slug, state)
	return verdictFromSignals(sig), nil
}

// computeModeSignals derives the countable facts from on-disk state.
func computeModeSignals(root, slug string, state *State) ModeSignals {
	sig := ModeSignals{TaskCount: len(state.Tasks)}

	waveWidth := map[int]int{}
	roles := map[string]struct{}{}
	for _, t := range state.Tasks {
		waveWidth[t.Wave]++
		if t.Role != "" {
			roles[t.Role] = struct{}{}
		}
	}
	for _, w := range waveWidth {
		if w > sig.MaxWaveWidth {
			sig.MaxWaveWidth = w
		}
	}
	sig.DistinctRoles = len(roles)
	sig.CrossSpecEdges = crossSpecEdgeCount(root, slug)
	sig.EstimatedTokens = estimatePlanningTokens(root, slug)
	return sig
}

// crossSpecEdgeCount counts program-manifest edges touching slug — both this
// spec's own dependencies and other specs that depend on it. Zero when there is
// no program manifest.
func crossSpecEdgeCount(root, slug string) int {
	prog, err := LoadProgram(root)
	if err != nil {
		return 0
	}
	edges := 0
	for from, deps := range prog.DependsOn {
		for _, to := range deps {
			if from == slug || to == slug {
				edges++
			}
		}
	}
	return edges
}

// estimatePlanningTokens estimates the token footprint of the spec's planning
// corpus (requirements + design + tasks), deterministically via EstimateTokens.
func estimatePlanningTokens(root, slug string) int {
	var b strings.Builder
	for _, name := range []string{"requirements.md", "design.md", "tasks.md"} {
		if s := ReadArtifact(root, slug, name); s != nil {
			b.WriteString(*s)
		}
	}
	return EstimateTokens([]byte(b.String()))
}

// verdictFromSignals applies the documented heuristic (spec §3.5). The number of
// distinct orchestration payoffs that fire sets the confidence: none → stay
// Base (neutral), one → suggest, two or more → strong.
func verdictFromSignals(sig ModeSignals) ModeRecommendation {
	rec := ModeRecommendation{Signals: sig, UserDecides: true}

	if sig.TaskCount == 0 {
		rec.Recommended = ModeSimple
		rec.Confidence = ConfidenceNeutral
		rec.Rationale = "No tasks.md DAG yet — nothing to measure. Stay Base; reconsider after tasks are planned."
		return rec
	}

	var reasons []string
	if sig.TaskCount >= 10 && sig.MaxWaveWidth >= 3 {
		reasons = append(reasons, fmt.Sprintf("%d tasks across waves up to %d wide (parallel Pinky dispatch likely saves wall-clock)", sig.TaskCount, sig.MaxWaveWidth))
	}
	if sig.DistinctRoles >= 3 {
		reasons = append(reasons, fmt.Sprintf("%d distinct roles in the DAG (role-isolation payoff)", sig.DistinctRoles))
	}
	if sig.CrossSpecEdges > 0 {
		reasons = append(reasons, fmt.Sprintf("%d cross-spec program edge(s) (program-level coordination)", sig.CrossSpecEdges))
	}

	switch len(reasons) {
	case 0:
		rec.Recommended = ModeSimple
		rec.Confidence = ConfidenceNeutral
		rec.Rationale = fmt.Sprintf("Small/serial work (%d tasks, max wave width %d, %d roles) — Base mode is the simpler fit.", sig.TaskCount, sig.MaxWaveWidth, sig.DistinctRoles)
	case 1:
		rec.Recommended = ModeOrchestrated
		rec.Confidence = ConfidenceSuggest
		rec.Rationale = "Orchestration may help: " + reasons[0] + "."
	default:
		rec.Recommended = ModeOrchestrated
		rec.Confidence = ConfidenceStrong
		rec.Rationale = "Orchestration recommended: " + strings.Join(reasons, "; ") + "."
	}
	return rec
}
