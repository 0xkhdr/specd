package core

import (
	"os"
	"path/filepath"
	"runtime"
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

// T4 / R2 + R3 regression closure. Asserts the core primitives that back the
// phase/gate state machine and evidence-gated task flips. Full CLI-side gate
// blocking (`specd approve`/`specd task` refusing while awaiting-approval) and
// the `--unverified` bypass live in internal/cmd and are owned by the
// regression-cli-cmd spec; here we lock the engine behaviors those depend on.

// R2.1: PhaseReadiness is the planning gate — advancement is blocked (non-empty
// problems) until the phase artifact passes, and permitted (empty) once it does.
func TestPhaseReadinessBlocksUntilClean(t *testing.T) {
	validReq := "## Requirement 1\n**User story:** As a user, I want X.\n\n**Acceptance criteria:**\n1. WHEN a thing happens THE SYSTEM SHALL do Y\n"
	empty := ""

	// requirements: missing/empty blocks; EARS-valid clears.
	if p := PhaseReadiness(StatusRequirements, &empty, nil, ParsedTasks{}); len(p) == 0 {
		t.Error("R2.1: empty requirements must block advancement")
	}
	if p := PhaseReadiness(StatusRequirements, &validReq, nil, ParsedTasks{}); len(p) != 0 {
		t.Errorf("R2.1: valid requirements must clear, got %v", p)
	}

	// design: missing design.md blocks.
	if p := PhaseReadiness(StatusDesign, &validReq, nil, ParsedTasks{}); len(p) == 0 {
		t.Error("R2.1: missing design must block advancement")
	}

	// tasks: a cyclic task graph blocks.
	cyclic := ParsedTasks{Tasks: []ParsedTask{
		{ID: "T1", Wave: 1, Meta: map[string]string{"depends": "T2"}},
		{ID: "T2", Wave: 1, Meta: map[string]string{"depends": "T1"}},
	}}
	p := PhaseReadiness(StatusTasks, &validReq, &validReq, cyclic)
	if len(p) == 0 {
		t.Error("R2.1: cyclic tasks must block advancement")
	}
}

// R2.1 (data invariant): the planning ratchet is forward-only and offers no
// transition out of post-approval statuses — the engine cannot skip a gate.
func TestPhaseAdvanceIsForwardOnlyRatchet(t *testing.T) {
	if PlanningAdvance[StatusRequirements].Status != StatusDesign {
		t.Error("requirements must advance to design")
	}
	if PlanningAdvance[StatusDesign].Status != StatusTasks {
		t.Error("design must advance to tasks")
	}
	if PlanningAdvance[StatusTasks].Status != StatusExecuting {
		t.Error("tasks must advance to executing")
	}
	for _, terminal := range []SpecStatus{StatusExecuting, StatusVerifying, StatusComplete} {
		if _, ok := PlanningAdvance[terminal]; ok {
			t.Errorf("status %s must have no planning advance (gate must intervene)", terminal)
		}
	}
}

// R2.2: a builder task flipped complete without evidence is rejected by the
// evidence gate; with evidence but no verified record it is still rejected.
func TestGateEvidenceRejectsUnproven(t *testing.T) {
	ev := "proof"
	noEvidence := &State{Tasks: map[string]TaskState{
		"T1": {ID: "T1", Role: "builder", Status: TaskComplete},
	}}
	if v, _ := GateEvidence(CheckCtx{State: noEvidence}); len(v) != 1 || v[0].Gate != "evidence" {
		t.Fatalf("R2.2: complete-without-evidence must be 1 evidence violation, got %v", v)
	}
	noVerified := &State{Tasks: map[string]TaskState{
		"T1": {ID: "T1", Role: "builder", Status: TaskComplete, Evidence: &ev},
	}}
	if v, _ := GateEvidence(CheckCtx{State: noVerified, Slug: "demo"}); len(v) != 1 {
		t.Fatalf("R2.2: evidence-but-unverified builder must be rejected, got %v", v)
	}
}

// R2.3: SaveState persists revision strictly monotonically across writes and
// never decreases or repeats.
func TestPhaseStateRevisionMonotonic(t *testing.T) {
	dir := t.TempDir()
	slug := "rev-spec"
	if err := os.MkdirAll(filepath.Join(dir, ".specd", "specs", slug), 0o755); err != nil {
		t.Fatal(err)
	}
	st := InitialState(slug, "Rev Spec")
	prev := st.Revision
	for i := 0; i < 5; i++ {
		if err := SaveState(dir, slug, &st); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
		if st.Revision != prev+1 {
			t.Fatalf("revision not monotonic: got %d, want %d", st.Revision, prev+1)
		}
		prev = st.Revision
	}
}

// R2.4: custom gates run after the ordered core pipeline and in configured
// order — the gate listing order is a stable contract.
func TestCustomGatePipelineOrder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("custom gate uses POSIX sh")
	}
	baseV, _ := RunGates(minimalCtx(nil))
	two := []CustomGateCfg{
		{Name: "first", Command: `echo '{"violations":[{"location":"a","message":"1"}]}'`},
		{Name: "second", Command: `echo '{"violations":[{"location":"b","message":"2"}]}'`},
	}
	v, _ := RunGates(minimalCtx(two))
	if len(v) != len(baseV)+2 {
		t.Fatalf("R2.4: want %d violations, got %d", len(baseV)+2, len(v))
	}
	// Core findings precede custom; customs appear in config order.
	if v[len(v)-2].Gate != "custom:first" || v[len(v)-1].Gate != "custom:second" {
		t.Fatalf("R2.4: custom gate order wrong: %s then %s", v[len(v)-2].Gate, v[len(v)-1].Gate)
	}
}

