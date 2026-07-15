package mcp

import (
	"encoding/json"
	"testing"
)

// Untrusted host-capabilities defensiveness (spec A7, Req 1).
//
// parseHostPrefs reads capabilities.specd from a host's initialize params. The
// host is untrusted: a new/unknown/garbage payload must never panic and must
// degrade to a conservative budget rather than a hostile one. These tests pin
// that posture so the existing maxContextTokens<0→0 clamp can never silently
// regress to trusting attacker-controlled fields.

// malformedHostCapsCorpus seeds the fuzzer (and runs as a deterministic table on
// every `go test`, so CI is reproducible without -fuzz). It covers the failure
// classes the spec calls out: negative, oversized, nil, and type-mismatched.
var malformedHostCapsCorpus = []string{
	``,          // empty
	`null`,      // JSON null
	`{}`,        // no capabilities
	`[]`,        // wrong top-level type
	`"garbage"`, // scalar where object expected
	`{"capabilities":null}`,
	`{"capabilities":{"specd":null}}`,
	`{"capabilities":{"specd":{"maxContextTokens":-1}}}`, // negative
	`{"capabilities":{"specd":{"maxContextTokens":-9999999999}}}`,
	`{"capabilities":{"specd":{"maxTools":-5}}}`,                          // negative
	`{"capabilities":{"specd":{"maxContextTokens":9223372036854775807}}}`, // oversized (max int64)
	`{"capabilities":{"specd":{"maxTools":"not-a-number"}}}`,              // type mismatch
	`{"capabilities":{"specd":{"preferredNamespaces":"read"}}}`,           // string where []string expected
	`{"capabilities":{"specd":{"preferredNamespaces":[1,2,3]}}}`,
	`{"capabilities":{"specd":{"maxContextTokens":{"nested":true}}}}`,
	`{"capabilities":{"specd":"not-an-object"}}`,
	`{not valid json`, // truncated / invalid
}

// assertConservativeHostPrefs verifies the invariants that hold for ANY input:
// the clamps must have run, so negatives can never leak downstream.
func assertConservativeHostPrefs(t *testing.T, hp hostPrefs) {
	t.Helper()
	if hp.maxContextTokens < 0 {
		t.Fatalf("maxContextTokens not clamped: %d", hp.maxContextTokens)
	}
	if hp.maxTools < 0 {
		t.Fatalf("maxTools not clamped: %d", hp.maxTools)
	}
}

func TestParseHostPrefsMalformedTable(t *testing.T) {
	for _, in := range malformedHostCapsCorpus {
		in := in
		t.Run("", func(t *testing.T) {
			// A panic here fails the test outright; that is the no-panic assertion.
			hp := parseHostPrefs(json.RawMessage(in))
			assertConservativeHostPrefs(t, hp)
		})
	}
}

func FuzzParseHostPrefs(f *testing.F) {
	for _, in := range malformedHostCapsCorpus {
		f.Add([]byte(in))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		// No panic on arbitrary bytes, and the budget stays conservative.
		hp := parseHostPrefs(json.RawMessage(raw))
		assertConservativeHostPrefs(t, hp)
	})
}
