package core

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPortfolioStatus(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	inputs := []PortfolioGovernanceInput{
		{SpecID: "zeta", Complete: true, Risk: RiskUnknown, Owner: "unknown", Governance: []GovernanceItem{{ID: "d2", Status: GovernanceAccepted, ReviewAt: "2026-07-12T00:00:00Z"}}, ProductionSignals: []ProductionSignal{{ID: "sig-b", Status: SignalUnresolved}}},
		{SpecID: "alpha", Risk: RiskHigh, Owner: "team-a", Governance: []GovernanceItem{{ID: "e1", Status: GovernanceExpired}, {ID: "d1", Status: GovernanceAccepted, ExpiresAt: "2026-07-12T00:00:00Z"}}},
	}
	got, err := BuildPortfolioGovernanceStatus(Program{}, inputs, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Specs) != 2 || got.Specs[0].SpecID != "alpha" || got.Specs[1].SpecID != "zeta" {
		t.Fatalf("unstable specs: %+v", got.Specs)
	}
	if got.Specs[0].Risk != RiskHigh || got.Specs[1].Risk != RiskUnknown {
		t.Fatalf("risk lost: %+v", got.Specs)
	}
	if !reflect.DeepEqual(got.Specs[0].StaleGovernance, []string{"d1", "e1"}) || !reflect.DeepEqual(got.Specs[1].StaleGovernance, []string{"d2"}) {
		t.Fatalf("stale governance = %+v", got.Specs)
	}
	if !reflect.DeepEqual(got.Specs[1].UnresolvedSignals, []string{"sig-b"}) {
		t.Fatalf("signals = %+v", got.Specs[1])
	}
	if inputs[0].SpecID != "zeta" {
		t.Fatal("projection mutated input")
	}
}

func TestSharedOutcome(t *testing.T) {
	p := Program{Links: []ProgramLink{{From: "app", To: "api", Kind: LinkKindFollows}}}
	got, err := BuildPortfolioGovernanceStatus(p, []PortfolioGovernanceInput{
		{SpecID: "api", Complete: true, Risk: RiskLow, Owner: "api-team"},
		{SpecID: "app", Complete: true, Risk: RiskLow, Owner: "app-team", SharedOutcomes: []SharedOutcome{{ID: "release-1", Status: OutcomeFailed, EvidenceRef: "evidence/release-1"}}},
	}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Blockers) != 0 {
		t.Fatalf("DAG should be satisfied: %+v", got.Blockers)
	}
	if got.Complete || len(got.Specs[1].IncompleteOutcomes) != 1 || got.Specs[1].IncompleteOutcomes[0] != "release-1" {
		t.Fatalf("failed shared outcome hidden: %+v", got)
	}
}

func TestPortfolioScale(t *testing.T) {
	inputs := make([]PortfolioGovernanceInput, PortfolioScaleLimit)
	for i := range inputs {
		inputs[i] = PortfolioGovernanceInput{SpecID: fmt.Sprintf("spec-%05d", i), Complete: true, Risk: RiskUnknown, Owner: "unknown"}
	}
	got, err := BuildPortfolioGovernanceStatus(Program{}, inputs, time.Time{})
	if err != nil || len(got.Specs) != PortfolioScaleLimit {
		t.Fatalf("scale envelope: len=%d err=%v", len(got.Specs), err)
	}
	if _, err := BuildPortfolioGovernanceStatus(Program{}, append(inputs, PortfolioGovernanceInput{SpecID: "overflow", Risk: RiskUnknown, Owner: "unknown"}), time.Time{}); err == nil {
		t.Fatal("unbounded portfolio accepted")
	}
}

func TestPortfolioCardinalityBounds(t *testing.T) {
	items := make([]GovernanceItem, PortfolioItemLimit+1)
	for i := range items {
		items[i] = GovernanceItem{ID: fmt.Sprintf("g-%d", i), Status: GovernanceAccepted}
	}
	_, err := BuildPortfolioGovernanceStatus(Program{}, []PortfolioGovernanceInput{{SpecID: "demo", Risk: RiskLow, Owner: "team", Governance: items}}, time.Time{})
	if err == nil {
		t.Fatal("unbounded governance cardinality accepted")
	}
	rows := make([]PortfolioSpec, PortfolioScaleLimit+1)
	for i := range rows {
		rows[i].SpecID = fmt.Sprintf("spec-%05d", i)
	}
	if _, err := BuildPortfolioView(Program{}, rows); err == nil {
		t.Fatal("unbounded portfolio view accepted")
	}
}

