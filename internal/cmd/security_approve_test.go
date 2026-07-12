package cmd

import (
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core/gates/security"
)

func TestSecurityApproveAndRevokeException(t *testing.T) {
	root := t.TempDir()
	flags := map[string]string{"action": "suppress", "reason": "false positive", "ticket": "SEC-1", "owner": "security", "scope": "scanner", "revision": "abc", "environment": "production", "issued-at": "2026-01-01T00:00:00Z", "expires-at": "2027-01-01T00:00:00Z", "control": "manual review", "approver": "lead"}
	if err := Run(root, "approve", []string{"exception", "approve", "fp"}, flags); err != nil {
		t.Fatal(err)
	}
	set, findings := security.LoadExceptions(root, "abc", "production", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if len(findings) != 0 || !set.Allows("fp") {
		t.Fatal("approved exception inactive")
	}
	flags["action"] = "revoke"
	if err := Run(root, "approve", []string{"exception", "revoke", "fp"}, flags); err != nil {
		t.Fatal(err)
	}
	set, _ = security.LoadExceptions(root, "abc", "production", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if set.Allows("fp") {
		t.Fatal("revoked exception remained active")
	}
}

func TestSecurityApproveCannotWaiveEvidence(t *testing.T) {
	flags := map[string]string{"action": "suppress", "reason": "x", "ticket": "x", "owner": "x", "scope": "evidence", "revision": "abc", "environment": "production", "issued-at": "2026-01-01T00:00:00Z", "expires-at": "2027-01-01T00:00:00Z", "control": "x", "approver": "x"}
	if err := Run(t.TempDir(), "approve", []string{"exception", "approve", "evidence-integrity"}, flags); err == nil {
		t.Fatal("evidence waiver accepted")
	}
}
