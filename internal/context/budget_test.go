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

// TestEnforceBudgetV2 (R3.2/R3.3): a required total over budget fails closed and
// truncates nothing; optional items shed in deterministic priority-desc order,
// each omission naming item and reason; budget<=0 disables enforcement.
func TestEnforceBudgetV2(t *testing.T) {
	req := ItemV2{Kind: "task", Required: true, Priority: 0, EstimatedTokens: 100, Reason: "selected task"}
	optHi := ItemV2{Kind: "memory", Source: "m1", Priority: 5, EstimatedTokens: 30, Reason: "relevant memory"}
	optLo := ItemV2{Kind: "examples", Source: "e1", Priority: 9, EstimatedTokens: 30, Reason: "example"}

	if _, _, _, _, err := EnforceBudgetV2([]ItemV2{req}, 50); err == nil {
		t.Fatal("required overflow must fail closed")
	} else if _, ok := err.(BudgetError); !ok {
		t.Fatalf("want BudgetError, got %v", err)
	}

	kept, oms, reqTok, optTok, err := EnforceBudgetV2([]ItemV2{req, optHi, optLo}, 130)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if reqTok != 100 {
		t.Fatalf("required tokens = %d", reqTok)
	}
	if len(oms) != 1 || oms[0].Source != "e1" || oms[0].Reason == "" {
		t.Fatalf("expected e1 shed first with a reason, got %+v", oms)
	}
	if len(kept) != 2 || optTok != 30 {
		t.Fatalf("kept=%+v optTok=%d", kept, optTok)
	}

	kept2, oms2, _, _, err := EnforceBudgetV2([]ItemV2{req, optHi}, 0)
	if err != nil || len(oms2) != 0 || len(kept2) != 2 {
		t.Fatalf("budget<=0 must keep all: kept=%+v oms=%+v err=%v", kept2, oms2, err)
	}
}

// TestBudgetUnderestimatesPayload characterizes the W0 gap W3 (R3.1) closes:
// ManifestBudget counts only each item's declared EstimatedTokens plus its
// short kind/path strings — it never reads the referenced file. So an item with
// EstimatedTokens=0 pointing at a large design.md/source file contributes only
// its path length, grossly underestimating the real context payload. When W3
// lands, the estimate accounts for the bytes the contract actually loads.
func TestBudgetUnderestimatesPayload(t *testing.T) {
	m := Manifest{
		Slug:   "demo",
		TaskID: "T1",
		Mode:   "craftsman",
		Items:  []Item{{Kind: "design", Path: "specs/demo/design.md", EstimatedTokens: 0}},
	}
	if got := ManifestBudget(m); got > 100 {
		t.Fatalf("W0 gap closed early: budget %d already accounts for file payload", got)
	}
}
