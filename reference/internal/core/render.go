package core

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Glyph maps each TaskStatus to the single-character symbol used when
// rendering task lists and wave graphs.
var Glyph = map[TaskStatus]string{
	TaskComplete: "✓",
	TaskRunning:  "◐",
	TaskPending:  "○",
	TaskBlocked:  "⚠",
}

// Counts tallies tasks by status (pending, running, complete, blocked) plus
// the overall total, as reported by CountTasks.
type Counts struct {
	Pending  int `json:"pending"`
	Running  int `json:"running"`
	Complete int `json:"complete"`
	Blocked  int `json:"blocked"`
	Total    int `json:"total"`
}

// CountTasks tallies the tasks in state by their Status into a Counts
// summary.
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

// DagTasksFromState converts state's tasks into the []DagTask form used by
// the DAG/critical-path algorithms (WaveGraph, NextRunnable, CriticalPath).
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

// WaveGraph renders a human-readable, wave-by-wave listing of state's
// tasks, including status glyphs, blocked reasons, and the critical path.
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

// NextSummary returns a short, human-readable description of what to do
// next: the next runnable task, an all-complete message, an all-blocked
// message, or a waiting-on message.
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

// BlockerLines renders state's recorded blockers as "<task>: <reason>"
// lines, one per blocker.
func BlockerLines(state *State) []string {
	out := make([]string, len(state.Blockers))
	for i, b := range state.Blockers {
		out[i] = fmt.Sprintf("%s: %s", b.Task, b.Reason)
	}
	return out
}

// MidreqSummary is a condensed view of the most recent mid-spec
// requirement-change turn: its turn number, impact level, and the verbatim
// user input that triggered it.
type MidreqSummary struct {
	Turn   int
	Impact string
	Input  string
}

var (
	midreqHeaderRe = regexp.MustCompile(`(?i)^##\s+Turn\s+(\d+).*?impact:\s*(\w+)`)
	midreqInputRe  = regexp.MustCompile(`(?im)\*\*User input \(verbatim\):\*\*\s*"?(.*?)"?\s*$`)
)

// LatestMidreq reads root/slug's mid-requirements.md artifact and returns a
// summary of the most recent turn, or nil if the artifact is missing or has
// no parseable turn.
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

// RequirementNumbers extracts the set of requirement numbers declared by
// `## Requirement N` headers in reqMd, after stripping HTML comments.
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

// UncoveredRequirements returns the requirement numbers declared in reqMd
// that no task in state references, sorted ascending, or nil if reqMd is
// nil.
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

// AcceptanceGaps reports the acceptance-criteria gaps for a spec:
// requirement numbers with no passing acceptance record, and the keys of
// acceptance records that failed.
type AcceptanceGaps struct {
	Unmet  []int    `json:"unmet"`
	Failed []string `json:"failed"`
}

// GetAcceptanceGaps computes the AcceptanceGaps for state against reqMd:
// requirement numbers with no passing acceptance record, and the keys of
// any failed acceptance records.
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
