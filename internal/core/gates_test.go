package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func strp(s string) *string { return &s }

// validDesignMd returns a design.md body containing every mandatory section so
// GateDesign passes.
func validDesignMd() string {
	var b strings.Builder
	for _, sec := range DesignSections {
		b.WriteString("## " + sec + "\n\ncontent for " + sec + "\n\n")
	}
	return b.String()
}

func TestGateEars(t *testing.T) {
	// Empty doc: LintEars flags "no '## Requirement N' sections found" — 1 violation.
	v, _ := GateEars(CheckCtx{ReqMd: strp("")})
	if len(v) != 1 || v[0].Gate != "ears" {
		t.Fatalf("empty reqMd: want 1 ears violation, got %v", v)
	}
	// Violating: missing requirements.md.
	v, _ = GateEars(CheckCtx{ReqMd: nil})
	if len(v) != 1 || v[0].Gate != "ears" || v[0].Message != "requirements.md missing" {
		t.Fatalf("nil reqMd: want ears missing violation, got %v", v)
	}
}

func TestGateDesign(t *testing.T) {
	root := t.TempDir()
	slug := "demo"
	write := func(body string) {
		p := ArtifactPath(root, slug, "design.md")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Passing: all mandatory sections present.
	write(validDesignMd())
	v, _ := GateDesign(CheckCtx{Root: root, Slug: slug})
	if len(v) != 0 {
		t.Fatalf("full design: want 0 violations, got %v", v)
	}
	// Violating: empty design.md.
	write("")
	v, _ = GateDesign(CheckCtx{Root: root, Slug: slug})
	if len(v) == 0 || v[0].Gate != "design" {
		t.Fatalf("empty design: want design violation, got %v", v)
	}
}

func TestGateTaskSchema(t *testing.T) {
	// Passing: valid role + runnable verify.
	good := &ParsedTasks{Tasks: []ParsedTask{
		{ID: "T1", Line: 5, Meta: map[string]string{"role": "builder", "verify": "go test ./..."}},
	}}
	v, _ := GateTaskSchema(CheckCtx{Doc: good})
	if len(v) != 0 {
		t.Fatalf("good schema: want 0 violations, got %v", v)
	}
	// Violating: invalid role + N/A verify for a non-readonly role.
	bad := &ParsedTasks{Tasks: []ParsedTask{
		{ID: "T1", Line: 5, Meta: map[string]string{"role": "wizard", "verify": "N/A"}},
	}}
	v, _ = GateTaskSchema(CheckCtx{Doc: bad})
	if len(v) != 2 {
		t.Fatalf("bad schema: want 2 violations, got %v", v)
	}
	// nil doc is a no-op.
	if v, _ := GateTaskSchema(CheckCtx{Doc: nil}); v != nil {
		t.Fatalf("nil doc: want nil, got %v", v)
	}
}

func TestGateDAG(t *testing.T) {
	// Passing: linear deps, same/earlier wave.
	good := &ParsedTasks{Tasks: []ParsedTask{
		{ID: "T1", Wave: 1, Meta: map[string]string{}},
		{ID: "T2", Wave: 2, Meta: map[string]string{"depends": "T1"}},
	}}
	if v, _ := GateDAG(CheckCtx{Doc: good}); len(v) != 0 {
		t.Fatalf("good dag: want 0 violations, got %v", v)
	}
	// Violating: orphan dependency on a missing task.
	bad := &ParsedTasks{Tasks: []ParsedTask{
		{ID: "T1", Wave: 1, Meta: map[string]string{"depends": "T99"}},
	}}
	v, _ := GateDAG(CheckCtx{Doc: bad})
	if len(v) != 1 || v[0].Gate != "dag" {
		t.Fatalf("bad dag: want 1 dag violation, got %v", v)
	}
}

func TestGateSync(t *testing.T) {
	doc := &ParsedTasks{Tasks: []ParsedTask{{ID: "T1", Line: 3, Checked: true, Meta: map[string]string{}}}}
	// Passing: checkbox checked, state complete.
	st := &State{Tasks: map[string]TaskState{"T1": {Status: TaskComplete}}}
	if v, _ := GateSync(CheckCtx{Doc: doc, State: st}); len(v) != 0 {
		t.Fatalf("synced: want 0 violations, got %v", v)
	}
	// Violating: checkbox checked but state pending → drift.
	st2 := &State{Tasks: map[string]TaskState{"T1": {Status: TaskPending}}}
	if v, _ := GateSync(CheckCtx{Doc: doc, State: st2}); len(v) != 1 || v[0].Gate != "sync" {
		t.Fatalf("drift: want 1 sync violation, got %v", v)
	}
}

func TestGateTraceability(t *testing.T) {
	req := "## Requirement 1\n\n1. the system shall do a thing\n"
	doc := &ParsedTasks{Tasks: []ParsedTask{
		{ID: "T1", Line: 3, Meta: map[string]string{"requirements": "1"}},
	}}
	// Passing: requirement 1 referenced, no undefined refs.
	if v, w := GateTraceability(CheckCtx{Doc: doc, ReqMd: &req}); len(v) != 0 || len(w) != 0 {
		t.Fatalf("traced: want 0/0, got v=%v w=%v", v, w)
	}
	// Violating: task references requirement 9 which is not defined.
	bad := &ParsedTasks{Tasks: []ParsedTask{
		{ID: "T1", Line: 3, Meta: map[string]string{"requirements": "9"}},
	}}
	v, _ := GateTraceability(CheckCtx{Doc: bad, ReqMd: &req})
	found := false
	for _, x := range v {
		if x.Gate == "traceability" && strings.Contains(x.Message, "requirement 9") {
			found = true
		}
	}
	if !found {
		t.Fatalf("undefined ref: want traceability violation for req 9, got %v", v)
	}
	// Unreferenced requirement is a warning under default config.
	unref := &ParsedTasks{Tasks: []ParsedTask{{ID: "T1", Line: 3, Meta: map[string]string{}}}}
	_, w := GateTraceability(CheckCtx{Doc: unref, ReqMd: &req})
	if len(w) != 1 {
		t.Fatalf("unreferenced: want 1 warning, got %v", w)
	}
	// Promoted to error when config says so.
	cfg := Config{Gates: GatesCfg{Traceability: "error"}}
	ev, ew := GateTraceability(CheckCtx{Doc: unref, ReqMd: &req, Cfg: cfg})
	if len(ev) != 1 || len(ew) != 0 {
		t.Fatalf("promoted: want 1 violation/0 warnings, got v=%v w=%v", ev, ew)
	}
}

func TestGateEvidence(t *testing.T) {
	ev := "manual proof"
	// Passing: read-only role complete with evidence, no verification needed.
	st := &State{Tasks: map[string]TaskState{
		"T1": {ID: "T1", Role: "reviewer", Status: TaskComplete, Evidence: &ev},
	}}
	if v, _ := GateEvidence(CheckCtx{State: st}); len(v) != 0 {
		t.Fatalf("readonly+evidence: want 0 violations, got %v", v)
	}
	// Violating: complete without evidence.
	st2 := &State{Tasks: map[string]TaskState{
		"T1": {ID: "T1", Role: "builder", Status: TaskComplete},
	}}
	if v, _ := GateEvidence(CheckCtx{State: st2}); len(v) != 1 || v[0].Gate != "evidence" {
		t.Fatalf("no evidence: want 1 evidence violation, got %v", v)
	}
	// Violating: builder complete with evidence but no verified record.
	st3 := &State{Tasks: map[string]TaskState{
		"T1": {ID: "T1", Role: "builder", Status: TaskComplete, Evidence: &ev},
	}}
	if v, _ := GateEvidence(CheckCtx{State: st3, Slug: "demo"}); len(v) != 1 {
		t.Fatalf("unverified builder: want 1 evidence violation, got %v", v)
	}
}

func TestBlockerHelpers(t *testing.T) {
	s := &State{Blockers: []Blocker{{Task: "T1", Reason: "a"}, {Task: "T2", Reason: "b"}}}
	RemoveBlocker(s, "T1")
	if len(s.Blockers) != 1 || s.Blockers[0].Task != "T2" {
		t.Fatalf("RemoveBlocker: want only T2, got %v", s.Blockers)
	}
	// AddBlocker replaces any existing entry for the same task.
	AddBlocker(s, "T2", "new", 5)
	if len(s.Blockers) != 1 || s.Blockers[0].Reason != "new" || s.Blockers[0].Since != "Turn 5" {
		t.Fatalf("AddBlocker replace: got %v", s.Blockers)
	}
	AddBlocker(s, "T3", "c", 7)
	if len(s.Blockers) != 2 {
		t.Fatalf("AddBlocker append: want 2, got %v", s.Blockers)
	}
}

func TestRemoveBlockerNoAlias(t *testing.T) {
	orig := []Blocker{{Task: "T1"}, {Task: "T2"}}
	s := &State{Blockers: orig}
	RemoveBlocker(s, "T1")
	// Mutating the result must not corrupt the original backing array.
	if orig[0].Task != "T1" {
		t.Fatalf("RemoveBlocker aliased original backing array: %v", orig)
	}
}
