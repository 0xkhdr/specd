package core

import (
	"fmt"
	"strings"
)

// MaintenanceSchedule is a registered recurring maintenance program (P3.5). It
// is a declaration only: specd never daemonizes and never runs it on a timer.
// A host scheduler (cron, CI, systemd timer) invokes `specd program tick`, which
// runs each schedule whose interval has elapsed exactly once, guarded by the
// program lock so a double-invoked tick is idempotent.
type MaintenanceSchedule struct {
	Name string `json:"name"`
	// Command is the operator-authored shell command, run through the shared
	// sandboxed exec path with a scrubbed env — never git/GitHub logic embedded.
	Command string `json:"command"`
	// Sandbox selects the runner backend ("" = default); mirrors submit.sandbox.
	Sandbox string `json:"sandbox,omitempty"`
	// IntervalSeconds is the minimum elapsed wall-clock time between runs.
	IntervalSeconds int64 `json:"intervalSeconds"`
	// LastRunUnix is the wall-clock second at which tick last claimed this
	// schedule. Zero means never run (so the first tick always claims it).
	LastRunUnix int64 `json:"lastRunUnix,omitempty"`
}

// validScheduleName restricts schedule names to a kebab-case charset so they are
// safe as manifest keys and never collide with shell/flag parsing.
func validScheduleName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return name[0] != '-' && name[len(name)-1] != '-'
}

// ValidateSchedule checks a schedule is well-formed before it is persisted.
func ValidateSchedule(s MaintenanceSchedule) error {
	if !validScheduleName(s.Name) {
		return GateError(fmt.Sprintf("invalid schedule name %q — use lowercase letters, digits and hyphens (max 64)", s.Name))
	}
	if strings.TrimSpace(s.Command) == "" {
		return GateError("schedule command is empty — provide --command")
	}
	if s.IntervalSeconds <= 0 {
		return GateError(fmt.Sprintf("invalid interval %ds — must be a positive number of seconds", s.IntervalSeconds))
	}
	return nil
}

// scheduleDue reports whether a schedule is due at nowUnix: it has never run, or
// at least IntervalSeconds have elapsed since its last claimed run.
func scheduleDue(s MaintenanceSchedule, nowUnix int64) bool {
	return nowUnix-s.LastRunUnix >= s.IntervalSeconds
}

// DueSchedules returns the schedules in manifest that are due at nowUnix, in the
// manifest's stored order. It is a pure read — it never claims or mutates state.
func DueSchedules(m ProgramManifest, nowUnix int64) []MaintenanceSchedule {
	var due []MaintenanceSchedule
	for _, s := range m.Schedules {
		if scheduleDue(s, nowUnix) {
			due = append(due, s)
		}
	}
	return due
}

// UpsertSchedule registers or replaces a schedule by name, persisting it to
// program.json under the program lock. A replaced schedule keeps its existing
// LastRunUnix so re-registering does not reset its cadence.
func UpsertSchedule(root string, s MaintenanceSchedule) error {
	if err := ValidateSchedule(s); err != nil {
		return err
	}
	_, err := WithProgramLock(root, func() (struct{}, error) {
		m, err := LoadProgram(root)
		if err != nil {
			return struct{}{}, err
		}
		replaced := false
		for i := range m.Schedules {
			if m.Schedules[i].Name == s.Name {
				s.LastRunUnix = m.Schedules[i].LastRunUnix
				m.Schedules[i] = s
				replaced = true
				break
			}
		}
		if !replaced {
			m.Schedules = append(m.Schedules, s)
		}
		return struct{}{}, SaveProgram(root, m)
	})
	return err
}

// RemoveSchedule deletes a schedule by name, returning whether one was removed.
func RemoveSchedule(root, name string) (bool, error) {
	return WithProgramLock(root, func() (bool, error) {
		m, err := LoadProgram(root)
		if err != nil {
			return false, err
		}
		kept := m.Schedules[:0]
		removed := false
		for _, s := range m.Schedules {
			if s.Name == name {
				removed = true
				continue
			}
			kept = append(kept, s)
		}
		if !removed {
			return false, nil
		}
		m.Schedules = kept
		return true, SaveProgram(root, m)
	})
}

// ClaimSchedule atomically claims a schedule for execution at nowUnix. Under the
// program lock it re-reads the manifest, and if the named schedule is still due
// it advances LastRunUnix to nowUnix and persists before returning ok=true. A
// concurrent or repeated tick that finds the schedule no longer due returns
// ok=false without executing — this is the CAS that makes tick idempotent. The
// claim is recorded before the command runs, so a crashed command is retried on
// the next elapsed interval rather than re-run within the same window.
func ClaimSchedule(root, name string, nowUnix int64) (MaintenanceSchedule, bool, error) {
	type claim struct {
		s  MaintenanceSchedule
		ok bool
	}
	res, err := WithProgramLock(root, func() (claim, error) {
		m, err := LoadProgram(root)
		if err != nil {
			return claim{}, err
		}
		for i := range m.Schedules {
			if m.Schedules[i].Name != name {
				continue
			}
			if !scheduleDue(m.Schedules[i], nowUnix) {
				return claim{s: m.Schedules[i], ok: false}, nil
			}
			claimed := m.Schedules[i]
			m.Schedules[i].LastRunUnix = nowUnix
			if err := SaveProgram(root, m); err != nil {
				return claim{}, err
			}
			return claim{s: claimed, ok: true}, nil
		}
		return claim{}, NotFoundError(fmt.Sprintf("no maintenance schedule named %q", name))
	})
	return res.s, res.ok, err
}
