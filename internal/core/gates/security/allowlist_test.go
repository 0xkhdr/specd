package security

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAllow(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, ".specd", "security")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "allow.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAllowlist(t *testing.T) {
	t.Run("missing_file_is_empty_not_error", func(t *testing.T) {
		a, findings := loadAllowlist(t.TempDir())
		if len(findings) != 0 {
			t.Fatalf("missing file produced findings: %+v", findings)
		}
		if a.allows("anything") {
			t.Fatal("empty allowlist should suppress nothing")
		}
	})

	t.Run("entry_missing_reason_fails_closed", func(t *testing.T) {
		root := t.TempDir()
		writeAllow(t, root, `[{"fingerprint":"abc123"}]`)
		a, findings := loadAllowlist(root)
		if len(findings) == 0 || findings[0].Rule != "reason" {
			t.Fatalf("expected reason error, got %+v", findings)
		}
		if a.allows("abc123") {
			t.Fatal("fail-closed load must suppress nothing")
		}
	})

	t.Run("valid_entry_suppresses_by_fingerprint", func(t *testing.T) {
		root := t.TempDir()
		writeAllow(t, root, `[{"fingerprint":"abc123","reason":"synthetic fixture"}]`)
		a, findings := loadAllowlist(root)
		if len(findings) != 0 {
			t.Fatalf("valid allowlist produced findings: %+v", findings)
		}
		if !a.allows("abc123") {
			t.Fatal("expected fingerprint suppressed")
		}
		if a.allows("other") {
			t.Fatal("only the exact fingerprint should be suppressed")
		}
	})

	t.Run("malformed_json_fails_closed", func(t *testing.T) {
		root := t.TempDir()
		writeAllow(t, root, `not json`)
		a, findings := loadAllowlist(root)
		if len(findings) == 0 {
			t.Fatal("expected parse error")
		}
		if a.allows("x") {
			t.Fatal("fail-closed load must suppress nothing")
		}
	})
}
