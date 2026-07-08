package context

import "testing"

// TestCheckBudgetDisabledDoesNoWork pins invariant A4: when the context budget
// is disabled (maxTokens <= 0), CheckBudget short-circuits and does no budget
// work at all — it never computes or enforces the manifest cost. Proven
// behaviourally: a manifest whose cost exceeds any positive budget is rejected
// at maxTokens=1 but accepted (unchecked) at maxTokens=0 and maxTokens<0.
//
// SPEC-01 owns this minimal runnable gate; SPEC-03 ratchets it to a measured
// O(0)/allocation benchmark. scripts/perf-gate.sh runs this test.
func TestCheckBudgetDisabledDoesNoWork(t *testing.T) {
	// A manifest that costs > 1 token, so a positive floor of 1 must reject it.
	m := Manifest{
		Slug:   "demo",
		TaskID: "T1",
		Mode:   "craftsman",
		Items:  []Item{{Kind: "role", Path: "roles/craftsman.md", EstimatedTokens: 9999}},
	}

	if got := ManifestBudget(m); got <= 1 {
		t.Fatalf("test manifest too cheap (%d tokens) to distinguish enforcement", got)
	}

	if err := CheckBudget(m, 1); err == nil {
		t.Fatal("enabled budget (maxTokens=1) accepted an over-budget manifest; enforcement is inert")
	}

	for _, disabled := range []int{0, -1} {
		if err := CheckBudget(m, disabled); err != nil {
			t.Fatalf("disabled budget (maxTokens=%d) did work and rejected: %v", disabled, err)
		}
	}
}
