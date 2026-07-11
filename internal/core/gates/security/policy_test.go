package security

import (
	"github.com/0xkhdr/specd/internal/core"
	"testing"
)

func TestPolicyCanonicalDigest(t *testing.T) {
	a, err := ResolvePolicy(core.SecurityConfig{Profile: "production", Secrets: "warn", Injection: "off", Slopsquat: "warn"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := ResolvePolicy(core.SecurityConfig{Profile: "production", Secrets: "warn", Injection: "off", Slopsquat: "warn"})
	if err != nil {
		t.Fatal(err)
	}
	if a.PolicyDigest == "" || a.PolicyDigest != b.PolicyDigest {
		t.Fatalf("digests %q %q", a.PolicyDigest, b.PolicyDigest)
	}
	if a.ScannerSeverities["injection"] != "error" || !a.SandboxRequired {
		t.Fatalf("production policy=%+v", a)
	}
}

func TestPolicyRejectsMissingOrUnknownProfile(t *testing.T) {
	for _, p := range []string{"", "unsafe"} {
		if _, err := ResolvePolicy(core.SecurityConfig{Profile: p}); err == nil {
			t.Fatalf("profile %q accepted", p)
		}
	}
}
