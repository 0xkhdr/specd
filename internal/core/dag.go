package core

import "sort"

// DagTask is a single task node in the task dependency graph, carrying its
// wave assignment, dependency ids, and current execution status.
type DagTask struct {
	ID      string
	Wave    int
	Depends []string
	Status  TaskStatus
}

// NextResultKind identifies what NextRunnable found: a runnable task, that
// all tasks are complete, that all remaining tasks are blocked, or that
// remaining tasks are merely waiting on incomplete dependencies.
type NextResultKind string

// Possible NextResultKind values returned by NextRunnable.
const (
	NextTask        NextResultKind = "task"
	NextAllComplete NextResultKind = "all-complete"
	NextAllBlocked  NextResultKind = "all-blocked"
	NextWaiting     NextResultKind = "waiting"
)

// NextResult is the outcome of NextRunnable: the Kind discriminates whether
// ID names the next runnable task, or Blocked/Blocking list the task ids
// involved in a blocked or waiting state.
type NextResult struct {
	Kind     NextResultKind `json:"kind"`
	ID       string         `json:"id,omitempty"`
	Blocked  []string       `json:"blocked,omitempty"`
	Blocking []string       `json:"blocking,omitempty"`
}

// ordinal extracts the numeric suffix used to break ties when ordering tasks.
//
// Task ids are `T\d+` (enforced by taskRE in tasksparser.go), so a valid id has
// exactly one leading digit run after the `T`. ordinal reads that first digit
// run, giving a numeric (not lexicographic) order — `T10` > `T9`. This keeps the
// tie-break in NextRunnable/RunnableFrontier total over valid ids. Ids without a
// digit sort last (max int); behaviour on malformed ids (e.g. `T1a2` → 1) is
// undefined-but-deterministic and never reached for parser-validated input.
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

// OrphanDeps returns every (task, dependency) pair where the dependency id
// does not match any task in the list.
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

// DetectCycle runs a depth-first search over the task dependency graph and
// returns the ids forming a cycle if one exists, or nil if the graph is acyclic.
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

// WaveViolations returns every (task, dependency) pair where the dependency
// is assigned to a later wave than the task that depends on it.
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

// dagTaskOrder reports whether a sorts before b in the canonical wave/ordinal
// order used for RunnableFrontier/NextRunnable's runnable-task output. It is
// shared with FrontierDetector's incremental path (frontier.go) so both stay
// in the same total order by construction rather than via two independent
// sort implementations that could drift apart.
func dagTaskOrder(a, b DagTask) bool {
	if a.Wave != b.Wave {
		return a.Wave < b.Wave
	}
	return ordinal(a.ID) < ordinal(b.ID)
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

// NextRunnable selects the next task to run from tasks, preferring the
// lowest-wave, lowest-ordinal pending task whose dependencies are all
// complete. It reports NextAllComplete, NextAllBlocked, or NextWaiting when
// no task is currently runnable.
func NextRunnable(tasks []DagTask) NextResult {
	m := byID(tasks)
	remaining := make([]DagTask, 0, len(tasks))
	for _, t := range tasks {
		if t.Status != TaskComplete {
			remaining = append(remaining, t)
		}
	}
	if len(remaining) == 0 {
		return NextResult{Kind: NextAllComplete}
	}

	runnable := make([]DagTask, 0, len(remaining))
	for _, t := range remaining {
		if isRunnable(t, m) {
			runnable = append(runnable, t)
		}
	}
	sort.Slice(runnable, func(i, j int) bool { return dagTaskOrder(runnable[i], runnable[j]) })
	if len(runnable) > 0 {
		return NextResult{Kind: NextTask, ID: runnable[0].ID}
	}

	pending := make([]DagTask, 0, len(remaining))
	blocked := make([]DagTask, 0, len(remaining))
	for _, t := range remaining {
		switch t.Status {
		case TaskBlocked:
			blocked = append(blocked, t)
		case TaskPending:
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

// RunnableFrontier returns every pending task whose dependencies are all
// complete, ordered by wave then ordinal.
func RunnableFrontier(tasks []DagTask) []DagTask {
	m := byID(tasks)
	var out []DagTask
	for _, t := range tasks {
		if isRunnable(t, m) {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return dagTaskOrder(out[i], out[j]) })
	return out
}

// WaveRow groups the tasks belonging to a single wave, as produced by GroupWaves.
type WaveRow struct {
	Wave  int
	Tasks []DagTask
}

// GroupWaves partitions tasks into WaveRow groups ordered by ascending wave
// number, with each group's tasks sorted by id ordinal.
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

// CriticalPath returns the longest dependency chain (by task count) in the DAG.
//
// Precondition: tasks must be acyclic. A cycle makes "longest path" ill-defined,
// and the memo would otherwise be populated with partial paths computed under a
// specific cycle-guard context and reused incorrectly across roots. To stay safe
// when called directly (it is exported), CriticalPath returns nil if DetectCycle
// reports a cycle.
func CriticalPath(tasks []DagTask) []string {
	if DetectCycle(tasks) != nil {
		return nil
	}
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
				if longerOrEarlierPath(p, best) {
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
		if longerOrEarlierPath(p, best) {
			best = p
		}
	}
	return best
}

func longerOrEarlierPath(candidate, current []string) bool {
	if len(candidate) != len(current) {
		return len(candidate) > len(current)
	}
	for i := range candidate {
		ci, cj := ordinal(candidate[i]), ordinal(current[i])
		if ci != cj {
			return ci < cj
		}
		if candidate[i] != current[i] {
			return candidate[i] < current[i]
		}
	}
	return false
}
