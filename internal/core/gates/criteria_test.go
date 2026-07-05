package gates

import "testing"

func TestCriterionIDs(t *testing.T) {
	doc := "# Requirements\n\n" +
		"- **R1** When a user submits, the system shall respond.\n" +
		"  - When input is valid, the system shall accept it.\n" +
		"  - When input is invalid, the system shall reject it.\n" +
		"- **R2** When idle, the system shall wait.\n"

	got := CriterionIDs(doc)
	want := []string{"1.1", "1.2", "2.1"}
	if len(got) != len(want) {
		t.Fatalf("got %d ids %v, want %v", len(got), got, want)
	}
	for i, id := range got {
		if id.String() != want[i] {
			t.Fatalf("id[%d] = %q, want %q", i, id.String(), want[i])
		}
	}

	// A bare requirement with no sub-bullets is a single criterion "<r>.1".
	if ids := CriterionIDs("- **R5** the system shall x.\n"); len(ids) != 1 || ids[0].String() != "5.1" {
		t.Fatalf("bare requirement = %v, want [5.1]", ids)
	}

	if HasCriterion(doc, "9.9") {
		t.Fatal("9.9 should not be a valid criterion")
	}
	if !HasCriterion(doc, "1.2") {
		t.Fatal("1.2 should be a valid criterion")
	}
}

func TestCriteriaRequired(t *testing.T) {
	base := CheckCtx{ApproveTarget: "complete", CriteriaRequired: true, CriteriaUnmet: []string{"1.2"}}

	// Armed + unmet ⇒ error naming the ids.
	if f := criteriaGate(base); !HasErrors(f) {
		t.Fatalf("unmet criteria should error, got %+v", f)
	}

	// Armed + all met ⇒ clean.
	if f := criteriaGate(CheckCtx{ApproveTarget: "complete", CriteriaRequired: true}); len(f) != 0 {
		t.Fatalf("all met should pass, got %+v", f)
	}

	// Not the completion transition ⇒ gate is inert even with unmet criteria.
	if f := criteriaGate(CheckCtx{ApproveTarget: "design", CriteriaRequired: true, CriteriaUnmet: []string{"1.2"}}); len(f) != 0 {
		t.Fatalf("non-completion transition should be inert, got %+v", f)
	}

	// Opt-out (default) ⇒ gate never fires.
	if f := criteriaGate(CheckCtx{ApproveTarget: "complete", CriteriaUnmet: []string{"1.2"}}); len(f) != 0 {
		t.Fatalf("disabled gate should be silent, got %+v", f)
	}
}
