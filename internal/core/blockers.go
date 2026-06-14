package core

import "fmt"

// RemoveBlocker drops any blocker entry for task id from s.Blockers. It always
// writes a fresh slice so the result never aliases the previous backing array
// (which may still be referenced by an already-marshaled snapshot of state).
func RemoveBlocker(s *State, id string) {
	kept := make([]Blocker, 0, len(s.Blockers))
	for _, b := range s.Blockers {
		if b.Task != id {
			kept = append(kept, b)
		}
	}
	s.Blockers = kept
}

// AddBlocker records (or replaces) the blocker for task id. Any prior entry for
// the same task is removed first so a task never has two blocker records.
func AddBlocker(s *State, id, reason string, turn int) {
	RemoveBlocker(s, id)
	s.Blockers = append(s.Blockers, Blocker{Task: id, Reason: reason, Since: fmt.Sprintf("Turn %d", turn)})
}
