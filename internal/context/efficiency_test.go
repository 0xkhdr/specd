package context

import (
	"strings"
	"testing"
)

func intp(v int) *int       { return &v }
func strp(v string) *string { return &v }

func TestEfficiencyExplicitUnknownAndStable(t *testing.T) {
	r := EfficiencyReport{SchemaVersion: EfficiencySchemaV1, SpecID: "demo", Tasks: []TaskEfficiency{{TaskID: "T1", EstimatedInputTokens: intp(120), OmittedItems: []Omission{{Kind: "memory", Source: "notes.md", Reason: "budget"}}, RetryCount: 2, FirstPassResult: "fail", DurationMS: intp(35), Cost: strp("0.25")}, {TaskID: "T2", FirstPassResult: "unknown"}}}
	a, err := RenderEfficiency(r)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RenderEfficiency(r)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("render not deterministic")
	}
	for _, want := range []string{"estimated_tokens=120", "actual_tokens=unknown", "omitted=memory:notes.md:budget", "retries=2", "first_pass=fail", "duration_ms=35", "cost=0.25", "task=T2 estimated_tokens=unknown actual_tokens=unknown", "duration_ms=unknown cost=unknown"} {
		if !strings.Contains(a, want) {
			t.Fatalf("missing %q in %s", want, a)
		}
	}
}

func TestEfficiencyRejectsZeroAsUnknownSentinel(t *testing.T) {
	r := EfficiencyReport{SchemaVersion: EfficiencySchemaV1, SpecID: "s", Tasks: []TaskEfficiency{{TaskID: "T", FirstPassResult: ""}}}
	if _, err := RenderEfficiency(r); err == nil {
		t.Fatal("missing first-pass state accepted")
	}
}
