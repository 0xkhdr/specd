package core

import (
	"sort"
	"strconv"
	"strings"
)

// WaveTelemetry is the aggregated cost/timing evidence for one wave. All numeric
// fields are sums of the per-task Telemetry records (which are themselves
// measured via the injectable Clock or operator-annotated); specd never computes
// cost or pricing — Tokens/Cost are simple roll-ups of annotated values.
type WaveTelemetry struct {
	Wave             int     `json:"wave"`
	Tasks            int     `json:"tasks"`
	DurationMs       int64   `json:"durationMs"`
	VerifyDurationMs int64   `json:"verifyDurationMs"`
	Retries          int     `json:"retries"`
	Tokens           int     `json:"tokens"`
	Cost             float64 `json:"cost"`
	CostAnnotated    bool    `json:"costAnnotated"`
}

// SpecTelemetry is the per-spec roll-up plus its per-wave breakdown. It is a pure
// function of the task Telemetry records — no clock, no IO — so it is
// deterministic and golden-comparable.
type SpecTelemetry struct {
	Spec             string          `json:"spec"`
	DurationMs       int64           `json:"durationMs"`
	VerifyDurationMs int64           `json:"verifyDurationMs"`
	Retries          int             `json:"retries"`
	Tokens           int             `json:"tokens"`
	Cost             float64         `json:"cost"`
	CostAnnotated    bool            `json:"costAnnotated"`
	Waves            []WaveTelemetry `json:"waves"`
}

// HasData reports whether any task carried telemetry worth rendering.
func (s SpecTelemetry) HasData() bool {
	return s.DurationMs > 0 || s.VerifyDurationMs > 0 || s.Retries > 0 || s.Tokens > 0 || s.CostAnnotated
}

// RollupTelemetry aggregates a spec's per-task Telemetry into per-wave and
// per-spec totals. Cost is parsed from the annotated string field; an
// unparseable cost is ignored (contributes 0) rather than failing the roll-up,
// keeping it total over partially-annotated specs.
func RollupTelemetry(state *State) SpecTelemetry {
	out := SpecTelemetry{Spec: state.Spec}
	byWave := map[int]*WaveTelemetry{}
	for _, t := range state.Tasks {
		w := byWave[t.Wave]
		if w == nil {
			w = &WaveTelemetry{Wave: t.Wave}
			byWave[t.Wave] = w
		}
		w.Tasks++
		if t.Telemetry == nil {
			continue
		}
		tel := t.Telemetry
		w.DurationMs += tel.DurationMs
		w.VerifyDurationMs += tel.VerifyDurationMs
		w.Retries += tel.Retries
		w.Tokens += tel.Tokens
		if c, ok := parseCostStr(tel.Cost); ok {
			w.Cost += c
			w.CostAnnotated = true
			out.CostAnnotated = true
		}

		out.DurationMs += tel.DurationMs
		out.VerifyDurationMs += tel.VerifyDurationMs
		out.Retries += tel.Retries
		out.Tokens += tel.Tokens
		if c, ok := parseCostStr(tel.Cost); ok {
			out.Cost += c
		}
	}
	waveNums := make([]int, 0, len(byWave))
	for w := range byWave {
		waveNums = append(waveNums, w)
	}
	sort.Ints(waveNums)
	for _, n := range waveNums {
		out.Waves = append(out.Waves, *byWave[n])
	}
	return out
}

// parseCostStr parses an operator-annotated cost string (e.g. "0.42") into a
// float. A blank or unparseable value yields (0, false) so callers can treat
// "no annotated cost" distinctly from "$0".
func parseCostStr(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "$")), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