func TestProgramEconomicRollupExactStableMissingAndDriftSources(t *testing.T) {
	inputs := []SpecEconomics{
		{SpecID: "zeta", Telemetry: &TelemetryReport{Cost: "0.20", InputTokens: 3}, SourceRefs: []string{"evidence:zeta:2"}},
		{SpecID: "alpha", Telemetry: nil},
		{SpecID: "beta", Telemetry: &TelemetryReport{Cost: "0.10", InputTokens: 0}, SourceRefs: []string{"evidence:beta:1"}, PreviousCost: "0.01"},
	}
	got, err := RollupEconomics(inputs, "0.05")
	if err != nil {
		t.Fatal(err)
	}
	if got.Cost != "0.3" || got.InputTokens != 3 {
		t.Fatalf("totals = %#v", got)
	}
	if !reflect.DeepEqual(got.MissingSpecs, []string{"alpha"}) {
		t.Fatalf("missing = %v", got.MissingSpecs)
	}
	if len(got.Specs) != 3 || got.Specs[0].SpecID != "alpha" || got.Specs[2].SpecID != "zeta" {
		t.Fatalf("unstable rows = %#v", got.Specs)
	}
	if len(got.Alerts) != 1 || !reflect.DeepEqual(got.Alerts[0].SourceRefs, []string{"evidence:beta:1"}) {
		t.Fatalf("alerts = %#v", got.Alerts)
	}
	if inputs[0].Telemetry.Cost != "0.20" {
		t.Fatal("roll-up mutated source")
	}
}

func TestProgramEconomicRollupRejectsDuplicateAndUnboundedDimensions(t *testing.T) {
	if _, err := RollupEconomics([]SpecEconomics{{SpecID: "a"}, {SpecID: "a"}}, ""); err == nil {
		t.Fatal("duplicate spec accepted")
	}
	if _, err := RollupEconomics([]SpecEconomics{{SpecID: "../secret"}}, ""); err == nil {
		t.Fatal("unbounded spec id accepted")
	}
}

// TestReopenRejected reproduces the R1.1 gap (09a/T04) and pins the guard that
// 09b/T09 lands: reopening, rewinding, or editing a `complete` spec must fail
// closed with a message directing the user to create a linked successor. Today
// AdvanceStatus rejects a backward move, but with a generic "cannot move status
// backward" message and no dedicated guard covering the edit / add-status paths
// — so the successor-directing assertion is RED. It is skipped (not left RED)
// per specs/prompt.md §5; 09b/T09 removes the skip and makes it GREEN.
func TestReopenRejected(t *testing.T) {
	_, err := AdvanceStatus(StatusComplete, StatusExecuting)
	if err == nil {
		t.Fatal("reopening a complete spec must fail closed")
	}
	if !strings.Contains(err.Error(), "successor") {
		t.Errorf("reopen error must direct to a linked successor, got %q", err.Error())
	}
}

func TestLinkKindDecode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "program.json")
	kindless := `{"schema_version":1,"links":[{"from":"next","to":"original"}]}` + "\n"
	if err := AtomicWrite(path, kindless); err != nil {
		t.Fatal(err)
	}
	program, err := LoadProgram(path)
	if err != nil {
		t.Fatalf("load program: %v", err)
	}
	if len(program.Links) != 1 || program.Links[0].Kind != LinkKindFollows {
		t.Fatalf("kindless link did not decode as follows: %+v", program.Links)
	}
	for _, kind := range []LinkKind{LinkKindFollows, LinkKindRegresses, LinkKindMaintains, LinkKindSupersedes} {
		if !kind.Valid() {
			t.Errorf("declared link kind %q is invalid", kind)
		}
	}

	bad := `{"schema_version":1,"links":[{"from":"next","to":"original","kind":"unknown"}]}` + "\n"
	if err := AtomicWrite(path, bad); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProgram(path); err == nil || !strings.Contains(err.Error(), "unknown link kind") {
		t.Fatalf("unknown kind must fail closed, got %v", err)
	}
}

