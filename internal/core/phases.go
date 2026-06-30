package core

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/0xkhdr/specd/internal/spec"
)

// DesignSections lists the required level-2 headings a design.md must contain,
// in the order DesignGate checks for them.
var DesignSections = []string{
	"Overview", "Architecture", "Components and interfaces", "Data models",
	"Error handling", "Verification strategy", "Risks and open questions",
}

// Violation describes a single gate-check failure, identifying which gate
// produced it, where in the document it occurred, and a human-readable reason.
type Violation struct {
	Gate     string `json:"gate"`
	Location string `json:"location"`
	Message  string `json:"message"`
}

// DesignGate checks design.md content against DesignSections, returning a
// Violation for every required section that is missing, empty, or still
// contains a TODO marker.
func DesignGate(md *string) []Violation {
	if md == nil || strings.TrimSpace(*md) == "" {
		return []Violation{{Gate: "design", Location: "design.md", Message: "design.md missing or empty"}}
	}
	lines := splitLines(StripHTMLComments(*md))
	var v []Violation
	for _, sec := range DesignSections {
		escaped := regexp.QuoteMeta(sec)
		re := regexp.MustCompile(`(?i)^##\s+` + escaped + `\s*$`)
		idx := -1
		for i, l := range lines {
			if re.MatchString(l) {
				idx = i
				break
			}
		}
		if idx == -1 {
			v = append(v, Violation{Gate: "design", Location: "design.md", Message: fmt.Sprintf("missing section: ## %s", sec)})
			continue
		}
		var body []string
		for i := idx + 1; i < len(lines); i++ {
			if len(lines[i]) > 2 && lines[i][:3] == "## " {
				break
			}
			body = append(body, lines[i])
		}
		text := strings.TrimSpace(strings.Join(body, "\n"))
		if text == "" {
			v = append(v, Violation{Gate: "design", Location: fmt.Sprintf("design.md:%d", idx+1), Message: fmt.Sprintf("section '%s' is empty", sec)})
		} else if strings.Contains(text, "TODO") {
			v = append(v, Violation{Gate: "design", Location: fmt.Sprintf("design.md:%d", idx+1), Message: fmt.Sprintf("section '%s' still contains a TODO marker", sec)})
		}
	}
	return v
}

// PhaseForStatus is re-exported from internal/spec so existing core call sites
// (and PlanningAdvance below) keep working without importing spec directly.
var PhaseForStatus = spec.PhaseForStatus

// AdvanceTarget is the status/phase pair a spec moves to when it advances
// past its current planning status.
type AdvanceTarget struct {
	Status SpecStatus
	Phase  Phase
}

// PlanningAdvance maps each planning status to the status and phase a spec
// advances to once that status's readiness gate is satisfied.
var PlanningAdvance = map[SpecStatus]AdvanceTarget{
	StatusRequirements: {StatusDesign, PhaseForStatus(StatusDesign)},
	StatusDesign:       {StatusTasks, PhaseForStatus(StatusTasks)},
	StatusTasks:        {StatusExecuting, PhaseForStatus(StatusExecuting)},
}

// PhaseReadiness checks whether the spec's current status is ready to advance,
// returning a list of human-readable issues (empty when ready). It validates
// requirements.md via LintEars, design.md via DesignGate, or tasks.md via the
// dependency-graph checks, depending on status.
func PhaseReadiness(status SpecStatus, reqMd *string, designMd *string, doc ParsedTasks) []string {
	if status == StatusRequirements {
		if reqMd == nil || strings.TrimSpace(*reqMd) == "" {
			return []string{"requirements.md missing or empty"}
		}
		issues := LintEars(*reqMd)
		out := make([]string, len(issues))
		for i, iss := range issues {
			out[i] = fmt.Sprintf("requirements.md:%d: %s", iss.Line, iss.Message)
		}
		return out
	}
	if status == StatusDesign {
		violations := DesignGate(designMd)
		out := make([]string, len(violations))
		for i, viol := range violations {
			out[i] = fmt.Sprintf("%s: %s", viol.Location, viol.Message)
		}
		return out
	}
	if status == StatusTasks {
		if len(doc.Tasks) == 0 {
			return []string{"tasks.md: no tasks defined"}
		}
		dag := make([]DagTask, len(doc.Tasks))
		for i, t := range doc.Tasks {
			dag[i] = DagTask{
				ID:      t.ID,
				Wave:    t.Wave,
				Depends: ParseDepends(t.Meta["depends"]),
				Status:  TaskPending,
			}
		}
		var out []string
		for _, o := range OrphanDeps(dag) {
			out = append(out, fmt.Sprintf("tasks.md: %s depends on missing task '%s'", o.Task, o.Dep))
		}
		if cyc := DetectCycle(dag); cyc != nil {
			out = append(out, fmt.Sprintf("tasks.md: dependency cycle: %s", strings.Join(cyc, " → ")))
		}
		for _, w := range WaveViolations(dag) {
			out = append(out, fmt.Sprintf("tasks.md: %s depends on later-wave task '%s'", w.Task, w.Dep))
		}
		return out
	}
	return nil
}
