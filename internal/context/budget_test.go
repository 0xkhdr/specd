package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func writeUnder(t *testing.T, root, rel string, n int) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(strings.Repeat("x", n)), 0o644); err != nil {
		t.Fatal(err)
	}
}

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

// TestBudgetAccountsForPayload closes the W0 gap W3 (R3.1) named: the estimate
// now covers the bytes the contract actually loads — design.md and the declared
// source files — not just the path string. A 4000-byte design.md (~1000 tokens)
// plus a 2000-byte declared source file (~500 tokens) must dominate the estimate.
func TestBudgetAccountsForPayload(t *testing.T) {
	root := t.TempDir()
	writeUnder(t, root, ".specd/specs/demo/design.md", 4000)
	writeUnder(t, root, "impl.go", 2000)
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"impl.go"}}}

	m, err := BuildManifest(root, "demo", tasks, "T1", 0)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	if m.EstimatedTokens < 1400 {
		t.Fatalf("estimate %d ignores file payload (design.md + impl.go ~1500 tokens)", m.EstimatedTokens)
	}
	var design, impl Item
	for _, it := range m.Items {
		switch {
		case it.Kind == "design":
			design = it
		case it.Path == "impl.go":
			impl = it
		}
	}
	if design.EstimatedTokens < 900 {
		t.Fatalf("design estimate %d underestimates a 4000-byte design.md", design.EstimatedTokens)
	}
	if impl.EstimatedTokens < 450 {
		t.Fatalf("declared-file estimate %d underestimates a 2000-byte source file", impl.EstimatedTokens)
	}
}
