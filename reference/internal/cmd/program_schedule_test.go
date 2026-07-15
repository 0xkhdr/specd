package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// tickArgs builds the cli.Args for a `program tick` at a fixed instant.
func tickArgs(now string) cli.Args {
	return cli.Args{Pos: []string{"tick"}, Flags: map[string]string{"now": now}}
}

// TestProgramTickDoubleInvokeRunsOnce is the P3.5 idempotency guarantee: two
// ticks at the same instant must execute a due schedule exactly once. The
// scheduled command appends a byte to a marker file; the file length is the
// run count.
func TestProgramTickDoubleInvokeRunsOnce(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "runs")
	if err := core.UpsertSchedule(root, core.MaintenanceSchedule{
		Name:            "nightly",
		Command:         "printf x >> " + marker,
		IntervalSeconds: 3600,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if code := programTick(root, tickArgs("100000")); code != core.ExitOK {
		t.Fatalf("first tick exit=%d", code)
	}
	if code := programTick(root, tickArgs("100000")); code != core.ExitOK {
		t.Fatalf("second tick exit=%d", code)
	}

	runs, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker not written — command never ran: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("double-invoke ran command %d times, want 1", len(runs))
	}

	// After the interval elapses, the schedule is due again and runs once more.
	if code := programTick(root, tickArgs("103600")); code != core.ExitOK {
		t.Fatalf("post-interval tick exit=%d", code)
	}
	runs, _ = os.ReadFile(marker)
	if len(runs) != 2 {
		t.Fatalf("post-interval run count = %d, want 2", len(runs))
	}
}

// TestProgramTickReportsCommandFailure: a non-zero scheduled command surfaces as
// a gate exit, and the schedule is still marked run (best-effort, retried next
// interval — not re-run within the same window).
func TestProgramTickReportsCommandFailure(t *testing.T) {
	root := t.TempDir()
	if err := core.UpsertSchedule(root, core.MaintenanceSchedule{
		Name:            "flaky",
		Command:         "exit 3",
		IntervalSeconds: 60,
	}); err != nil {
		t.Fatal(err)
	}
	if code := programTick(root, tickArgs("500")); code != core.ExitGate {
		t.Fatalf("failing schedule exit=%d, want ExitGate", code)
	}
	// Same window: not due, so it does not re-run (exit OK, nothing ran).
	if code := programTick(root, tickArgs("500")); code != core.ExitOK {
		t.Fatalf("re-tick in window exit=%d, want ExitOK", code)
	}
}

func TestProgramScheduleRegisterListRemove(t *testing.T) {
	root := t.TempDir()

	reg := cli.Args{Pos: []string{"schedule", "audit"}, Flags: map[string]string{"interval": "3600", "command": "make audit"}}
	if code := programSchedule(root, reg); code != core.ExitOK {
		t.Fatalf("register exit=%d", code)
	}
	m, _ := core.LoadProgram(root)
	if len(m.Schedules) != 1 || m.Schedules[0].Command != "make audit" {
		t.Fatalf("register did not persist: %+v", m.Schedules)
	}

	bad := cli.Args{Pos: []string{"schedule", "audit"}, Flags: map[string]string{"interval": "notanum", "command": "x"}}
	if code := programSchedule(root, bad); code != core.ExitUsage {
		t.Fatalf("bad interval exit=%d, want ExitUsage", code)
	}

	rm := cli.Args{Pos: []string{"schedule", "audit"}, Flags: map[string]string{"remove": "true"}}
	if code := programSchedule(root, rm); code != core.ExitOK {
		t.Fatalf("remove exit=%d", code)
	}
	m, _ = core.LoadProgram(root)
	if len(m.Schedules) != 0 {
		t.Fatalf("remove did not delete: %+v", m.Schedules)
	}
}