// R3.1: a completed task's evidence and finish timestamp survive a
// save/load round-trip (the durable proof of a flip).
func TestTaskFlipPersistsEvidenceAndTimestamp(t *testing.T) {
	dir := t.TempDir()
	slug := "ev-spec"
	if err := os.MkdirAll(filepath.Join(dir, ".specd", "specs", slug), 0o755); err != nil {
		t.Fatal(err)
	}
	st := InitialState(slug, "Ev Spec")
	ev := "go test ./... → ok"
	ts := NowISO()
	st.Tasks = map[string]TaskState{
		"T1": {ID: "T1", Role: "builder", Status: TaskComplete, Evidence: &ev, FinishedAt: &ts},
	}
	if err := SaveState(dir, slug, &st); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadState(dir, slug)
	if err != nil {
		t.Fatal(err)
	}
	got := loaded.Tasks["T1"]
	if got.Evidence == nil || *got.Evidence != ev {
		t.Errorf("R3.1: evidence not persisted: %v", got.Evidence)
	}
	if got.FinishedAt == nil || *got.FinishedAt != ts {
		t.Errorf("R3.1: finish timestamp not persisted: %v", got.FinishedAt)
	}
}

// R3.3: telemetry annotations (tokens, cost) are stored verbatim and only
// summed — never priced or computed. The roll-up of stored per-task values
// must equal the literal inputs.
func TestTaskTelemetryStoredNotComputed(t *testing.T) {
	st := &State{Spec: "tel", Tasks: map[string]TaskState{
		"T1": {ID: "T1", Wave: 1, Telemetry: &Telemetry{Tokens: 1000, Cost: "0.42"}},
		"T2": {ID: "T2", Wave: 1, Telemetry: &Telemetry{Tokens: 234, Cost: "0.58"}},
	}}
	roll := RollupTelemetry(st)
	// Tokens are summed verbatim, not derived from any pricing model.
	if roll.Tokens != 1234 {
		t.Errorf("R3.3: tokens = %d, want 1234 (verbatim sum)", roll.Tokens)
	}
	// Cost is the sum of the annotated strings parsed as-is (0.42 + 0.58).
	if roll.Cost != 1.0 || !roll.CostAnnotated {
		t.Errorf("R3.3: cost = %v annotated=%v, want 1.0/true", roll.Cost, roll.CostAnnotated)
	}
}

// acceptanceReqMd is a requirements.md whose Requirement 1 has two acceptance
// criteria, giving stable ids "1.1" and "1.2".
const acceptanceReqMd = `## Requirement 1 — Login

**User story:** As a user, I want to log in.

**Acceptance criteria:**
1. When credentials are valid, the system shall grant access.
2. When credentials are invalid, the system shall deny access.
`