func TestProgramCycleWithKinds(t *testing.T) {
	var p Program
	if err := p.AddTypedLink("a", "b", LinkKindSupersedes, "replacement"); err != nil {
		t.Fatal(err)
	}
	if err := p.AddTypedLink("b", "c", LinkKindMaintains, "maintenance"); err != nil {
		t.Fatal(err)
	}
	if got, want := p.WouldCycle("c", "a"), []string{"c", "a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("typed cycle = %v, want %v", got, want)
	}
	none := func(string) bool { return false }
	if got := p.IncompleteDeps("a", none); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("typed ordering changed: blockers = %v", got)
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

func TestProgramFeedbackLinksSuccessorWithoutMutatingCompletedHistory(t *testing.T) {
	p := Program{SchemaVersion: ProgramSchemaVersion, Links: []ProgramLink{{From: "checkout", To: "platform", Kind: LinkKindFollows}}}
	before := append([]ProgramLink(nil), p.Links...)
	complete := func(slug string) bool { return slug == "checkout" || slug == "platform" }
	if err := p.AddFeedbackLink("checkout-fix", "checkout", "runtime feedback rel-7", complete); err != nil {
		t.Fatal(err)
	}
	if len(p.Links) != 2 || p.Links[1].From != "checkout-fix" || p.Links[1].To != "checkout" || p.Links[1].Kind != LinkKindMaintains {
		t.Fatalf("maintenance edge not created: %+v", p.Links)
	}
	if p.Links[0] != before[0] {
		t.Fatalf("completed history mutated: before=%+v after=%+v", before, p.Links)
	}
}

func TestProgramFeedbackFailsClosedForMutableSourceOrCycle(t *testing.T) {
	complete := func(slug string) bool { return slug == "done" }
	p := Program{SchemaVersion: ProgramSchemaVersion}
	if err := p.AddFeedbackLink("fix", "active", "feedback", complete); err == nil {
		t.Fatal("active source accepted")
	}
	p.Links = []ProgramLink{{From: "done", To: "fix", Kind: LinkKindFollows}}
	if err := p.AddFeedbackLink("fix", "done", "feedback", complete); err == nil {
		t.Fatal("cycle accepted")
	}
}

func TestTypedLinkReplayConflict(t *testing.T) {
	p := Program{}
	if err := p.AddTypedLink("repair", "source", LinkKindMaintains, "same"); err != nil {
		t.Fatal(err)
	}
	if err := p.AddTypedLink("repair", "source", LinkKindMaintains, "same"); err != nil || len(p.Links) != 1 {
		t.Fatalf("exact replay: %v %+v", err, p.Links)
	}
	if err := p.AddTypedLink("repair", "source", LinkKindRegresses, "different"); err == nil {
		t.Fatal("metadata mismatch accepted")
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

func TestProgramStatusDerivesChainFromLifecycleState(t *testing.T) {
	var program Program
	program.AddLink("later", "middle")
	program.AddLink("middle", "root")
	states := []State{
		InitialState("later"),
		InitialState("root"),
		InitialState("middle"),
	}

	got := BuildProgramStatus(program, states)
	if !reflect.DeepEqual(got.Frontier, []string{"root"}) {
		t.Fatalf("frontier = %v, want [root]", got.Frontier)
	}
	if got.Specs[0].Complete || got.Specs[1].Complete || got.Specs[2].Complete {
		t.Fatalf("requirements-stage specs reported complete: %+v", got.Specs)
	}
	if got.Specs[0].Dependencies[0].Complete || got.Specs[1].Dependencies[0].Complete {
		t.Fatalf("requirements-stage dependency reported complete: %+v", got.Specs)
	}

	states[1].Status = StatusComplete
	states[1].Stage = StageComplete
	states[1].Condition = ConditionComplete
	states[1].Phase = PhaseReflect
	got = BuildProgramStatus(program, states)
	if !reflect.DeepEqual(got.Frontier, []string{"middle"}) || !got.Specs[1].Dependencies[0].Complete || !got.Specs[2].Complete {
		t.Fatalf("completed root did not advance the lifecycle projection: %+v", got)
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
