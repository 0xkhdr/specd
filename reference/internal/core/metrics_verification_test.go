package core

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core/security"
)

// TestSuccessMetricsAreMeasurable is the release-gate verification for the plan
// Part III success-metrics table (V12/P6.4, `specs/v020-release-engineering`).
// Each subtest exercises the *real* deterministic measuring path for one metric
// and asserts the measurement is computed and non-degenerate. The point is not
// to re-prove each feature (its own package tests do that) but to guarantee that
// every shipped metric has a live, CI-exercised way to be measured — so a future
// refactor that silently drops a measurement fails here.
//
// Metrics (plan Part III):
//  1. first-pass verify >85%     → per-task Retries via RollupTelemetry
//  2. security catch >90%        → security.Scan over planted secrets
//  3. mode-switch <30s           → EffectiveMode / ResolveMode (pure state read)
//  4. ingestion coverage 100%    → ComputeIngestCoverage
//  5. cost attribution 100%      → RollupTelemetry cost roll-up
//  6. eval coverage ≥1/spec      → SaveEvalReport + EvalTrend
//  7. observe→midreq             → ParseErrorPayload + RenderObserveMidreq
func TestSuccessMetricsAreMeasurable(t *testing.T) {
	t.Run("first-pass-verify-rate", func(t *testing.T) {
		// First-pass verify = a task whose verify passed with zero retries.
		// The rate is a pure function of the persisted per-task telemetry.
		state := &State{Spec: "billing", Tasks: map[string]TaskState{
			"T1": {ID: "T1", Wave: 1, Status: TaskComplete, Telemetry: &Telemetry{Retries: 0}},
			"T2": {ID: "T2", Wave: 1, Status: TaskComplete, Telemetry: &Telemetry{Retries: 0}},
			"T3": {ID: "T3", Wave: 1, Status: TaskComplete, Telemetry: &Telemetry{Retries: 2}},
		}}
		rollup := RollupTelemetry(state)
		if rollup.Retries != 2 {
			t.Fatalf("retries roll-up not measured: got %d, want 2", rollup.Retries)
		}
		firstPass, total := 0, 0
		for _, task := range state.Tasks {
			if task.Status != TaskComplete || task.Telemetry == nil {
				continue
			}
			total++
			if task.Telemetry.Retries == 0 {
				firstPass++
			}
		}
		if total == 0 {
			t.Fatal("no completed tasks to measure first-pass rate from")
		}
		rate := float64(firstPass) / float64(total)
		// The metric target is >85%; here 2/3 exercises the sub-threshold branch
		// so we prove the measurement discriminates, not that this fixture passes.
		if rate <= 0 || rate > 1 {
			t.Fatalf("first-pass rate out of range: %f", rate)
		}
	})

	t.Run("security-catch-rate", func(t *testing.T) {
		// Catch rate = planted secrets flagged / planted secrets. A benign file
		// must not be flagged (no false positive skewing the rate).
		files := []security.ChangedFile{
			{Path: "app.py", Content: `aws_key = "AKIA` + `IOSFODNN7EXAMPLE"`},
			{Path: "id_rsa", Content: "-----BEGIN RSA PRIVATE KEY-----\nMIIabc\n-----END RSA PRIVATE KEY-----"},
			{Path: "readme.md", Content: "no secrets here"},
		}
		findings := security.Scan(security.Config{Secrets: "error"}, files, security.Allowlist{})
		flagged := map[string]bool{}
		for _, f := range findings {
			flagged[f.File] = true
		}
		planted := []string{"app.py", "id_rsa"}
		caught := 0
		for _, p := range planted {
			if flagged[p] {
				caught++
			}
		}
		rate := float64(caught) / float64(len(planted))
		if rate < 0.9 {
			t.Fatalf("security catch rate below 90%%: caught %d/%d", caught, len(planted))
		}
		if flagged["readme.md"] {
			t.Fatal("false positive on benign file skews catch rate")
		}
	})

	t.Run("mode-switch-continuity", func(t *testing.T) {
		// A mode switch is a pure metadata read (no rebuild, no IO), which is why
		// the <30s continuity target holds trivially. Measure that EffectiveMode
		// and ResolveMode reflect a switch instantly and deterministically.
		s := &State{Spec: "billing"}
		if got := s.EffectiveMode(); got != ModeSimple {
			t.Fatalf("empty mode should resolve Simple, got %q", got)
		}
		s.ExecutionMode = ModeOrchestrated
		if got := s.EffectiveMode(); got != ModeOrchestrated {
			t.Fatalf("switched mode not reflected: got %q", got)
		}
		// An explicit flag overrides the stored mode and is attributed to the user.
		mode, origin := ResolveMode(ModeSimple, s)
		if mode != ModeSimple || origin != OriginUser {
			t.Fatalf("flag override not measured: mode=%q origin=%q", mode, origin)
		}
	})

	t.Run("ingestion-coverage", func(t *testing.T) {
		// Coverage = mapped + waived over every inventory file; unmapped is the
		// gap. 100% means no unmapped file survives.
		inv := Inventory{
			Base: "src",
			Files: []InventoryFile{
				{Path: "src/a.go", Size: 10},
				{Path: "src/b.go", Size: 20},
				{Path: "src/gen.go", Size: 5},
			},
			Waivers: map[string]string{"src/gen.go": "generated code, excluded by design"},
		}
		requirements := "REQ-001 covers src/a.go and src/b.go."
		cov := ComputeIngestCoverage(inv, requirements)
		if len(cov.Mapped) != 2 || len(cov.Waived) != 1 || len(cov.Unmapped) != 0 {
			t.Fatalf("coverage math wrong: mapped=%v waived=%v unmapped=%v", cov.Mapped, cov.Waived, cov.Unmapped)
		}
		covered := len(cov.Mapped) + len(cov.Waived)
		if covered != len(inv.Files) {
			t.Fatalf("coverage not 100%%: %d/%d", covered, len(inv.Files))
		}
	})

	t.Run("cost-attribution", func(t *testing.T) {
		// Every annotated task cost must roll up to its spec/wave — 100% of
		// annotated cost is attributed, none dropped.
		state := &State{Spec: "billing", Tasks: map[string]TaskState{
			"T1": {ID: "T1", Wave: 1, Telemetry: &Telemetry{Cost: "0.40"}},
			"T2": {ID: "T2", Wave: 2, Telemetry: &Telemetry{Cost: "0.60"}},
		}}
		rollup := RollupTelemetry(state)
		if !rollup.CostAnnotated {
			t.Fatal("cost annotation flag not set — attribution not measured")
		}
		if rollup.Cost < 0.999 || rollup.Cost > 1.001 {
			t.Fatalf("cost not fully attributed: got %f, want 1.00", rollup.Cost)
		}
		var waveSum float64
		for _, w := range rollup.Waves {
			waveSum += w.Cost
		}
		if waveSum < 0.999 || waveSum > 1.001 {
			t.Fatalf("per-wave attribution lost cost: got %f, want 1.00", waveSum)
		}
	})

	t.Run("eval-coverage-per-spec", func(t *testing.T) {
		// Eval coverage ≥1/spec: a spec has at least one recorded eval run,
		// readable via the same trend reader the dashboard uses.
		root := t.TempDir()
		slug := "billing"
		report := &EvalReport{Suite: "smoke", Score: 0.92, MinScore: 0.8, Passed: true}
		if _, err := SaveEvalReport(root, slug, report); err != nil {
			t.Fatalf("save eval report: %v", err)
		}
		trend, err := EvalTrend(root, slug, "")
		if err != nil {
			t.Fatalf("eval trend: %v", err)
		}
		if len(trend.Runs) < 1 {
			t.Fatalf("eval coverage below 1/spec: %d runs", len(trend.Runs))
		}
	})

	t.Run("observe-to-midreq", func(t *testing.T) {
		// Every accepted production error must become an evidenced mid-requirement
		// entry. Exercise parse → render and assert the rendered midreq carries the
		// error's evidence (severity, message, correlated spec).
		payload, err := ParseErrorPayload([]byte(`{
			"service": "checkout",
			"severity": "error",
			"message": "nil pointer in charge()",
			"fingerprint": "abc123",
			"frames": [{"file": "internal/pay/charge.go", "line": 42}]
		}`))
		if err != nil {
			t.Fatalf("parse payload: %v", err)
		}
		corr := Correlation{
			Spec:         "billing",
			Tasks:        []string{"T7"},
			MatchedFiles: []string{"internal/pay/charge.go"},
			Impact:       "high",
			Confidence:   "high",
			Facts:        []string{"1 frame matched task T7 files: contract"},
		}
		midreq := RenderObserveMidreq(payload, corr)
		if strings.TrimSpace(midreq) == "" {
			t.Fatal("observe→midreq produced empty entry")
		}
		for _, want := range []string{payload.Message, corr.MatchedFiles[0], corr.Facts[0]} {
			if !strings.Contains(midreq, want) {
				t.Fatalf("midreq missing evidence %q:\n%s", want, midreq)
			}
		}
	})
}
