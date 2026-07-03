package core

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func TestValidateSchedule(t *testing.T) {
	cases := []struct {
		name string
		s    MaintenanceSchedule
		ok   bool
	}{
		{"good", MaintenanceSchedule{Name: "nightly-audit", Command: "make audit", IntervalSeconds: 86400}, true},
		{"empty name", MaintenanceSchedule{Command: "x", IntervalSeconds: 1}, false},
		{"bad chars", MaintenanceSchedule{Name: "Nightly Audit", Command: "x", IntervalSeconds: 1}, false},
		{"leading hyphen", MaintenanceSchedule{Name: "-x", Command: "x", IntervalSeconds: 1}, false},
		{"empty command", MaintenanceSchedule{Name: "x", Command: "  ", IntervalSeconds: 1}, false},
		{"zero interval", MaintenanceSchedule{Name: "x", Command: "x", IntervalSeconds: 0}, false},
		{"negative interval", MaintenanceSchedule{Name: "x", Command: "x", IntervalSeconds: -5}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateSchedule(c.s)
			if c.ok && err != nil {
				t.Fatalf("want valid, got %v", err)
			}
			if !c.ok && err == nil {
				t.Fatal("want invalid, got nil")
			}
		})
	}
}

func TestUpsertScheduleRoundTripAndReplace(t *testing.T) {
	root := t.TempDir()
	if err := UpsertSchedule(root, MaintenanceSchedule{Name: "a", Command: "echo a", IntervalSeconds: 60}); err != nil {
		t.Fatalf("upsert a: %v", err)
	}
	// Advance a's last-run to prove re-registration preserves cadence.
	if _, ok, err := ClaimSchedule(root, "a", 1000); err != nil || !ok {
		t.Fatalf("claim a: ok=%v err=%v", ok, err)
	}
	// Re-register a with a new command; LastRunUnix must survive.
	if err := UpsertSchedule(root, MaintenanceSchedule{Name: "a", Command: "echo a2", IntervalSeconds: 120}); err != nil {
		t.Fatalf("re-upsert a: %v", err)
	}
	m, err := LoadProgram(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Schedules) != 1 {
		t.Fatalf("want 1 schedule, got %d", len(m.Schedules))
	}
	got := m.Schedules[0]
	if got.Command != "echo a2" || got.IntervalSeconds != 120 {
		t.Fatalf("replace did not apply: %+v", got)
	}
	if got.LastRunUnix != 1000 {
		t.Fatalf("re-register reset cadence: LastRunUnix=%d, want 1000", got.LastRunUnix)
	}
}

func TestRemoveSchedule(t *testing.T) {
	root := t.TempDir()
	_ = UpsertSchedule(root, MaintenanceSchedule{Name: "keep", Command: "x", IntervalSeconds: 1})
	_ = UpsertSchedule(root, MaintenanceSchedule{Name: "drop", Command: "x", IntervalSeconds: 1})

	removed, err := RemoveSchedule(root, "drop")
	if err != nil || !removed {
		t.Fatalf("remove drop: removed=%v err=%v", removed, err)
	}
	if removed, _ := RemoveSchedule(root, "missing"); removed {
		t.Fatal("removing a missing schedule reported removed")
	}
	m, _ := LoadProgram(root)
	if len(m.Schedules) != 1 || m.Schedules[0].Name != "keep" {
		t.Fatalf("unexpected schedules after remove: %+v", m.Schedules)
	}
}

func TestClaimScheduleIdempotent(t *testing.T) {
	root := t.TempDir()
	_ = UpsertSchedule(root, MaintenanceSchedule{Name: "hourly", Command: "x", IntervalSeconds: 3600})

	// First tick at T: never run -> due -> claimed.
	if _, ok, err := ClaimSchedule(root, "hourly", 10_000); err != nil || !ok {
		t.Fatalf("first claim: ok=%v err=%v", ok, err)
	}
	// Second tick at the SAME instant: not due -> not claimed (the CAS).
	if _, ok, err := ClaimSchedule(root, "hourly", 10_000); err != nil || ok {
		t.Fatalf("double-invoke claimed twice: ok=%v err=%v", ok, err)
	}
	// Still inside the interval: not due.
	if _, ok, _ := ClaimSchedule(root, "hourly", 12_000); ok {
		t.Fatal("claimed before interval elapsed")
	}
	// Interval elapsed: due again.
	if _, ok, err := ClaimSchedule(root, "hourly", 13_600); err != nil || !ok {
		t.Fatalf("claim after interval: ok=%v err=%v", ok, err)
	}
}

