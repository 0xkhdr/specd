package cmd

import (
	"path/filepath"

	"github.com/0xkhdr/specd/internal/core"
)

// escalationMaxFails resolves the ratchet threshold from config (default 3, 0
// disables). Config errors are non-fatal here — a broken config falls back to
// the default rather than silently disabling the safety net.
func escalationMaxFails(root string) int {
	cfg, _ := core.LoadConfig(core.ConfigPaths{Project: filepath.Join(root, "project.yml")}, getenv())
	return cfg.Escalation.MaxVerifyFails
}

// verifyTimeoutSecs resolves the per-verify wall-clock bound from config (0 =
// unbounded). Config errors fall back to unbounded rather than blocking verify.
func verifyTimeoutSecs(root string) int {
	cfg, _ := core.LoadConfig(core.ConfigPaths{Project: filepath.Join(root, "project.yml")}, getenv())
	return cfg.Verify.TimeoutSecs
}

// escalatedCounts returns the escalated task ids (→ consecutive fail count) for
// a spec, reading the evidence and override ledgers.
func escalatedCounts(root, slug string, tasks []core.TaskRow) (map[string]int, error) {
	evidence, err := core.LoadEvidenceRecords(core.EvidencePath(root, slug))
	if err != nil {
		return nil, err
	}
	overrides, err := core.LoadOverrides(core.OverridePath(root, slug))
	if err != nil {
		return nil, err
	}
	return core.EscalatedCounts(evidence, overrides, tasks, escalationMaxFails(root)), nil
}

// taskFailCount returns the current consecutive verify-fail count for one task.
func taskFailCount(root, slug, id string) (int, error) {
	evidence, err := core.LoadEvidenceRecords(core.EvidencePath(root, slug))
	if err != nil {
		return 0, err
	}
	overrides, err := core.LoadOverrides(core.OverridePath(root, slug))
	if err != nil {
		return 0, err
	}
	return core.ConsecutiveVerifyFails(evidence, overrides, id), nil
}

// escalatedAdvisory returns escalated tasks for display. When the ratchet is
// active (maxFails > 0) it reports genuinely-blocked tasks; when disabled it
// still surfaces tasks at or above the default threshold as advisory, so
// `status` shows repeated failures regardless of the ratchet setting (spec 06
// R2/R6). The bool reports whether the ratchet is actually enforcing.
func escalatedAdvisory(root, slug string, tasks []core.TaskRow) (map[string]int, bool, error) {
	maxFails := escalationMaxFails(root)
	active := maxFails > 0
	effective := maxFails
	if !active {
		effective = core.EscalationDefaultMaxVerifyFails
	}
	evidence, err := core.LoadEvidenceRecords(core.EvidencePath(root, slug))
	if err != nil {
		return nil, active, err
	}
	overrides, err := core.LoadOverrides(core.OverridePath(root, slug))
	if err != nil {
		return nil, active, err
	}
	return core.EscalatedCounts(evidence, overrides, tasks, effective), active, nil
}

// escalatedBoolSet projects a count map to the boolean set FrontierExcluding wants.
func escalatedBoolSet(counts map[string]int) map[string]bool {
	if len(counts) == 0 {
		return nil
	}
	set := make(map[string]bool, len(counts))
	for id := range counts {
		set[id] = true
	}
	return set
}
