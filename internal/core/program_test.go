package core

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestReopenRejected reproduces the R1.1 gap (09a/T04) and pins the guard that
// 09b/T09 lands: reopening, rewinding, or editing a `complete` spec must fail
// closed with a message directing the user to create a linked successor. Today
// AdvanceStatus rejects a backward move, but with a generic "cannot move status
// backward" message and no dedicated guard covering the edit / add-status paths
// — so the successor-directing assertion is RED. It is skipped (not left RED)
// per specs/prompt.md §5; 09b/T09 removes the skip and makes it GREEN.
func TestReopenRejected(t *testing.T) {
	t.Skip("R1.1 reopen guard + successor-directing message lands in 09b/T09")

	_, err := AdvanceStatus(StatusComplete, StatusExecuting)
	if err == nil {
		t.Fatal("reopening a complete spec must fail closed")
	}
	if !strings.Contains(err.Error(), "successor") {
		t.Errorf("reopen error must direct to a linked successor, got %q", err.Error())
	}
}

func TestProgramWouldCycle(t *testing.T) {
	var p Program
	p.AddLink("a", "b") // a depends on b
	p.AddLink("b", "c") // b depends on c

	// c→a would close the cycle a→b→c→a; the returned path reads from→…→from.
	cycle := p.WouldCycle("c", "a")
	want := []string{"c", "a", "b", "c"}
	if !reflect.DeepEqual(cycle, want) {
		t.Fatalf("cycle = %v, want %v", cycle, want)
	}

	// A safe link returns nil.
	if got := p.WouldCycle("a", "c"); got != nil {
		t.Fatalf("a→c is acyclic, got cycle %v", got)
	}
	// A self-link is always a cycle.
	if got := p.WouldCycle("x", "x"); got == nil {
		t.Fatal("self-link must be reported as a cycle")
	}
}

func TestProgramAddIsIdempotentAndRemove(t *testing.T) {
	var p Program
	p.AddLink("a", "b")
	p.AddLink("a", "b") // duplicate is a no-op
	if len(p.Links) != 1 {
		t.Fatalf("duplicate AddLink should not grow links: %d", len(p.Links))
	}
	if !p.RemoveLink("a", "b") {
		t.Fatal("RemoveLink should report the removed link")
	}
	if p.RemoveLink("a", "b") {
		t.Fatal("removing a nonexistent link should report false")
	}
}

func TestProgramFrontierAndIncompleteDeps(t *testing.T) {
	var p Program
	p.AddLink("a", "b") // a depends on b
	p.AddLink("b", "c") // b depends on c
	specs := []string{"a", "b", "c"}

	// Nothing complete: only c (no deps) is actionable.
	none := func(string) bool { return false }
	if got := p.Frontier(specs, none); !reflect.DeepEqual(got, []string{"c"}) {
		t.Fatalf("frontier with nothing complete = %v, want [c]", got)
	}
	if got := p.IncompleteDeps("a", none); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("a's incomplete deps = %v, want [b]", got)
	}

	// c complete: b becomes actionable, a still blocked by b.
	cDone := func(s string) bool { return s == "c" }
	if got := p.Frontier(specs, cDone); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("frontier with c complete = %v, want [b]", got)
	}
	if got := p.IncompleteDeps("a", cDone); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("a still blocked by b, got %v", got)
	}
}

func TestProgramLoadSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "program.json")

	// A missing file is an empty program at the current schema version.
	empty, err := LoadProgram(path)
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if empty.SchemaVersion != ProgramSchemaVersion || len(empty.Links) != 0 {
		t.Fatalf("missing file should be empty program, got %+v", empty)
	}

	empty.AddLink("api", "auth")
	if err := SaveProgram(path, empty); err != nil {
		t.Fatalf("save: %v", err)
	}
	back, err := LoadProgram(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !back.HasLink("api", "auth") || back.SchemaVersion != ProgramSchemaVersion {
		t.Fatalf("round-trip lost data: %+v", back)
	}
}

func TestProgramRejectsFutureSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "program.json")
	if err := AtomicWrite(path, `{"schema_version":999,"links":[]}`+"\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProgram(path); err == nil {
		t.Fatal("a newer schema must fail closed, not silently misread")
	}
}
