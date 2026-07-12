package security

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExceptionRequiredFieldsAndLifecycle(t *testing.T) {
	valid := Exception{Finding: "fp", Action: "suppress", Reason: "false positive", Ticket: "SEC-1", Owner: "security", Scope: "scanner", Revision: "abc", Environment: "production", IssuedAt: "2026-01-01T00:00:00Z", ExpiresAt: "2027-01-01T00:00:00Z", CompensatingControl: "manual review", Approver: "lead"}
	if err := ValidateException(valid); err != nil {
		t.Fatal(err)
	}
	valid.Ticket = ""
	if err := ValidateException(valid); err == nil {
		t.Fatal("missing ticket accepted")
	}
}

func TestExceptionLoadSuppressesOnlyActiveExactRevision(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".specd", "security")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"finding":"fp","action":"suppress","reason":"false positive","ticket":"SEC-1","owner":"security","scope":"scanner","revision":"abc","environment":"production","issued_at":"2026-01-01T00:00:00Z","expires_at":"2027-01-01T00:00:00Z","compensating_control":"manual review","approver":"lead"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "exceptions.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	set, findings := LoadExceptions(root, "abc", "production", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if len(findings) != 0 || !set.Allows("fp") {
		t.Fatalf("active exception not applied: %#v", findings)
	}
	set, _ = LoadExceptions(root, "def", "production", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if set.Allows("fp") {
		t.Fatal("wrong-revision exception suppressed finding")
	}
}

func TestExceptionDigestChangesOnEdit(t *testing.T) {
	a := []byte("one\n")
	b := []byte("two\n")
	if ExceptionDigest(a) == ExceptionDigest(b) {
		t.Fatal("edited ledger retained digest")
	}
}
