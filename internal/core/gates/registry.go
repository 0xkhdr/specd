package gates

import (
	speccontext "github.com/0xkhdr/specd/internal/context"
)

type Severity string

const (
	Info  Severity = "info"
	Warn  Severity = "warn"
	Error Severity = "error"
)

type Finding struct {
	Gate     string   `json:"gate"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

type Gate interface {
	Name() string
	Run(CheckCtx) []Finding
}

type Registry struct {
	gates []Gate
}

func NewRegistry() Registry {
	return Registry{}
}

func (r *Registry) Register(gate Gate) {
	r.gates = append(r.gates, gate)
}

func (r Registry) Names() []string {
	names := make([]string, 0, len(r.gates))
	for _, gate := range r.gates {
		names = append(names, gate.Name())
	}
	return names
}

func (r Registry) Run(ctx CheckCtx) []Finding {
	var findings []Finding
	for _, gate := range r.gates {
		for _, finding := range gate.Run(ctx) {
			if finding.Gate == "" {
				finding.Gate = gate.Name()
			}
			findings = append(findings, finding)
		}
	}
	return findings
}

// Append returns a registry with gates appended in caller order. It makes
// profile composition explicit while preserving stable core gate order.
func (r Registry) Append(gates ...Gate) Registry {
	for _, gate := range gates {
		r.Register(gate)
	}
	return r
}

// steeringApplicability emits exactly one warning when every steering file in
// the root is dropped from the machine manifest for missing applicability
// metadata (R1.2). Per-file omission (R1.3) stays silent, an empty/absent
// steering directory emits nothing, and the check never gates completion — it
// is a diagnostic, pure over on-disk bytes, with no LLM in its path.
func steeringApplicability(ctx CheckCtx) []Finding {
	if ctx.Root == "" {
		return nil
	}
	misconfigured, err := speccontext.SteeringTotalOmission(ctx.Root)
	if err != nil || !misconfigured {
		return nil
	}
	return []Finding{{
		Severity: Warn,
		Message:  "every steering file is dropped from the machine manifest for missing `specd-context` metadata; add a `specd-context` block (id, version, priority) to each `.specd/steering/*.md` or drivers run with no project steering",
	}}
}

func HasErrors(findings []Finding) bool {
	for _, finding := range findings {
		if finding.Severity == Error {
			return true
		}
	}
	return false
}
