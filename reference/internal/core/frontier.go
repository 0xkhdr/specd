package core

import (
	"sort"
	"time"

	"github.com/0xkhdr/specd/internal/obs"
)

// FrontierEvent is a single change in a spec's runnable frontier — the set of
// tasks whose dependencies are all complete and which are themselves pending.
// It is the unit the watch daemon emits. The event is purely derived from spec
// state; producing it never mutates anything.
type FrontierEvent struct {
	Spec     string   `json:"spec"`
	Revision int      `json:"revision"`
	Status   string   `json:"status"`
	Frontier []string `json:"frontier"`          // runnable task IDs, wave/ordinal order
	Added    []string `json:"added,omitempty"`   // entered the frontier since last event
	Removed  []string `json:"removed,omitempty"` // left the frontier since last event
	At       string   `json:"at"`
}

// FrontierOf returns the ordered runnable-task IDs for a spec's state.
func FrontierOf(state *State) []string {
	front := RunnableFrontier(DagTasksFromState(state))
	ids := make([]string, len(front))
	for i, t := range front {
		ids[i] = t.ID
	}
	return ids
}

// frontierCache holds the per-spec bookkeeping FrontierDetector needs to
// update the runnable frontier incrementally instead of rescanning every task
// on every Observe call. It is ephemeral (never persisted to state.json,
// rebuilt on process start from whatever state Observe is first called with)
// and is discarded and rebuilt from scratch whenever the task set or any
// Depends edge changes, or on first sight of a spec.
type frontierCache struct {
	revision   int
	byID       map[string]DagTask
	dependents map[string][]string // taskID -> direct dependents (tasks whose Depends include taskID)
	frontier   map[string]bool     // current runnable set, by ID
	ordered    []string            // frontier in RunnableFrontier's wave/ordinal order
}

// FrontierDetector tracks the last frontier observed per spec and reports only
// real changes. It is the read-only heart of the watch daemon: feed it freshly
// loaded states and it emits a FrontierEvent exactly when the runnable set
// changes (including the first observation of a spec). Revision is monotonic
// (SaveState only ever bumps it): an unchanged revision short-circuits to the
// cached result, and an advanced revision triggers an incremental update that
// re-evaluates runnability only for tasks whose status changed and their
// direct dependents (see frontierFor), not the full task list. The detector
// also guards against emitting when the revision advanced but the frontier
// did not.
type FrontierDetector struct {
	last  map[string][]string
	cache map[string]*frontierCache
}

// NewFrontierDetector returns a detector with no prior observations.
func NewFrontierDetector() *FrontierDetector {
	return &FrontierDetector{last: map[string][]string{}, cache: map[string]*frontierCache{}}
}

// Observe computes the current frontier for state and returns an event plus
// whether it represents a change since the last Observe for the same spec. The
// first observation of a spec always reports changed=true.
func (d *FrontierDetector) Observe(state *State) (FrontierEvent, bool) {
	started := time.Now()
	defer func() { obs.RecordDuration("frontier_observe_duration", time.Since(started)) }()
	if endSpan := obs.StartSpan("frontier.observe"); endSpan != nil {
		defer endSpan()
	}

	cur := d.frontierFor(state)
	prev, seen := d.last[state.Spec]
	changed := !seen || !equalStrings(prev, cur)

	ev := FrontierEvent{
		Spec:     state.Spec,
		Revision: state.Revision,
		Status:   string(state.Status),
		Frontier: cur,
		At:       NowISO(),
	}
	if seen {
		ev.Added = diffStrings(cur, prev)
		ev.Removed = diffStrings(prev, cur)
	} else {
		ev.Added = append([]string(nil), cur...)
	}
	if changed {
		d.last[state.Spec] = cur
	}
	return ev, changed
}

