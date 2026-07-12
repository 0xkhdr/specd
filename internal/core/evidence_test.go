package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTruncateEvidenceOutputReportsLimit(t *testing.T) {
	output := strings.Repeat("x", EvidenceOutputLimit+200)
	truncated := TruncateEvidenceOutput(output)
	if !strings.Contains(truncated, "output truncated to 65536 of 65736 bytes") {
		t.Fatalf("missing truncation marker: %q", truncated[len(truncated)-80:])
	}
	if len(truncated) >= len(output) {
		t.Fatalf("expected shortened output, got %d >= %d", len(truncated), len(output))
	}
}

func TestEvidenceQualityLegacyBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.jsonl")
	if err := os.WriteFile(path, []byte(`{"task_id":"T1","command":"go test ./...","exit_code":0,"git_head":"abc"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	records, err := LoadEvidence(path)
	if err != nil {
		t.Fatal(err)
	}
	if !HasPassingEvidence(records, "T1") {
		t.Fatal("legacy verify lost test compatibility")
	}
	if _, ok := records["T2"]; ok {
		t.Fatal("wrong-task evidence selected")
	}
}

func TestEvidenceMalformedBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.jsonl")
	os.WriteFile(path, []byte("{truncated\n"), 0o644)
	if _, err := LoadEvidence(path); err == nil {
		t.Fatal("malformed evidence accepted")
	}
}

// TestEvidenceTelemetryEnvelope pins the W1 versioned envelope on evidence
// records (spec 07 R1.1/R1.2): a legacy telemetry line decodes unchanged, a
// canonical v1 record round-trips through disk, and a malformed canonical
// telemetry line fails closed on decode.
func TestEvidenceTelemetryEnvelope(t *testing.T) {
	dir := t.TempDir()

	// Legacy telemetry (bare cost, no currency/version) still decodes.
	legacy := filepath.Join(dir, "legacy.jsonl")
	os.WriteFile(legacy, []byte(`{"task_id":"T1","exit_code":0,"git_head":"abc","telemetry":{"cost":"0.01"}}`+"\n"), 0o644)
	if _, err := LoadEvidenceRecords(legacy); err != nil {
		t.Fatalf("legacy telemetry rejected: %v", err)
	}

	// Canonical v1 telemetry round-trips through append + load.
	canon := filepath.Join(dir, "canon.jsonl")
	rec := EvidenceRecord{TaskID: "T1", ExitCode: 0, GitHead: "abc",
		Telemetry: &Annotations{EnvelopeVersion: "v1", Source: "worker", Cost: "0.02", Currency: "USD"}}
	if err := AppendEvidence(canon, rec); err != nil {
		t.Fatalf("append canonical telemetry: %v", err)
	}
	got, err := LoadEvidenceRecords(canon)
	if err != nil {
		t.Fatalf("load canonical telemetry: %v", err)
	}
	if got[0].Telemetry.Currency != "USD" || got[0].Telemetry.Source != "worker" {
		t.Fatalf("canonical envelope not round-tripped: %+v", got[0].Telemetry)
	}

	// Malformed canonical telemetry (cost without currency) fails closed.
	bad := filepath.Join(dir, "bad.jsonl")
	os.WriteFile(bad, []byte(`{"task_id":"T1","exit_code":0,"git_head":"abc","telemetry":{"envelope_version":"v1","telemetry_source":"worker","cost":"0.02"}}`+"\n"), 0o644)
	if _, err := LoadEvidenceRecords(bad); err == nil {
		t.Fatal("malformed canonical telemetry accepted on decode")
	}
	if err := AppendEvidence(bad, EvidenceRecord{TaskID: "T1", GitHead: "abc",
		Telemetry: &Annotations{EnvelopeVersion: "v2"}}); err == nil {
		t.Fatal("unknown envelope version accepted on append")
	}
}

func TestEvidenceRedactsCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.jsonl")
	secret := "abcdefghijklmnop"
	if err := AppendEvidence(path, EvidenceRecord{TaskID: "T1", Command: "echo Authorization: Bearer " + secret, GitHead: "abc"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), secret) {
		t.Fatalf("evidence leaked credential: %s", body)
	}
}
