// Package spec holds the cross-cutting state and domain value types that both
// internal/core and internal/context depend on. It is a leaf: it imports only
// the standard library and must never import internal/core, so the two heavier
// packages can share these enums without an import cycle.
package spec

// SpecStatus is the lifecycle status of a spec, recorded in state.json.
type SpecStatus string

const (
	StatusRequirements SpecStatus = "requirements"
	StatusDesign       SpecStatus = "design"
	StatusTasks        SpecStatus = "tasks"
	StatusExecuting    SpecStatus = "executing"
	StatusVerifying    SpecStatus = "verifying"
	StatusComplete     SpecStatus = "complete"
	StatusBlocked      SpecStatus = "blocked"
)

// IsValid reports whether s is one of the recognized lifecycle statuses. It is
// the single source of truth for "is this a status specd ever writes", so a
// resume that finds a hand-edited or corrupt on-disk status can fail loud
// instead of silently coercing it to a default.
func (s SpecStatus) IsValid() bool {
	switch s {
	case StatusRequirements, StatusDesign, StatusTasks,
		StatusExecuting, StatusVerifying, StatusComplete, StatusBlocked:
		return true
	}
	return false
}
