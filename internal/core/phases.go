package core

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/0xkhdr/specd/internal/spec"
)

var DesignSections = []string{
	"Overview", "Architecture", "Components and interfaces", "Data models",
	"Error handling", "Verification strategy", "Risks and open questions",
}

type Violation struct {
	Gate     string `json:"gate"`
	Location string `json:"location"`
	Message  string `json:"message"`
}

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

type AdvanceTarget struct {
	Status SpecStatus
	Phase  Phase
}

var PlanningAdvance = map[SpecStatus]AdvanceTarget{
	StatusRequirements: {StatusDesign, PhaseForStatus(StatusDesign)},
	StatusDesign:       {StatusTasks, PhaseForStatus(StatusTasks)},
	StatusTasks:        {StatusExecuting, PhaseForStatus(StatusExecuting)},
}

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
