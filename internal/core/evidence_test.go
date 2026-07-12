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
