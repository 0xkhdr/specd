package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

func conformanceRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd", "specs", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// R7.1: every enumerated protocol event is recordable and round-trips.
func TestConformanceEventsRecordsEveryKind(t *testing.T) {
	root := conformanceRoot(t)

	for _, kind := range core.ConformanceEventKinds {
		if err := core.RecordConformanceEvent(root, "demo", core.ConformanceEvent{
			Kind: kind, TaskID: "T1", Operation: "complete-task", Detail: "observed",
		}); err != nil {
			t.Fatalf("record %s: %v", kind, err)
		}
	}

	events, err := core.LoadConformanceEvents(core.ConformancePath(root, "demo"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(events) != len(core.ConformanceEventKinds) {
		t.Fatalf("recorded %d events, want %d", len(events), len(core.ConformanceEventKinds))
	}
	seen := map[string]bool{}
	for _, event := range events {
		seen[event.Kind] = true
		if event.Slug != "demo" || event.Timestamp == "" || event.Actor == "" {
			t.Errorf("event is unattributable: %+v", event)
		}
	}
	for _, kind := range core.ConformanceEventKinds {
		if !seen[kind] {
			t.Errorf("kind %q did not survive the round trip", kind)
		}
	}
}

// R7.1 lists eight events. If one is added to the constant set without being
// added to ConformanceEventKinds it becomes unrecordable, which this catches.
func TestConformanceEventsKindSetIsComplete(t *testing.T) {
	want := []string{
		core.ConformanceActedWithoutAuthority,
		core.ConformanceContextAckSkipped,
		core.ConformanceDirectSpecdMutation,
		core.ConformanceHumanOnlyInvoked,
		core.ConformancePrematureCompletion,
		core.ConformanceStaleActionReplayed,
		core.ConformanceUndeclaredPathTouched,
		core.ConformanceWorkWithoutBootstrap,
	}
	if len(core.ConformanceEventKinds) != len(want) {
		t.Fatalf("kind set has %d entries, want the %d R7.1 events", len(core.ConformanceEventKinds), len(want))
	}
	for i, kind := range want {
		if core.ConformanceEventKinds[i] != kind {
			t.Errorf("kind %d = %q, want %q (order must stay stable)", i, core.ConformanceEventKinds[i], kind)
		}
		if !core.IsConformanceKind(kind) {
			t.Errorf("%q is not recognized as a conformance kind", kind)
		}
	}
	if core.IsConformanceKind("invented_event") {
		t.Error("an unknown kind was accepted")
	}
}

func TestConformanceEventsRefusesUnknownKind(t *testing.T) {
	root := conformanceRoot(t)
	err := core.RecordConformanceEvent(root, "demo", core.ConformanceEvent{Kind: "invented_event"})
	if err == nil {
		t.Fatal("an unknown event kind was recorded")
	}
	if refusal, ok := core.AsRefusal(err); !ok || refusal.Code != "CONFORMANCE_KIND_UNKNOWN" {
		t.Fatalf("got %v, want CONFORMANCE_KIND_UNKNOWN", err)
	}
}

// R7.2, the load-bearing test: removing the event log changes no gate outcome.
// Gates are run over a real spec with a full log present, then with the log
// deleted, and the findings must be identical.
func TestConformanceEventsRemovingLogChangesNoGateOutcome(t *testing.T) {
	root := conformanceRoot(t)
	dir := filepath.Join(root, ".specd", "specs", "demo")
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("requirements.md", "# Requirements — demo\n\n- **R1** When the log is removed, the system shall produce identical gate findings.\n")
	write("design.md", "# Design — demo\n\n## Modules\nConformance.\n\n## On-disk contracts\nA JSONL log.\n\n## Invariants\nObservational only.\n")

	// The fixture is deliberately flawed so the gates actually emit findings.
	// Comparing two empty finding sets would pass even if a gate did read the
	// log, which is the failure this test exists to rule out: T1 is a write task
	// with a trivial verify line, and T2 cites a requirement that does not exist.
	tasks := []core.TaskRow{
		{ID: "T1", Role: "craftsman", Files: "thing.go", DeclaredFiles: []string{"thing.go"}, Verify: "printf ok", Acceptance: "R1"},
		{ID: "T2", Role: "craftsman", Files: "other.go", DeclaredFiles: []string{"other.go"}, Verify: "go test ./...", Acceptance: "R99"},
	}
	ctx := gates.CheckCtx{
		Root: root, Slug: "demo", Tasks: tasks, MaxContextTokens: 10000,
		TrivialVerify: core.DefaultTrivialVerify,
	}

	// Record every violation the protocol knows about, so if any gate were
	// reading this log it would have the most to react to.
	for _, kind := range core.ConformanceEventKinds {
		if err := core.RecordConformanceEvent(root, "demo", core.ConformanceEvent{Kind: kind, TaskID: "T1"}); err != nil {
			t.Fatal(err)
		}
	}
	withLog := gates.CoreRegistry().Run(ctx)
	if len(withLog) == 0 {
		t.Fatal("fixture produced no gate findings; comparing two empty sets would pass vacuously")
	}

	if err := os.Remove(core.ConformancePath(root, "demo")); err != nil {
		t.Fatal(err)
	}
	withoutLog := gates.CoreRegistry().Run(ctx)

	if len(withLog) != len(withoutLog) {
		t.Fatalf("gate findings changed with the log removed: %d then %d", len(withLog), len(withoutLog))
	}
	for i := range withLog {
		if withLog[i] != withoutLog[i] {
			t.Fatalf("finding %d changed with the log removed:\n with: %+v\n without: %+v", i, withLog[i], withoutLog[i])
		}
	}
}

// R7.2 structurally: no gate may reference the conformance log at all. The
// behavioural test above proves today's gates ignore it; this one fails the
// moment someone wires one up, which is the edit that would quietly turn an
// observation into an enforcement path.
func TestConformanceEventsNoGateReferencesTheLog(t *testing.T) {
	entries, err := os.ReadDir(filepath.FromSlash("gates"))
	if err != nil {
		t.Fatalf("read gates package: %v", err)
	}
	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("gates", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for _, banned := range []string{"Conformance", "conformance.jsonl"} {
			if strings.Contains(string(raw), banned) {
				t.Errorf("gates/%s references %q; conformance events must stay observational (R7.2)", entry.Name(), banned)
			}
		}
	}
	if checked == 0 {
		t.Fatal("scanned no gate source files; the check would pass vacuously")
	}
}

// A corrupt line must not lose the intact observations around it.
func TestConformanceEventsSkipsMalformedLines(t *testing.T) {
	root := conformanceRoot(t)
	if err := core.RecordConformanceEvent(root, "demo", core.ConformanceEvent{Kind: core.ConformanceHumanOnlyInvoked}); err != nil {
		t.Fatal(err)
	}
	if err := core.AppendFile(core.ConformancePath(root, "demo"), "{not json\n"); err != nil {
		t.Fatal(err)
	}
	if err := core.RecordConformanceEvent(root, "demo", core.ConformanceEvent{Kind: core.ConformanceDirectSpecdMutation}); err != nil {
		t.Fatal(err)
	}

	events, err := core.LoadConformanceEvents(core.ConformancePath(root, "demo"))
	if err != nil {
		t.Fatalf("a corrupt line broke the reader: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want the 2 intact ones", len(events))
	}
}

// A spec nobody violated has an empty history, not an error.
func TestConformanceEventsMissingLogIsEmptyHistory(t *testing.T) {
	root := conformanceRoot(t)
	events, err := core.LoadConformanceEvents(core.ConformancePath(root, "demo"))
	if err != nil {
		t.Fatalf("missing log returned an error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("got %d events from a missing log", len(events))
	}
}

func TestConformanceEventsSummaryReportsZeroKinds(t *testing.T) {
	summary := core.ConformanceSummary([]core.ConformanceEvent{
		{Kind: core.ConformanceHumanOnlyInvoked},
		{Kind: core.ConformanceHumanOnlyInvoked},
	})
	if len(summary) != len(core.ConformanceEventKinds) {
		t.Fatalf("summary has %d kinds, want every kind so zero differs from untracked", len(summary))
	}
	if summary[core.ConformanceHumanOnlyInvoked] != 2 {
		t.Errorf("count = %d, want 2", summary[core.ConformanceHumanOnlyInvoked])
	}
	if summary[core.ConformancePrematureCompletion] != 0 {
		t.Errorf("unseen kind = %d, want an explicit 0", summary[core.ConformancePrematureCompletion])
	}
}
