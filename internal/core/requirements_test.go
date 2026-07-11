package core

import (
	"reflect"
	"strings"
	"testing"
)

func TestRequirementsValidate(t *testing.T) {
	raw := []byte(`### R1 — Group one

- R1.1: When a user acts, system shall respond.
- R1.1: When a user acts again, system shall respond twice.
- R2.5: When misfiled, system shall be flagged.
- R1.2: this clause has no EARS shape at all.

### R1 — Duplicate group
`)
	doc, err := ParseRequirements(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	findings := ValidateRequirements(doc)
	want := map[string]string{
		"R1":   "duplicate requirement id",
		"R1.1": "duplicate criterion id",
		"R2.5": "does not belong",
		"R1.2": "EARS",
	}
	for id, sub := range want {
		if !hasFinding(findings, id, sub) {
			t.Fatalf("missing finding %s ~ %q in %+v", id, sub, findings)
		}
	}
}

func TestRequirementsValidateEmpty(t *testing.T) {
	doc, _ := ParseRequirements([]byte("# Requirements\n\nno structured requirements here\n"))
	if len(ValidateRequirements(doc)) == 0 {
		t.Fatal("empty requirements should produce a finding")
	}
}

func hasFinding(findings []ReqFinding, id, sub string) bool {
	for _, f := range findings {
		if f.ID == id && strings.Contains(f.Message, sub) {
			return true
		}
	}
	return false
}

func TestRequirementsParse(t *testing.T) {
	raw := []byte(`# Requirements — demo

### R1 — Structured requirements

- R1.1: When author submits requirements, system shall parse IDs.
- R1.2: When ID is missing, the system shall fail approval.
- owner: platform-team
- priority: high
- risk: R2
- edge: empty document rejected

## Non-goals

- No LLM semantic gate.
`)
	doc, err := ParseRequirements(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Requirements) != 1 {
		t.Fatalf("requirements = %d, want 1", len(doc.Requirements))
	}
	r := doc.Requirements[0]
	if r.ID != "R1" || r.Title != "Structured requirements" {
		t.Fatalf("group = %+v", r)
	}
	if r.Owner != "platform-team" || r.Priority != "high" || r.Risk != "R2" {
		t.Fatalf("meta = %+v", r)
	}
	if len(r.Edges) != 1 || r.Edges[0] != "empty document rejected" {
		t.Fatalf("edges = %+v", r.Edges)
	}
	if len(r.Criteria) != 2 {
		t.Fatalf("criteria = %d, want 2", len(r.Criteria))
	}
	c := r.Criteria[0]
	if c.ID != "R1.1" {
		t.Fatalf("criterion id = %q", c.ID)
	}
	if c.Trigger != "author submits requirements" {
		t.Fatalf("trigger = %q", c.Trigger)
	}
	if c.Response != "parse IDs." {
		t.Fatalf("response = %q", c.Response)
	}
	// second criterion uses "the system shall" and must strip the same way.
	if doc.Requirements[0].Criteria[1].Trigger != "ID is missing" {
		t.Fatalf("trigger2 = %q", doc.Requirements[0].Criteria[1].Trigger)
	}
	if len(doc.Exclusions) != 1 || doc.Exclusions[0] != "No LLM semantic gate." {
		t.Fatalf("exclusions = %+v", doc.Exclusions)
	}
}

func TestRequirementsByteStable(t *testing.T) {
	raw := []byte("### R1 — X\n\n- R1.1: When a happens, system shall b.\n")
	src := append([]byte(nil), raw...)
	doc1, err := ParseRequirements(raw)
	if err != nil {
		t.Fatalf("parse1: %v", err)
	}
	doc2, err := ParseRequirements(append([]byte(nil), src...))
	if err != nil {
		t.Fatalf("parse2: %v", err)
	}
	// Idempotent: identical bytes → identical normalized records.
	if !reflect.DeepEqual(doc1.Requirements, doc2.Requirements) {
		t.Fatalf("not idempotent:\n%#v\n%#v", doc1.Requirements, doc2.Requirements)
	}
	// Author bytes preserved verbatim in Raw.
	if string(doc1.Raw) != string(src) {
		t.Fatalf("raw bytes changed: %q", doc1.Raw)
	}
	// Defensive copy: mutating Raw must not corrupt the caller's slice.
	if len(doc1.Raw) > 0 {
		doc1.Raw[0] = 'Z'
		if raw[0] == 'Z' {
			t.Fatal("ParseRequirements aliased the input slice")
		}
	}
}
