package core

import (
	"fmt"
	"sort"
	"strings"
)

// PRSummaryTask is one task row in a PR summary.
type PRSummaryTask struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Role   string `json:"role"`
}

// PRSummaryWave groups tasks by wave for the summary's DAG view.
type PRSummaryWave struct {
	Wave  int             `json:"wave"`
	Tasks []PRSummaryTask `json:"tasks"`
}

// PRSummary is a deterministic, network-free snapshot of a spec suitable for a
// pull-request comment: gate status, wave/task progress, and (optionally) the
// commit↔task link map. It is derived purely from in-process data — no GitHub
// API, no network.
type PRSummary struct {
	Spec       string          `json:"spec"`
	Title      string          `json:"title"`
	Status     string          `json:"status"`
	GatesOK    bool            `json:"gatesOk"`
	TasksDone  int             `json:"tasksDone"`
	TasksTotal int             `json:"tasksTotal"`
	Waves      []PRSummaryWave `json:"waves"`
	Violations []Violation     `json:"violations"`
	Warnings   []Violation     `json:"warnings"`
	Commits    []CommitLink    `json:"commits,omitempty"`
}

// BuildPRSummary assembles a PRSummary from spec state, the gate result, and an
// optional commit-link map. Passing nil commits omits the commit section.
func BuildPRSummary(state *State, violations, warnings []Violation, commits []CommitLink) PRSummary {
	if violations == nil {
		violations = []Violation{}
	}
	if warnings == nil {
		warnings = []Violation{}
	}
	s := PRSummary{
		Spec:       state.Spec,
		Title:      state.Title,
		Status:     string(state.Status),
		GatesOK:    len(violations) == 0,
		TasksTotal: len(state.Tasks),
		Violations: violations,
		Warnings:   warnings,
		Commits:    commits,
	}

	byWave := map[int][]PRSummaryTask{}
	for _, t := range state.Tasks {
		if t.Status == TaskComplete {
			s.TasksDone++
		}
		byWave[t.Wave] = append(byWave[t.Wave], PRSummaryTask{
			ID: t.ID, Title: t.Title, Status: string(t.Status), Role: t.Role,
		})
	}
	waveNums := make([]int, 0, len(byWave))
	for w := range byWave {
		waveNums = append(waveNums, w)
	}
	sort.Ints(waveNums)
	for _, w := range waveNums {
		tasks := byWave[w]
		sort.Slice(tasks, func(i, j int) bool { return ordinal(tasks[i].ID) < ordinal(tasks[j].ID) })
		s.Waves = append(s.Waves, PRSummaryWave{Wave: w, Tasks: tasks})
	}
	return s
}

// Markdown renders the summary as a GitHub-flavored Markdown comment. Output is
// a pure function of the PRSummary value — identical input yields identical
// bytes.
func (s PRSummary) Markdown() string {
	var b strings.Builder
	gate := "✅ all gates green"
	if !s.GatesOK {
		gate = fmt.Sprintf("❌ %d gate violation(s)", len(s.Violations))
	}
	fmt.Fprintf(&b, "## specd — %s (`%s`)\n\n", s.Title, s.Spec)
	fmt.Fprintf(&b, "- **Status:** %s\n", s.Status)
	fmt.Fprintf(&b, "- **Gates:** %s\n", gate)
	fmt.Fprintf(&b, "- **Tasks:** %d / %d complete\n\n", s.TasksDone, s.TasksTotal)

	for _, w := range s.Waves {
		fmt.Fprintf(&b, "### Wave %d\n\n", w.Wave)
		b.WriteString("| Task | Role | Status |\n|------|------|--------|\n")
		for _, t := range w.Tasks {
			fmt.Fprintf(&b, "| %s — %s | %s | %s |\n", t.ID, t.Title, t.Role, statusMark(t.Status))
		}
		b.WriteString("\n")
	}

	if len(s.Violations) > 0 {
		b.WriteString("### Gate violations\n\n")
		for _, v := range s.Violations {
			fmt.Fprintf(&b, "- `%s` %s — %s\n", v.Gate, v.Location, v.Message)
		}
		b.WriteString("\n")
	}
	if len(s.Warnings) > 0 {
		b.WriteString("### Warnings\n\n")
		for _, w := range s.Warnings {
			fmt.Fprintf(&b, "- `%s` %s — %s\n", w.Gate, w.Location, w.Message)
		}
		b.WriteString("\n")
	}
	if len(s.Commits) > 0 {
		b.WriteString("### Commits\n\n")
		for _, c := range s.Commits {
			ref := "_(no task ref)_"
			if len(c.Tasks) > 0 {
				ref = strings.Join(c.Tasks, ", ")
			}
			fmt.Fprintf(&b, "- `%s` %s → %s\n", shortSHA(c.SHA), c.Subject, ref)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func statusMark(status string) string {
	switch TaskStatus(status) {
	case TaskComplete:
		return "✅ complete"
	case TaskRunning:
		return "▶ running"
	case TaskBlocked:
		return "⛔ blocked"
	default:
		return "○ pending"
	}
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
