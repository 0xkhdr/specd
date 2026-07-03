package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	conductorTaskRE  = regexp.MustCompile(`^- \[[ xX]\] ([A-Za-z][A-Za-z0-9_.-]*):\s*(.+)$`)
	conductorMicroRE = regexp.MustCompile(`^\s+- \[([ xX])\] (m[A-Za-z0-9_.-]*):\s*(.+)$`)
	conductorMetaRE  = regexp.MustCompile(`^\s+([A-Za-z][A-Za-z0-9_-]*):\s*(.*)$`)
)

type MicroTask struct {
	TaskID  string   `json:"taskID"`
	ID      string   `json:"id"`
	Key     string   `json:"key"`
	Title   string   `json:"title"`
	Checked bool     `json:"checked"`
	Deps    []string `json:"deps,omitempty"`
	Verify  string   `json:"verify,omitempty"`
	Line    int      `json:"line"`
}

type ConductorPlan struct {
	Slug   string      `json:"slug"`
	Micros []MicroTask `json:"micros"`
}

type ConductorEvent struct {
	SessionID string `json:"sessionID"`
	Time      string `json:"time"`
	Action    string `json:"action"`
	Task      string `json:"task,omitempty"`
	Micro     string `json:"micro,omitempty"`
	Status    string `json:"status,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type ConductorStatus struct {
	Active    *ConductorSession `json:"active,omitempty"`
	Frontier  []MicroTask       `json:"frontier"`
	Completed []string          `json:"completed,omitempty"`
	Rejected  int               `json:"rejected"`
	Events    int               `json:"events"`
}

func LoadConductorPlan(root, slug string) (ConductorPlan, error) {
	path := ArtifactPath(root, slug, "tasks.md")
	f, err := os.Open(path)
	if err != nil {
		return ConductorPlan{}, err
	}
	defer f.Close()

	var micros []MicroTask
	seen := map[string]int{}
	byTask := map[string]map[string]bool{}
	currentTask := ""
	var currentMicro *MicroTask

	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Text()
		if m := conductorTaskRE.FindStringSubmatch(raw); m != nil && !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") {
			currentTask = m[1]
			currentMicro = nil
			if byTask[currentTask] == nil {
				byTask[currentTask] = map[string]bool{}
			}
			continue
		}
		if m := conductorMicroRE.FindStringSubmatch(raw); m != nil {
			if currentTask == "" {
				return ConductorPlan{}, GateError(fmt.Sprintf("tasks.md:%d: micro task %s has no parent task", line, m[2]))
			}
			key := currentTask + "/" + m[2]
			if first, ok := seen[key]; ok {
				return ConductorPlan{}, GateError(fmt.Sprintf("tasks.md:%d: duplicate micro task %s (first seen on line %d)", line, key, first))
			}
			seen[key] = line
			byTask[currentTask][m[2]] = true
			micro := MicroTask{
				TaskID:  currentTask,
				ID:      m[2],
				Key:     key,
				Title:   strings.TrimSpace(m[3]),
				Checked: strings.EqualFold(m[1], "x"),
				Line:    line,
			}
			micros = append(micros, micro)
			currentMicro = &micros[len(micros)-1]
			continue
		}
		if currentMicro != nil {
			if m := conductorMetaRE.FindStringSubmatch(raw); m != nil {
				switch strings.ToLower(m[1]) {
				case "deps":
					currentMicro.Deps = parseMicroDeps(m[2])
				case "verify":
					currentMicro.Verify = strings.TrimSpace(m[2])
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return ConductorPlan{}, err
	}
	for _, micro := range micros {
		for _, dep := range micro.Deps {
			depID := dep
			if strings.Contains(dep, "/") {
				parts := strings.SplitN(dep, "/", 2)
				if parts[0] != micro.TaskID {
					return ConductorPlan{}, GateError(fmt.Sprintf("tasks.md:%d: micro dependency %s crosses parent task boundary", micro.Line, dep))
				}
				depID = parts[1]
			}
			if !byTask[micro.TaskID][depID] {
				return ConductorPlan{}, GateError(fmt.Sprintf("tasks.md:%d: unknown micro dependency %s", micro.Line, dep))
			}
		}
	}
	return ConductorPlan{Slug: slug, Micros: micros}, nil
}

func ConductorFrontier(plan ConductorPlan, events []ConductorEvent) []MicroTask {
	done := conductorDoneSet(events)
	var out []MicroTask
	for _, micro := range plan.Micros {
		if micro.Checked || done[micro.Key] {
			continue
		}
		ready := true
		for _, dep := range micro.Deps {
			key := dep
			if !strings.Contains(key, "/") {
				key = micro.TaskID + "/" + dep
			}
			if !done[key] {
				ready = false
				break
			}
		}
		if ready {
			out = append(out, micro)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TaskID != out[j].TaskID {
			return out[i].TaskID < out[j].TaskID
		}
		return out[i].Line < out[j].Line
	})
	return out
}

func ReadConductorEvents(root, slug string) ([]ConductorEvent, error) {
	path := ConductorLedgerPath(root, slug)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var events []ConductorEvent
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		var ev ConductorEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			return nil, GateError(fmt.Sprintf("conductor.jsonl:%d: invalid event: %v", line, err))
		}
		events = append(events, ev)
	}
	return events, scanner.Err()
}

func AppendConductorEvent(root, slug string, ev ConductorEvent) error {
	if ev.Time == "" {
		ev.Time = Clock().UTC().Format(time.RFC3339Nano)
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return AppendFile(ConductorLedgerPath(root, slug), string(b)+"\n")
}

func ConductorLedgerPath(root, slug string) string {
	return ArtifactPath(root, slug, "conductor.jsonl")
}

func BuildConductorStatus(state *State, plan ConductorPlan, events []ConductorEvent) ConductorStatus {
	done := conductorDoneSet(events)
	var completed []string
	for key := range done {
		completed = append(completed, key)
	}
	sort.Strings(completed)
	rejected := 0
	for _, ev := range events {
		if ev.Action == "reject" {
			rejected++
		}
	}
	return ConductorStatus{
		Active:    state.Conductor,
		Frontier:  ConductorFrontier(plan, events),
		Completed: completed,
		Rejected:  rejected,
		Events:    len(events),
	}
}

// RejectionCluster is one exact rejection reason with its occurrence count.
type RejectionCluster struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// ConductorRejectionReport clusters conductor reject events by their exact
// reason string and counts each. Rejections are the training signal; this is a
// pure count with no interpretation of the prose (invariant 6). Clusters sort
// by descending count, then reason ascending, so the report is deterministic.
func ConductorRejectionReport(events []ConductorEvent) []RejectionCluster {
	counts := map[string]int{}
	for _, ev := range events {
		if ev.Action == "reject" {
			counts[ev.Reason]++
		}
	}
	out := make([]RejectionCluster, 0, len(counts))
	for reason, count := range counts {
		out = append(out, RejectionCluster{Reason: reason, Count: count})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}

func parseMicroDeps(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	var out []string
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	sort.Strings(out)
	return out
}

func conductorDoneSet(events []ConductorEvent) map[string]bool {
	done := map[string]bool{}
	for _, ev := range events {
		key := ev.Task + "/" + ev.Micro
		switch ev.Action {
		case "accept":
			if ev.Task != "" && ev.Micro != "" {
				done[key] = true
			}
		case "reject":
			delete(done, key)
		}
	}
	return done
}
