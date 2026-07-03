package core

import (
	"strings"
	"testing"
)

// seedDashboardSpec writes a spec state with the given tasks/evals/escalation so
// the dashboard aggregator has something to project.
func seedDashboardSpec(t *testing.T, root, slug string, mutate func(*State)) {
	t.Helper()
	st := InitialState(slug, slug)
	st.Title = strings.ToUpper(slug)
	st.Status = StatusExecuting
	if mutate != nil {
		mutate(&st)
	}
	if err := SaveState(root, slug, &st); err != nil {
		t.Fatal(err)
	}
}

func TestBuildDashboardProjectsSpecs(t *testing.T) {
	root := t.TempDir()
	seedDashboardSpec(t, root, "billing", func(s *State) {
		s.Tasks = map[string]TaskState{
			"T1": {ID: "T1", Wave: 1, Status: TaskComplete},
			"T2": {ID: "T2", Wave: 1, Status: TaskPending},
			"T3": {ID: "T3", Wave: 2, Status: TaskPending},
		}
		s.Evals = map[string]EvalSummary{"smoke": {Suite: "smoke", Score: 0.9, MinScore: 0.8}}
		s.Escalation = &EscalationRecord{Task: "T2", Time: "2026-07-03T00:00:00Z"}
	})
	seedDashboardSpec(t, root, "auth", nil)

	d, err := BuildDashboard(root, "all")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(d.Specs) != 2 {
		t.Fatalf("got %d specs, want 2", len(d.Specs))
	}
	// Deterministic slug order: auth before billing.
	if d.Specs[0].Slug != "auth" || d.Specs[1].Slug != "billing" {
		t.Fatalf("specs not sorted: %s, %s", d.Specs[0].Slug, d.Specs[1].Slug)
	}
	b := d.Specs[1]
	if b.TasksDone != 1 || b.TasksTotal != 3 {
		t.Fatalf("billing tasks = %d/%d, want 1/3", b.TasksDone, b.TasksTotal)
	}
	if len(b.Waves) != 2 || b.Waves[0].Wave != 1 || b.Waves[0].Done != 1 || b.Waves[0].Total != 2 {
		t.Fatalf("waves projection wrong: %+v", b.Waves)
	}
	if len(b.Evals) != 1 || b.Evals[0].Suite != "smoke" {
		t.Fatalf("evals projection wrong: %+v", b.Evals)
	}
	if b.Escalation == nil || b.Escalation.Task != "T2" {
		t.Fatalf("escalation projection wrong: %+v", b.Escalation)
	}
}

func TestBuildDashboardRejectsUnknownMode(t *testing.T) {
	root := t.TempDir()
	if _, err := BuildDashboard(root, "nonsense"); err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestRenderDashboardHTMLDeterministicAndFiltered(t *testing.T) {
	root := t.TempDir()
	seedDashboardSpec(t, root, "billing", func(s *State) {
		s.Tasks = map[string]TaskState{"T1": {ID: "T1", Wave: 1, Status: TaskComplete}}
		s.Evals = map[string]EvalSummary{"smoke": {Suite: "smoke", Score: 0.9, MinScore: 0.8}}
		s.Conductor = &ConductorSession{SessionID: "sess-1", Task: "T1"}
		s.Escalation = &EscalationRecord{Task: "T1", Time: "2026-07-03T00:00:00Z"}
	})

	all, _ := BuildDashboard(root, "all")
	h1 := RenderDashboardHTML(all, 0)
	h2 := RenderDashboardHTML(all, 0)
	if h1 != h2 {
		t.Fatal("render is not deterministic")
	}
	for _, want := range []string{"Specs &amp; waves", "Conductor sessions", "Cost attribution", "Eval trends", "Escalations"} {
		if !strings.Contains(h1, want) {
			t.Fatalf("mode=all missing panel %q", want)
		}
	}

	// mode=cost renders only the cost panel among the mode-gated ones.
	cost, _ := BuildDashboard(root, "cost")
	hc := RenderDashboardHTML(cost, 0)
	if !strings.Contains(hc, "Cost attribution") {
		t.Fatal("mode=cost missing cost panel")
	}
	if strings.Contains(hc, "Eval trends") || strings.Contains(hc, "Conductor sessions") {
		t.Fatal("mode=cost leaked other panels")
	}
}

func TestNormalizeDashboardMode(t *testing.T) {
	for _, in := range []string{"", "all", "CONDUCTOR", " cost ", "eval", "orchestrator"} {
		if _, ok := NormalizeDashboardMode(in); !ok {
			t.Errorf("valid mode %q rejected", in)
		}
	}
	if _, ok := NormalizeDashboardMode("bogus"); ok {
		t.Error("bogus mode accepted")
	}
}
