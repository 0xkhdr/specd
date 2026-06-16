package core

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

// FrontierDetector tracks the last frontier observed per spec and reports only
// real changes. It is the read-only heart of the watch daemon: feed it freshly
// loaded states and it emits a FrontierEvent exactly when the runnable set
// changes (including the first observation of a spec). Revision is monotonic
// (SaveState only ever bumps it), so an unchanged revision can be skipped
// without recomputation by callers; the detector also guards against emitting
// when the revision advanced but the frontier did not.
type FrontierDetector struct {
	last map[string][]string
}

// NewFrontierDetector returns a detector with no prior observations.
func NewFrontierDetector() *FrontierDetector {
	return &FrontierDetector{last: map[string][]string{}}
}

// Observe computes the current frontier for state and returns an event plus
// whether it represents a change since the last Observe for the same spec. The
// first observation of a spec always reports changed=true.
func (d *FrontierDetector) Observe(state *State) (FrontierEvent, bool) {
	cur := FrontierOf(state)
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
