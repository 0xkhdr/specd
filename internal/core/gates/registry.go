package gates

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

func HasErrors(findings []Finding) bool {
	for _, finding := range findings {
		if finding.Severity == Error {
			return true
		}
	}
	return false
}