// acceptanceTasksMd maps T1 to criterion 1.1 via its acceptance metadata.
const acceptanceTasksMd = `# Tasks — Login

## Wave 1
- [ ] T1 — implement login
  - why: users need access
  - role: builder
  - files: internal/auth/login.go
  - contract: login works
  - acceptance: 1.1=TestLoginValid
  - verify: go test ./internal/auth/
  - depends: —
  - requirements: 1
`

func mustParse(t *testing.T, md string) *ParsedTasks {
	t.Helper()
	doc, err := ParseTasks(md)
	if err != nil {
		t.Fatalf("ParseTasks: %v", err)
	}
	return &doc
}

func TestGateAcceptanceOffIsNoop(t *testing.T) {
	doc := mustParse(t, acceptanceTasksMd)
	st := &State{Tasks: map[string]TaskState{"T1": {ID: "T1", Status: TaskComplete}}}
	for _, mode := range []string{"", "off"} {
		c := CheckCtx{ReqMd: strp(acceptanceReqMd), Doc: doc, State: st, Cfg: Config{Gates: GatesCfg{Acceptance: mode}}}
		v, w := GateAcceptance(c)
		if len(v) != 0 || len(w) != 0 {
			t.Fatalf("acceptance=%q: want no findings, got v=%v w=%v", mode, v, w)
		}
	}
}

func TestGateAcceptanceCompleteWithoutPass(t *testing.T) {
	doc := mustParse(t, acceptanceTasksMd)
	st := &State{Tasks: map[string]TaskState{"T1": {ID: "T1", Status: TaskComplete}}}

	// error mode: missing recorded pass is a violation.
	c := CheckCtx{Slug: "demo", ReqMd: strp(acceptanceReqMd), Doc: doc, State: st, Cfg: Config{Gates: GatesCfg{Acceptance: "error"}}}
	v, _ := GateAcceptance(c)
	if len(v) != 1 || v[0].Gate != "acceptance" {
		t.Fatalf("want 1 acceptance violation, got %v", v)
	}

	// warn mode: same finding demoted to a warning.
	c.Cfg.Gates.Acceptance = "warn"
	v, w := GateAcceptance(c)
	if len(v) != 0 || len(w) != 1 {
		t.Fatalf("warn: want 0 violations / 1 warning, got v=%v w=%v", v, w)
	}

	// Recording the pass clears the finding.
	st.Acceptance = map[string]CriterionRecord{"1.1": {Status: "pass"}}
	c.Cfg.Gates.Acceptance = "error"
	v, _ = GateAcceptance(c)
	if len(v) != 0 {
		t.Fatalf("with recorded pass: want 0 violations, got %v", v)
	}
}

func TestGateAcceptanceUndefinedCriterionAlwaysError(t *testing.T) {
	// acceptance maps to 9.9 which is not in requirements.md.
	md := "# Tasks — X\n\n## Wave 1\n- [ ] T1 — x\n  - why: w\n  - role: builder\n  - files: a.go\n  - contract: c\n  - acceptance: 9.9=TestX\n  - verify: go test ./\n  - depends: —\n  - requirements: 1\n"
	doc := mustParse(t, md)
	st := &State{Tasks: map[string]TaskState{"T1": {ID: "T1", Status: TaskPending}}}
	// Even in warn mode, a broken reference is a hard violation.
	c := CheckCtx{Slug: "demo", ReqMd: strp(acceptanceReqMd), Doc: doc, State: st, Cfg: Config{Gates: GatesCfg{Acceptance: "warn"}}}
	v, _ := GateAcceptance(c)
	if len(v) != 1 {
		t.Fatalf("want 1 hard violation for undefined criterion, got %v", v)
	}
}

func scopeTasksMd(files string) string {
	return "# Tasks — X\n\n## Wave 1\n- [ ] T1 — x\n  - why: w\n  - role: builder\n  - files: " + files +
		"\n  - contract: c\n  - acceptance: —\n  - verify: go test ./\n  - depends: —\n  - requirements: 1\n"
}

func TestGateScopeOffIsNoop(t *testing.T) {
	doc := mustParse(t, scopeTasksMd("internal/core/login.go"))
	st := &State{Tasks: map[string]TaskState{"T1": {ID: "T1", Verification: &VerificationRecord{ChangedFiles: []string{"totally/unrelated.go"}}}}}
	for _, mode := range []string{"", "off", "*"} {
		c := CheckCtx{Doc: doc, State: st, Cfg: Config{Gates: GatesCfg{Scope: mode}}}
		v, w := GateScope(c)
		if len(v) != 0 || len(w) != 0 {
			t.Fatalf("scope=%q: want no findings, got v=%v w=%v", mode, v, w)
		}
	}
}

