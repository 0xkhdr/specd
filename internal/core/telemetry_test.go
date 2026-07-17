package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTelemetryMetadataOnly is the privacy contract for the telemetry schema
// (spec 07 R5.2): a fully populated Annotations serializes to metadata keys
// only. No field can carry a prompt, response, chain-of-thought, file content,
// or raw worker output, so a default fixture is structurally metadata-only.
func TestTelemetryMetadataOnly(t *testing.T) {
	full := Annotations{
		Tokens: 10, InputTokens: 4, OutputTokens: 5, CachedTokens: 1, Cost: "0.01", DurationMs: 5,
		Source: TelemetrySourceWorker, Currency: "USD", PricingRef: "pricing/v1", Provider: "acme", Model: "m1",
		AttestationRef: "att/abc", EnvelopeVersion: TelemetryEnvelopeV1,
	}
	data, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	var keyed map[string]json.RawMessage
	if err := json.Unmarshal(data, &keyed); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"tokens": true, "cost": true, "duration_ms": true,
		"input_tokens": true, "output_tokens": true, "cached_tokens": true,
		"pricing_ref": true, "provider": true, "model": true,
		"telemetry_source": true, "currency": true,
		"attestation_ref": true, "envelope_version": true,
	}
	for k := range keyed {
		if !allowed[k] {
			t.Fatalf("telemetry carries non-metadata field %q — schema must stay metadata-only", k)
		}
	}
}

// TestTelemetryAttestationRefRedacted pins central redaction of the one
// free-form telemetry field before persistence/display (spec 07 R5.2/R5.4): a
// secret or absolute home path smuggled into attestation_ref is scrubbed by the
// same central redactor that guards command/evidence_ref, so it never reaches
// disk.
func TestTelemetryAttestationRefRedacted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.jsonl")
	secret := "leaked-secret-token-value"
	rec := EvidenceRecord{TaskID: "T1", GitHead: "abc", Telemetry: &Annotations{
		EnvelopeVersion: TelemetryEnvelopeV1, Source: TelemetrySourceWorker,
		AttestationRef: "api_key=" + secret + " at /home/alice/.specd/att",
	}}
	if err := AppendEvidence(path, rec); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), secret) {
		t.Fatalf("telemetry leaked secret to evidence: %s", body)
	}
	if strings.Contains(string(body), "/home/alice") {
		t.Fatalf("telemetry leaked absolute home path to evidence: %s", body)
	}
}

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

// TestTelemetryEnvelopeCanonical pins the versioned envelope
// (R1.1). A bare record (tokens/cost/duration only) round-trips byte-for-byte
// with no envelope fields injected, so old fixtures decode unchanged. A
// canonical v1 record carries its version, provenance, and currency and
// round-trips byte-stably. (Run/attempt correlation is Domain 07 W2/W6, not W1.)
func TestTelemetryEnvelopeCanonical(t *testing.T) {
	bare := `{"tokens":10,"cost":"1.50","duration_ms":5}`
	var ann Annotations
	if err := json.Unmarshal([]byte(bare), &ann); err != nil {
		t.Fatal(err)
	}
	blob, err := json.Marshal(&ann)
	if err != nil {
		t.Fatal(err)
	}
	if string(blob) != bare {
		t.Fatalf("bare record not byte-stable: got %s want %s", blob, bare)
	}
	for _, injected := range []string{"telemetry_source", "currency", "envelope_version"} {
		if strings.Contains(string(blob), injected) {
			t.Fatalf("bare record silently gained %q: %s", injected, blob)
		}
	}

	canonical := &Annotations{
		Tokens: 10, Cost: "1.50", Currency: "USD",
		Source: TelemetrySourceWorker, EnvelopeVersion: TelemetryEnvelopeV1,
	}
	first, err := json.Marshal(canonical)
	if err != nil {
		t.Fatal(err)
	}
	var back Annotations
	if err := json.Unmarshal(first, &back); err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(&back)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("canonical record not byte-stable: %s vs %s", first, second)
	}
	js := string(first)
	for _, want := range []string{`"telemetry_source":"worker"`, `"currency":"USD"`, `"envelope_version":"v1"`} {
		if !strings.Contains(js, want) {
			t.Fatalf("canonical envelope missing %q: %s", want, js)
		}
	}
}

func TestValidateAnnotations(t *testing.T) {
	ok := []struct {
		name string
		ann  *Annotations
	}{
		{"nil", nil},
		{"canonical_worker", &Annotations{EnvelopeVersion: "v1", Source: "worker", Cost: "1.50", Currency: "USD", PricingRef: "pricing/v1"}},
		{"canonical_no_cost_no_currency", &Annotations{EnvelopeVersion: "v1", Source: "operator"}},
		{"canonical_adapter_attested", &Annotations{EnvelopeVersion: "v1", Source: "provider_adapter", AttestationRef: "att://x"}},
	}
	for _, tc := range ok {
		if err := ValidateAnnotations(tc.ann); err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
	}

	bad := []struct {
		name string
		ann  *Annotations
	}{
		{"malformed_decimal", &Annotations{Cost: "1,00"}},
		{"negative_tokens", &Annotations{Tokens: -1}},
		{"negative_duration", &Annotations{DurationMs: -5}},
		{"missing_version", &Annotations{Tokens: 100}},
		{"cost_without_currency", &Annotations{Cost: "0.01"}},
		{"unknown_version", &Annotations{EnvelopeVersion: "v2"}},
		{"canonical_cost_without_currency", &Annotations{EnvelopeVersion: "v1", Source: "worker", Cost: "1.50"}},
		{"canonical_missing_source", &Annotations{EnvelopeVersion: "v1", Cost: "1.50", Currency: "USD"}},
		{"canonical_unknown_source", &Annotations{EnvelopeVersion: "v1", Source: "robot"}},
	}
	for _, tc := range bad {
		if err := ValidateAnnotations(tc.ann); err == nil {
			t.Fatalf("%s: expected fail-closed error", tc.name)
		}
	}
}

