package core

import (
	"testing"
)

func present(names ...string) func(string) bool {
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	return func(name string) bool { return set[name] }
}

func TestTelemetryFields(t *testing.T) {
	// No flags ⇒ nil telemetry, no error (always optional, R5).
	ann, err := ParseAnnotations("", "", "", present())
	if err != nil || ann != nil {
		t.Fatalf("no flags should yield (nil,nil), got (%+v,%v)", ann, err)
	}

	// Stored verbatim (R1): the decimal string is not normalized.
	ann, err = ParseAnnotations("1200", "0.034", "45000", present("tokens", "cost", "duration-ms"))
	if err != nil {
		t.Fatalf("valid annotations: %v", err)
	}
	if ann.Tokens != 1200 || ann.Cost != "0.034" || ann.DurationMs != 45000 {
		t.Fatalf("annotations not stored verbatim: %+v", ann)
	}

	// Malformed values fail closed (R2) — the caller maps this to exit 2.
	bad := []struct {
		name              string
		tokens, cost, dur string
		flags             []string
	}{
		{"non_integer_tokens", "1.5", "", "", []string{"tokens"}},
		{"negative_tokens", "-1", "", "", []string{"tokens"}},
		{"non_decimal_cost", "", "1,00", "", []string{"cost"}},
		{"negative_cost", "", "-0.5", "", []string{"cost"}},
		{"fraction_cost_rejected", "", "1/3", "", []string{"cost"}},
		{"non_integer_duration", "", "", "9.9", []string{"duration-ms"}},
	}
	for _, tc := range bad {
		if _, err := ParseAnnotations(tc.tokens, tc.cost, tc.dur, present(tc.flags...)); err == nil {
			t.Fatalf("%s: expected validation error", tc.name)
		}
	}
}

func TestTelemetryAggregateExactDecimal(t *testing.T) {
	// Float-poison: 0.1 + 0.2 accumulated must be exactly 0.3, not 0.30000000004.
	records := []EvidenceRecord{
		{TaskID: "T1", Telemetry: &Annotations{Tokens: 100, Cost: "0.1", DurationMs: 10}},
		{TaskID: "T1", Telemetry: &Annotations{Tokens: 200, Cost: "0.2", DurationMs: 20}},
		{TaskID: "T2"}, // no telemetry ⇒ shown as missing, never imputed
	}
	report := AggregateTelemetry(records, []string{"T1", "T2"})

	if report.Cost != "0.3" {
		t.Fatalf("cost aggregation not exact: %q", report.Cost)
	}
	if report.Tokens != 300 || report.DurationMs != 30 {
		t.Fatalf("integer sums wrong: tokens=%d dur=%d", report.Tokens, report.DurationMs)
	}
	if len(report.Missing) != 1 || report.Missing[0] != "T2" {
		t.Fatalf("missing not surfaced: %v", report.Missing)
	}
	// T1 has two attempts recorded.
	for _, task := range report.Tasks {
		if task.TaskID == "T1" {
			if !task.HasTelemetry || len(task.Attempts) != 2 {
				t.Fatalf("T1 per-attempt breakdown wrong: %+v", task)
			}
		}
		if task.TaskID == "T2" && task.HasTelemetry {
			t.Fatalf("T2 must not report telemetry")
		}
	}
}
