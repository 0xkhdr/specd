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

func TestEvidenceQualityMinimalBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.jsonl")
	if err := os.WriteFile(path, []byte(`{"task_id":"T1","command":"go test ./...","exit_code":0,"git_head":"abc"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	records, err := LoadEvidence(path)
	if err != nil {
		t.Fatal(err)
	}
	if !HasPassingEvidence(records, "T1") {
		t.Fatal("minimal verify record lost test compatibility")
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

// TestEvidenceTelemetryEnvelope pins the versioned envelope on evidence
// records (spec 07 R1.1/R1.2): a telemetry line without the v1 envelope fails
// closed, a canonical v1 record round-trips through disk, and a malformed
// canonical telemetry line fails closed on decode.
func TestEvidenceTelemetryEnvelope(t *testing.T) {
	dir := t.TempDir()

	// Telemetry without the v1 envelope (bare cost, no version) fails closed.
	bare := filepath.Join(dir, "bare.jsonl")
	os.WriteFile(bare, []byte(`{"task_id":"T1","exit_code":0,"git_head":"abc","telemetry":{"cost":"0.01"}}`+"\n"), 0o644)
	if _, err := LoadEvidenceRecords(bare); err == nil {
		t.Fatal("telemetry without v1 envelope accepted")
	}

	// Canonical v1 telemetry round-trips through append + load.
	canon := filepath.Join(dir, "canon.jsonl")
	rec := EvidenceRecord{TaskID: "T1", ExitCode: 0, GitHead: "abc",
		Telemetry: &Annotations{EnvelopeVersion: "v1", Source: "worker", Cost: "0.02", Currency: "USD", PricingRef: "pricing/v1"}}
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

// TestEvidenceRefRejectsUnsafe pins the evidence_ref locator contract (spec 07
// R5.3): a ref must be workspace-relative or content-addressed. Absolute paths,
// URLs, and parent traversal fail closed on both append and decode; a relative
// or content-addressed ref is accepted.
func TestEvidenceRefRejectsUnsafe(t *testing.T) {
	dir := t.TempDir()
	for _, bad := range []string{
		"/etc/passwd",
		"~/secrets",
		"https://evil.example/x",
		"file:///etc/shadow",
		"../../outside",
		"artifacts/../../escape",
	} {
		p := filepath.Join(dir, "bad.jsonl")
		if err := AppendEvidence(p, EvidenceRecord{TaskID: "T1", GitHead: "abc", EvidenceRef: bad}); err == nil {
			t.Fatalf("unsafe evidence_ref accepted on append: %q", bad)
		}
	}

	// Good refs: workspace-relative path and content-addressed digest.
	for _, good := range []string{"artifacts/out.log", "sha256:deadbeef"} {
		p := filepath.Join(dir, "good.jsonl")
		if err := AppendEvidence(p, EvidenceRecord{TaskID: "T1", GitHead: "abc", EvidenceRef: good}); err != nil {
			t.Fatalf("safe evidence_ref rejected: %q: %v", good, err)
		}
	}

	// A ledger line carrying an unsafe ref fails closed on decode.
	tampered := filepath.Join(dir, "tampered.jsonl")
	os.WriteFile(tampered, []byte(`{"task_id":"T1","exit_code":0,"git_head":"abc","evidence_ref":"/etc/passwd"}`+"\n"), 0o644)
	if _, err := LoadEvidenceRecords(tampered); err == nil {
		t.Fatal("unsafe evidence_ref accepted on decode")
	}
	if _, err := LoadEvidence(tampered); err == nil {
		t.Fatal("unsafe evidence_ref accepted on LoadEvidence")
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

func TestEvidencePinsContextReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.jsonl")
	digest := strings.Repeat("a", 64)
	if err := AppendEvidence(path, EvidenceRecord{TaskID: "T1", GitHead: "abc", ContextReceiptDigest: digest}); err != nil {
		t.Fatal(err)
	}
	records, err := LoadEvidenceRecords(path)
	if err != nil || len(records) != 1 || records[0].ContextReceiptDigest != digest {
		t.Fatalf("records=%+v err=%v", records, err)
	}
	if err := AppendEvidence(path, EvidenceRecord{TaskID: "T2", GitHead: "abc", ContextReceiptDigest: "not-a-digest"}); err == nil {
		t.Fatal("invalid context receipt digest accepted")
	}

	minimal := filepath.Join(t.TempDir(), "minimal.jsonl")
	if err := os.WriteFile(minimal, []byte(`{"task_id":"T0","git_head":"abc"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadEvidenceRecords(minimal); err != nil || len(got) != 1 || got[0].ContextReceiptDigest != "" {
		t.Fatalf("minimal evidence unreadable: %+v %v", got, err)
	}
}