func TestTelemetrySourceProvenance(t *testing.T) {
	// An attempt with no explicit source is reported as worker-reported; the
	// render marks values as reported, never independently measured (R1.3).
	report := AggregateTelemetry([]EvidenceRecord{
		{TaskID: "T1", Telemetry: &Annotations{Tokens: 5, Cost: "0.01"}},
	}, []string{"T1"})
	if report.Tasks[0].Source != TelemetrySourceWorker {
		t.Fatalf("sourceless attempt provenance = %q, want worker", report.Tasks[0].Source)
	}
	out := RenderTelemetry("demo", report)
	if !strings.Contains(out, "worker-reported, not independently measured") {
		t.Fatalf("render missing provenance disclaimer:\n%s", out)
	}
	// An adapter-sourced attempt surfaces its provenance.
	adapter := AggregateTelemetry([]EvidenceRecord{
		{TaskID: "T2", Telemetry: &Annotations{EnvelopeVersion: "v1", Source: TelemetrySourceAdapter, Cost: "0.02", Currency: "USD", PricingRef: "pricing/v1"}},
	}, []string{"T2"})
	if adapter.Tasks[0].Source != TelemetrySourceAdapter {
		t.Fatalf("adapter provenance = %q, want provider_adapter", adapter.Tasks[0].Source)
	}
}

func TestTelemetryProviderNeutralAnnotations(t *testing.T) {
	ann, err := ParseAnnotationFlags(map[string]string{
		"tokens": "100", "input-tokens": "60", "output-tokens": "30", "cached-tokens": "10",
		"cost": "0.0125", "currency": "USD", "pricing-ref": "pricing/acme-v1",
		"provider": "acme", "model": "reasoner-v2", "telemetry-source": "provider_adapter",
		"attestation-ref": "attestations/run-1.json", "duration-ms": "9",
	}, func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if ann.InputTokens != 60 || ann.OutputTokens != 30 || ann.CachedTokens != 10 || ann.Provider != "acme" || ann.Model != "reasoner-v2" {
		t.Fatalf("provider-neutral fields lost: %+v", ann)
	}
	raw, err := json.Marshal(ann)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"input_tokens":60`, `"output_tokens":30`, `"cached_tokens":10`, `"pricing_ref":"pricing/acme-v1"`, `"provider":"acme"`, `"model":"reasoner-v2"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("annotation JSON missing %s: %s", want, raw)
		}
	}
	bare, err := json.Marshal(Annotations{Tokens: 7})
	if err != nil || string(bare) != `{"tokens":7}` {
		t.Fatalf("bare shape changed: %s %v", bare, err)
	}
}

func TestTelemetryCategoryConsistencyAndPricing(t *testing.T) {
	bad := []*Annotations{
		{EnvelopeVersion: "v1", Source: "worker", Tokens: 10, InputTokens: 4, OutputTokens: 5},
		{EnvelopeVersion: "v1", Source: "worker", Cost: "1.0", Currency: "USD"},
	}
	for _, ann := range bad {
		if err := ValidateAnnotations(ann); err == nil {
			t.Fatalf("invalid annotation accepted: %+v", ann)
		}
	}
	good := &Annotations{EnvelopeVersion: "v1", Source: "worker", Tokens: 10, InputTokens: 4, OutputTokens: 5, CachedTokens: 1, Cost: "1.0", Currency: "USD", PricingRef: "pricing/v1"}
	if err := ValidateAnnotations(good); err != nil {
		t.Fatal(err)
	}
	report := AggregateTelemetry([]EvidenceRecord{
		{TaskID: "T1", Telemetry: &Annotations{Tokens: 10, InputTokens: 4, OutputTokens: 5, CachedTokens: 1, Cost: "0.1"}},
		{TaskID: "T1", Telemetry: &Annotations{Tokens: 20, InputTokens: 8, OutputTokens: 10, CachedTokens: 2, Cost: "0.2"}},
	}, []string{"T1"})
	if report.Cost != "0.3" || report.InputTokens != 12 || report.OutputTokens != 15 || report.CachedTokens != 3 {
		t.Fatalf("category aggregate = %+v", report)
	}
}

func TestTelemetryProviderModelBounds(t *testing.T) {
	for _, ann := range []*Annotations{
		{EnvelopeVersion: "v1", Source: "worker", Provider: strings.Repeat("x", 65)},
		{EnvelopeVersion: "v1", Source: "worker", Model: "bad model"},
	} {
		if err := ValidateAnnotations(ann); err == nil {
			t.Fatalf("unbounded identifier accepted: %+v", ann)
		}
	}
	if err := ValidateAnnotations(&Annotations{EnvelopeVersion: "v1", Source: "worker"}); err != nil {
		t.Fatalf("provider/model became required: %v", err)
	}
	metrics := RenderTelemetry("demo", AggregateTelemetry([]EvidenceRecord{{TaskID: "T1", Telemetry: &Annotations{Provider: "secret-provider", Model: "secret-model"}}}, []string{"T1"}))
	if strings.Contains(metrics, "secret-provider") || strings.Contains(metrics, "secret-model") {
		t.Fatalf("provider/model leaked into metrics: %s", metrics)
	}
}
