package core

import "testing"

// TestEvalTrend covers the eval-history reader that feeds the V11 dashboard eval
// panel: score deltas across sequential runs, per-suite filtering, and failure
// clustering, all as a pure function of the on-disk result files.
func TestEvalTrend(t *testing.T) {
	root := t.TempDir()
	slug := "billing"

	// No evals dir yet → empty report, no error.
	empty, err := EvalTrend(root, slug, "")
	if err != nil {
		t.Fatalf("empty trend: %v", err)
	}
	if len(empty.Runs) != 0 || len(empty.Clusters) != 0 {
		t.Fatalf("expected empty trend, got %+v", empty)
	}

	// Two runs of the smoke suite with a rising score and a repeated failure,
	// plus one run of a different suite to exercise the filter.
	save := func(suite string, score float64, passed bool, failures []string) {
		r := &EvalReport{Suite: suite, Score: score, MinScore: 0.8, Passed: passed, Failures: failures}
		if _, err := SaveEvalReport(root, slug, r); err != nil {
			t.Fatalf("save %s: %v", suite, err)
		}
	}
	save("smoke", 0.70, false, []string{"c1", "c2"})
	save("smoke", 0.90, true, []string{"c1"})
	save("regression", 0.95, true, nil)

	// All suites: three runs; smoke's second run shows a positive delta.
	all, err := EvalTrend(root, slug, "")
	if err != nil {
		t.Fatalf("all trend: %v", err)
	}
	if len(all.Runs) != 3 {
		t.Fatalf("want 3 runs, got %d (%+v)", len(all.Runs), all.Runs)
	}
	var sawDelta bool
	for _, run := range all.Runs {
		if run.Suite == "smoke" && run.Delta > 0.19 && run.Delta < 0.21 {
			sawDelta = true
		}
	}
	if !sawDelta {
		t.Fatalf("expected a ~+0.20 smoke delta: %+v", all.Runs)
	}
	// c1 failed twice, c2 once → c1 clusters first.
	if len(all.Clusters) != 2 || all.Clusters[0].CheckID != "c1" || all.Clusters[0].Count != 2 {
		t.Fatalf("failure clustering wrong: %+v", all.Clusters)
	}

	// Suite filter keeps only smoke runs.
	smoke, err := EvalTrend(root, slug, "smoke")
	if err != nil {
		t.Fatalf("smoke trend: %v", err)
	}
	if len(smoke.Runs) != 2 {
		t.Fatalf("want 2 smoke runs, got %d", len(smoke.Runs))
	}
}
