package core

import "testing"

// helpers_test.go is the single home for package-core test helpers that build or
// project unexported package types (DagTask, State, ACPStore). Shared by every
// _test.go in package core; do not re-declare these per file. See
// testharness/HELPERS.md for the placement rule.

// ids projects a DagTask slice to its ordered IDs for slice comparisons.
func ids(tasks []DagTask) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	return out
}

// mkState assembles a minimal executing State from task states keyed by ID.
func mkState(spec string, rev int, tasks ...TaskState) *State {
	m := map[string]TaskState{}
	for _, t := range tasks {
		m[t.ID] = t
	}
	return &State{Spec: spec, Revision: rev, Status: StatusExecuting, Tasks: m}
}

// newTestACPStore opens an ACPStore rooted at a throwaway temp dir.
func newTestACPStore(t *testing.T) *ACPStore {
	t.Helper()
	store, err := NewACPStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}