// TestClaimScheduleConcurrent proves the program lock makes the claim a true
// CAS: N goroutines racing to claim the same due schedule at the same instant
// yield exactly one winner.
func TestClaimScheduleConcurrent(t *testing.T) {
	root := t.TempDir()
	_ = UpsertSchedule(root, MaintenanceSchedule{Name: "race", Command: "x", IntervalSeconds: 3600})

	const n = 8
	var wg sync.WaitGroup
	var wins int32
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, ok, err := ClaimSchedule(root, "race", 50_000); err == nil && ok {
				atomic.AddInt32(&wins, 1)
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("concurrent claim winners = %d, want 1", wins)
	}
}

func TestClaimScheduleMissing(t *testing.T) {
	root := t.TempDir()
	if _, _, err := ClaimSchedule(root, "nope", 1); err == nil {
		t.Fatal("want NotFound for missing schedule")
	}
}

func TestDueSchedules(t *testing.T) {
	m := ProgramManifest{Schedules: []MaintenanceSchedule{
		{Name: "never", Command: "x", IntervalSeconds: 100, LastRunUnix: 0},
		{Name: "fresh", Command: "x", IntervalSeconds: 100, LastRunUnix: 950},
		{Name: "stale", Command: "x", IntervalSeconds: 100, LastRunUnix: 800},
	}}
	due := DueSchedules(m, 1000)
	if len(due) != 2 {
		t.Fatalf("want 2 due, got %d: %+v", len(due), due)
	}
	names := map[string]bool{due[0].Name: true, due[1].Name: true}
	if !names["never"] || !names["stale"] || names["fresh"] {
		t.Fatalf("wrong due set: %+v", due)
	}
}

func TestScheduleByteStableRoundTrip(t *testing.T) {
	root := t.TempDir()
	_ = UpsertSchedule(root, MaintenanceSchedule{Name: "b", Command: "echo b", IntervalSeconds: 30})
	_ = UpsertSchedule(root, MaintenanceSchedule{Name: "a", Command: "echo a", IntervalSeconds: 60})

	first, err := os.ReadFile(ProgramPath(root))
	if err != nil {
		t.Fatal(err)
	}
	// Reload and rewrite unchanged: bytes must be identical (deterministic order).
	m, _ := LoadProgram(root)
	if err := SaveProgram(root, m); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(ProgramPath(root))
	if string(first) != string(second) {
		t.Fatalf("program.json not byte-stable:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// FuzzLoadProgramSchedules drives arbitrary program.json bytes through the
// manifest parser: it must never panic, and any manifest that parses must
// re-save byte-stably (deterministic schedule ordering).
func FuzzLoadProgramSchedules(f *testing.F) {
	f.Add(`{"version":1,"dependsOn":{},"schedules":[{"name":"a","command":"x","intervalSeconds":60}]}`)
	f.Add(`{"schedules":[{"name":"b","command":"y","intervalSeconds":1,"lastRunUnix":9}]}`)
	f.Add(`not json`)
	f.Add(`{"schedules":null}`)
	f.Fuzz(func(t *testing.T, doc string) {
		root := t.TempDir()
		if err := os.MkdirAll(SpecdDir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ProgramPath(root), []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		m, err := LoadProgram(root)
		if err != nil {
			return // corrupt input is a clean error, not a panic
		}
		if err := SaveProgram(root, m); err != nil {
			t.Fatalf("save after load: %v", err)
		}
		first, _ := os.ReadFile(ProgramPath(root))
		reloaded, err := LoadProgram(root)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if err := SaveProgram(root, reloaded); err != nil {
			t.Fatal(err)
		}
		second, _ := os.ReadFile(ProgramPath(root))
		if string(first) != string(second) {
			t.Fatalf("not byte-stable:\n%s\n---\n%s", first, second)
		}
	})
}
