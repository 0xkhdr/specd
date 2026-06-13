package core

import "sort"

type DagTask struct {
	ID      string
	Wave    int
	Depends []string
	Status  TaskStatus
}

type NextResultKind string

const (
	NextTask       NextResultKind = "task"
	NextAllComplete NextResultKind = "all-complete"
	NextAllBlocked  NextResultKind = "all-blocked"
	NextWaiting     NextResultKind = "waiting"
)

type NextResult struct {
	Kind     NextResultKind `json:"kind"`
	ID       string         `json:"id,omitempty"`
	Blocked  []string       `json:"blocked,omitempty"`
	Blocking []string       `json:"blocking,omitempty"`
}

func ordinal(id string) int {
	for i, c := range id {
		if c >= '0' && c <= '9' {
			n := 0
			for _, d := range id[i:] {
				if d < '0' || d > '9' {
					break
				}
				n = n*10 + int(d-'0')
			}
			return n
		}
	}
	return int(^uint(0) >> 1)
}

func byID(tasks []DagTask) map[string]DagTask {
	m := make(map[string]DagTask, len(tasks))
	for _, t := range tasks {
		m[t.ID] = t
	}
	return m
}

func OrphanDeps(tasks []DagTask) []struct{ Task, Dep string } {
	ids := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		ids[t.ID] = true
	}
	var out []struct{ Task, Dep string }
	for _, t := range tasks {
		for _, d := range t.Depends {
			if !ids[d] {
				out = append(out, struct{ Task, Dep string }{t.ID, d})
			}
		}
	}
	return out
}

func DetectCycle(tasks []DagTask) []string {
	m := byID(tasks)
	const (
		WHITE = 0
		GREY  = 1
		BLACK = 2
	)
	color := make(map[string]int, len(tasks))
	for _, t := range tasks {
		color[t.ID] = WHITE
	}
	stack := []string{}

	var dfs func(id string) []string
	dfs = func(id string) []string {
		color[id] = GREY
		stack = append(stack, id)
		node, ok := m[id]
		if ok {
			for _, dep := range node.Depends {
				if _, exists := m[dep]; !exists {
					continue
				}
				c := color[dep]
				if c == GREY {
					idx := 0
					for i, s := range stack {
						if s == dep {
							idx = i
							break
						}
					}
					cycle := make([]string, len(stack)-idx+1)
					copy(cycle, stack[idx:])
					cycle[len(cycle)-1] = dep
					return cycle
				}
				if c == WHITE {
					if found := dfs(dep); found != nil {
						return found
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[id] = BLACK
		return nil
	}

	for _, t := range tasks {
		if color[t.ID] == WHITE {
			if cycle := dfs(t.ID); cycle != nil {
				return cycle
			}
		}
	}
	return nil
}

func WaveViolations(tasks []DagTask) []struct{ Task, Dep string } {
	m := byID(tasks)
	var out []struct{ Task, Dep string }
	for _, t := range tasks {
		for _, d := range t.Depends {
			dep, ok := m[d]
			if ok && dep.Wave > t.Wave {
				out = append(out, struct{ Task, Dep string }{t.ID, d})
			}
		}
	}
	return out
}

func isRunnable(t DagTask, m map[string]DagTask) bool {
	if t.Status != TaskPending {
		return false
	}
	for _, d := range t.Depends {
		dep, ok := m[d]
		if !ok || dep.Status != TaskComplete {
			return false
		}
	}
	return true
}

func NextRunnable(tasks []DagTask) NextResult {
	m := byID(tasks)
	var remaining []DagTask
	for _, t := range tasks {
		if t.Status != TaskComplete {
			remaining = append(remaining, t)
		}
	}
	if len(remaining) == 0 {
		return NextResult{Kind: NextAllComplete}
	}

	var runnable []DagTask
	for _, t := range remaining {
		if isRunnable(t, m) {
			runnable = append(runnable, t)
		}
	}
	sort.Slice(runnable, func(i, j int) bool {
		if runnable[i].Wave != runnable[j].Wave {
			return runnable[i].Wave < runnable[j].Wave
		}
		return ordinal(runnable[i].ID) < ordinal(runnable[j].ID)
	})
	if len(runnable) > 0 {
		return NextResult{Kind: NextTask, ID: runnable[0].ID}
	}

	var pending, blocked []DagTask
	for _, t := range remaining {
		if t.Status == TaskBlocked {
			blocked = append(blocked, t)
		} else if t.Status == TaskPending {
			pending = append(pending, t)
		}
	}
	if len(pending) == 0 && len(blocked) > 0 {
		ids := make([]string, len(blocked))
		for i, t := range blocked {
			ids[i] = t.ID
		}
		return NextResult{Kind: NextAllBlocked, Blocked: ids}
	}

	blockingSet := make(map[string]bool)
	for _, t := range pending {
		for _, d := range t.Depends {
			dep, ok := m[d]
			if !ok || dep.Status != TaskComplete {
				blockingSet[d] = true
			}
		}
	}
	blocking := make([]string, 0, len(blockingSet))
	for id := range blockingSet {
		blocking = append(blocking, id)
	}
	sort.Slice(blocking, func(i, j int) bool {
		return ordinal(blocking[i]) < ordinal(blocking[j])
	})
	return NextResult{Kind: NextWaiting, Blocking: blocking}
}

func RunnableFrontier(tasks []DagTask) []DagTask {
	m := byID(tasks)
	var out []DagTask
	for _, t := range tasks {
		if isRunnable(t, m) {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Wave != out[j].Wave {
			return out[i].Wave < out[j].Wave
		}
		return ordinal(out[i].ID) < ordinal(out[j].ID)
	})
	return out
}

type WaveRow struct {
	Wave  int
	Tasks []DagTask
}

func GroupWaves(tasks []DagTask) []WaveRow {
	waveSet := make(map[int]bool)
	for _, t := range tasks {
		waveSet[t.Wave] = true
	}
	waves := make([]int, 0, len(waveSet))
	for w := range waveSet {
		waves = append(waves, w)
	}
	sort.Ints(waves)
	rows := make([]WaveRow, len(waves))
	for i, w := range waves {
		var ts []DagTask
		for _, t := range tasks {
			if t.Wave == w {
				ts = append(ts, t)
			}
		}
		sort.Slice(ts, func(a, b int) bool {
			return ordinal(ts[a].ID) < ordinal(ts[b].ID)
		})
		rows[i] = WaveRow{Wave: w, Tasks: ts}
	}
	return rows
}

func CriticalPath(tasks []DagTask) []string {
	m := byID(tasks)
	memo := make(map[string][]string)

	var longest func(id string, seen map[string]bool) []string
	longest = func(id string, seen map[string]bool) []string {
		if v, ok := memo[id]; ok {
			return v
		}
		if seen[id] {
			return []string{id}
		}
		seen[id] = true
		node, ok := m[id]
		var best []string
		if ok {
			for _, d := range node.Depends {
				if _, exists := m[d]; !exists {
					continue
				}
				p := longest(d, seen)
				if len(p) > len(best) {
					best = p
				}
			}
		}
		delete(seen, id)
		result := append(append([]string{}, best...), id)
		memo[id] = result
		return result
	}

	var best []string
	for _, t := range tasks {
		p := longest(t.ID, make(map[string]bool))
		if len(p) > len(best) {
			best = p
		}
	}
	return best
}