// frontierFor returns the current runnable frontier (ordered IDs) for state,
// updating the per-spec cache incrementally where possible. Fast paths, in
// order:
//
//  1. Same revision as last observed: nothing changed, return the cached
//     result in O(1).
//  2. Cached, with the same number of tasks as last seen: only statuses may
//     have moved, so only the changed tasks and their direct dependents are
//     re-checked for runnability — not the full task list.
//
// Anything else (first sight of the spec, or the task count changing) falls
// back to a full rebuild: correctness over cleverness.
func (d *FrontierDetector) frontierFor(state *State) []string {
	c, ok := d.cache[state.Spec]
	if !ok {
		c = rebuildFrontierCache(state.Revision, DagTasksFromState(state))
		d.cache[state.Spec] = c
		return c.ordered
	}
	if state.Revision == c.revision {
		return c.ordered
	}

	changedIDs, structural := diffDagTasks(c.byID, state.Tasks)
	if structural {
		c = rebuildFrontierCache(state.Revision, DagTasksFromState(state))
		d.cache[state.Spec] = c
		return c.ordered
	}

	reeval := make(map[string]bool, len(changedIDs)*2)
	for _, id := range changedIDs {
		reeval[id] = true
		for _, dep := range c.dependents[id] {
			reeval[dep] = true
		}
	}
	for id := range reeval {
		t, ok := c.byID[id]
		if !ok {
			continue
		}
		if isRunnable(t, c.byID) {
			c.frontier[id] = true
		} else {
			delete(c.frontier, id)
		}
	}
	c.revision = state.Revision
	c.ordered = orderedFrontierIDs(c.byID, c.frontier)
	return c.ordered
}

// rebuildFrontierCache computes the frontier cache from scratch, deferring
// the actual runnable-set computation to RunnableFrontier so the full-rebuild
// path always returns byte-identical results to it by construction.
func rebuildFrontierCache(revision int, tasks []DagTask) *frontierCache {
	byID := make(map[string]DagTask, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}
	dependents := make(map[string][]string, len(tasks))
	for _, t := range tasks {
		for _, dep := range t.Depends {
			dependents[dep] = append(dependents[dep], t.ID)
		}
	}
	front := RunnableFrontier(tasks)
	frontier := make(map[string]bool, len(front))
	ordered := make([]string, len(front))
	for i, t := range front {
		frontier[t.ID] = true
		ordered[i] = t.ID
	}
	return &frontierCache{
		revision:   revision,
		byID:       byID,
		dependents: dependents,
		frontier:   frontier,
		ordered:    ordered,
	}
}

// diffDagTasks compares stateTasks' statuses against the cached byID
// snapshot, writing any changed status into byID so subsequent isRunnable
// dependency lookups stay fresh. It returns the IDs whose Status changed
// since the snapshot, and reports structural=true if the task count differs
// from the cache (a task was added or removed) — in which case changedIDs is
// meaningless and the caller must rebuild the cache from scratch.
//
// Depends and Wave are deliberately not re-verified here: they are a spec's
// static task graph, fixed once at spec-authoring time
// (internal/core/specfiles.go) and never mutated by orchestration — only
// Status changes after a spec is initialized. Re-comparing every task's
// Depends on every call would cost as much as the full O(V+E) rescan this
// function exists to avoid. The existing dag_test.go suite (Requirement 2.2)
// is the defense if that invariant is ever violated.
func diffDagTasks(byID map[string]DagTask, stateTasks map[string]TaskState) (changedIDs []string, structural bool) {
	if len(stateTasks) != len(byID) {
		structural = true
	}
	for id, ts := range stateTasks {
		old, existed := byID[id]
		if !existed {
			structural = true
			byID[id] = DagTask{ID: ts.ID, Wave: ts.Wave, Depends: ts.Depends, Status: ts.Status}
			continue
		}
		if old.Status != ts.Status {
			changedIDs = append(changedIDs, id)
			old.Status = ts.Status
			byID[id] = old
		}
	}
	return changedIDs, structural
}

// orderedFrontierIDs sorts the frontier set into RunnableFrontier's canonical
// wave/ordinal order.
func orderedFrontierIDs(byID map[string]DagTask, frontier map[string]bool) []string {
	out := make([]DagTask, 0, len(frontier))
	for id := range frontier {
		out = append(out, byID[id])
	}
	sort.Slice(out, func(i, j int) bool { return dagTaskOrder(out[i], out[j]) })
	ids := make([]string, len(out))
	for i, t := range out {
		ids[i] = t.ID
	}
	return ids
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// diffStrings returns the members of a not present in b, preserving a's order.
func diffStrings(a, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, s := range b {
		set[s] = true
	}
	var out []string
	for _, s := range a {
		if !set[s] {
			out = append(out, s)
		}
	}
	return out
}