func TestGateScopeFlagsOutOfContract(t *testing.T) {
	doc := mustParse(t, scopeTasksMd("internal/core/*.go"))
	st := &State{Tasks: map[string]TaskState{"T1": {ID: "T1", Verification: &VerificationRecord{
		ChangedFiles: []string{"internal/core/login.go", "cmd/main.go"},
	}}}}
	c := CheckCtx{Doc: doc, State: st, Cfg: Config{Gates: GatesCfg{Scope: "error"}}}
	v, _ := GateScope(c)
	if len(v) != 1 || v[0].Gate != "scope" {
		t.Fatalf("want 1 scope violation for cmd/main.go, got %v", v)
	}

	// A "*" contract opts the task out individually.
	doc2 := mustParse(t, scopeTasksMd("*"))
	c.Doc = doc2
	v, _ = GateScope(c)
	if len(v) != 0 {
		t.Fatalf("star contract: want 0 violations, got %v", v)
	}
}

func TestMatchesAnyGlob(t *testing.T) {
	cases := []struct {
		p    string
		pats []string
		want bool
	}{
		{"internal/core/x.go", []string{"internal/core/*.go"}, true},
		{"internal/core/sub/x.go", []string{"internal/core/*.go"}, false},
		{"internal/core/sub/x.go", []string{"internal/core/**"}, true},
		{"internal/core/x.go", []string{"internal/core"}, true},
		{"docs/x.md", []string{"internal/core"}, false},
		{"a.go", []string{"a.go"}, true},
	}
	for _, tc := range cases {
		if got := matchesAnyGlob(tc.p, tc.pats); got != tc.want {
			t.Errorf("matchesAnyGlob(%q,%v)=%v want %v", tc.p, tc.pats, got, tc.want)
		}
	}
}

func TestGateContextBudget(t *testing.T) {
	st := &State{Status: StatusExecuting, Tasks: map[string]TaskState{}}

	// Off by default: no config severity => no-op (core pipeline unchanged).
	if v, w := GateContextBudget(CheckCtx{State: st}); len(v) != 0 || len(w) != 0 {
		t.Fatalf("default: want 0/0, got v=%v w=%v", v, w)
	}
	if v, w := GateContextBudget(CheckCtx{State: st, Cfg: Config{Gates: GatesCfg{ContextBudget: "off"}}}); len(v) != 0 || len(w) != 0 {
		t.Fatalf("off: want 0/0, got v=%v w=%v", v, w)
	}

	// Under budget: enabled with the derived ceiling, required estimate fits.
	under := Config{Gates: GatesCfg{ContextBudget: "error"}}
	if v, w := GateContextBudget(CheckCtx{State: st, Cfg: under}); len(v) != 0 || len(w) != 0 {
		t.Fatalf("under budget: want 0/0, got v=%v w=%v", v, w)
	}

	// Over budget (error): a tight host cap forces the required estimate over the
	// effective budget; the gate fails and names the heaviest required item.
	over := Config{Gates: GatesCfg{ContextBudget: "error", MaxContextTokens: 2000}}
	v, w := GateContextBudget(CheckCtx{State: st, Cfg: over})
	if len(v) != 1 || len(w) != 0 || v[0].Gate != "context-budget" {
		t.Fatalf("over budget error: want 1 context-budget violation/0 warnings, got v=%v w=%v", v, w)
	}
	if !strings.Contains(v[0].Message, "phase-skill") || !strings.Contains(v[0].Message, "exceeds budget") {
		t.Fatalf("over budget message should name heaviest item: %q", v[0].Message)
	}

	// Over budget (warn): same overflow folds into a warning, not a violation.
	warn := Config{Gates: GatesCfg{ContextBudget: "warn", MaxContextTokens: 2000}}
	wv, ww := GateContextBudget(CheckCtx{State: st, Cfg: warn})
	if len(wv) != 0 || len(ww) != 1 {
		t.Fatalf("over budget warn: want 0 violations/1 warning, got v=%v w=%v", wv, ww)
	}
}
