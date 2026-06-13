package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var Glyph = map[TaskStatus]string{
	TaskComplete: "✓",
	TaskRunning:  "◐",
	TaskPending:  "○",
	TaskBlocked:  "⚠",
}

type Counts struct {
	Pending  int `json:"pending"`
	Running  int `json:"running"`
	Complete int `json:"complete"`
	Blocked  int `json:"blocked"`
	Total    int `json:"total"`
}

func CountTasks(state *State) Counts {
	var c Counts
	for _, t := range state.Tasks {
		switch t.Status {
		case TaskPending:
			c.Pending++
		case TaskRunning:
			c.Running++
		case TaskComplete:
			c.Complete++
		case TaskBlocked:
			c.Blocked++
		}
		c.Total++
	}
	return c
}

func DagTasksFromState(state *State) []DagTask {
	out := make([]DagTask, 0, len(state.Tasks))
	for _, t := range state.Tasks {
		out = append(out, DagTask{
			ID:      t.ID,
			Wave:    t.Wave,
			Depends: t.Depends,
			Status:  t.Status,
		})
	}
	return out
}

func WaveGraph(state *State) string {
	tasks := DagTasksFromState(state)
	if len(tasks) == 0 {
		return "(no tasks yet)"
	}
	var lines []string
	for _, row := range GroupWaves(tasks) {
		lines = append(lines, fmt.Sprintf("Wave %d", row.Wave))
		for _, t := range row.Tasks {
			full := state.Tasks[t.ID]
			line := fmt.Sprintf("  %s %s  %s", Glyph[t.Status], t.ID, full.Title)
			if t.Status == TaskBlocked && full.Blocker != nil {
				line += fmt.Sprintf("  (blocked: %s)", *full.Blocker)
			}
			lines = append(lines, line)
		}
	}
	cp := CriticalPath(tasks)
	if len(cp) > 0 {
		lines = append(lines, "", "Critical path: "+strings.Join(cp, " → "))
	}
	return strings.Join(lines, "\n")
}

func NextSummary(state *State) string {
	r := NextRunnable(DagTasksFromState(state))
	switch r.Kind {
	case NextTask:
		t := state.Tasks[r.ID]
		return fmt.Sprintf("%s — %s", r.ID, t.Title)
	case NextAllComplete:
		return "all tasks complete"
	case NextAllBlocked:
		return "all remaining blocked: " + strings.Join(r.Blocked, ", ")
	case NextWaiting:
		return "waiting on: " + strings.Join(r.Blocking, ", ")
	}
	return ""
}

func BlockerLines(state *State) []string {
	out := make([]string, len(state.Blockers))
	for i, b := range state.Blockers {
		out[i] = fmt.Sprintf("%s: %s", b.Task, b.Reason)
	}
	return out
}

type MidreqSummary struct {
	Turn   int
	Impact string
	Input  string
}

var (
	midreqHeaderRe = regexp.MustCompile(`(?i)^##\s+Turn\s+(\d+).*?impact:\s*(\w+)`)
	midreqInputRe  = regexp.MustCompile(`(?im)\*\*User input \(verbatim\):\*\*\s*"?(.*?)"?\s*$`)
)

func LatestMidreq(root, slug string) *MidreqSummary {
	raw := ReadArtifact(root, slug, "mid-requirements.md")
	if raw == nil {
		return nil
	}
	idx := strings.LastIndex(*raw, "## Turn ")
	if idx == -1 {
		return nil
	}
	block := (*raw)[idx:]
	headerM := midreqHeaderRe.FindStringSubmatch(block)
	if headerM == nil {
		return nil
	}
	turn := 0
	fmt.Sscanf(headerM[1], "%d", &turn)
	impact := strings.ToLower(headerM[2])
	inputM := midreqInputRe.FindStringSubmatch(block)
	input := ""
	if inputM != nil {
		input = strings.TrimSpace(inputM[1])
	}
	if len(input) > 120 {
		input = input[:117] + "..."
	}
	return &MidreqSummary{Turn: turn, Impact: impact, Input: input}
}

var reqNumRe = regexp.MustCompile(`(?im)^##\s+Requirement\s+(\d+)`)

func RequirementNumbers(reqMd string) map[int]bool {
	m := reqNumRe.FindAllStringSubmatch(StripHTMLComments(reqMd), -1)
	out := make(map[int]bool, len(m))
	for _, match := range m {
		var n int
		fmt.Sscanf(match[1], "%d", &n)
		out[n] = true
	}
	return out
}

func UncoveredRequirements(state *State, reqMd *string) []int {
	if reqMd == nil {
		return nil
	}
	referenced := make(map[int]bool)
	for _, t := range state.Tasks {
		for _, n := range t.Requirements {
			referenced[n] = true
		}
	}
	reqNums := RequirementNumbers(*reqMd)
	var out []int
	for n := range reqNums {
		if !referenced[n] {
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

type AcceptanceGaps struct {
	Unmet  []int    `json:"unmet"`
	Failed []string `json:"failed"`
}

func GetAcceptanceGaps(state *State, reqMd *string) AcceptanceGaps {
	var reqs []int
	if reqMd != nil {
		nums := RequirementNumbers(*reqMd)
		for n := range nums {
			reqs = append(reqs, n)
		}
	}
	passed := make(map[int]bool)
	var failed []string
	for key, rec := range state.Acceptance {
		if rec.Status == "pass" {
			passed[rec.Requirement] = true
		} else {
			failed = append(failed, key)
		}
	}
	var unmet []int
	for _, n := range reqs {
		if !passed[n] {
			unmet = append(unmet, n)
		}
	}
	sort.Ints(unmet)
	sort.Strings(failed)
	return AcceptanceGaps{Unmet: unmet, Failed: failed}
}
