package orchestration

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestDispatchEnvelopePinsAndCanonicalizes(t *testing.T) {
	m := validMission()
	e, err := NewDispatchEnvelope("/repo", m)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateDispatchEnvelope(e); err != nil {
		t.Fatal(err)
	}
	if e.EnvelopeDigest == "" || e.DeclaredFiles[0] != "a.go" || e.Acceptance[0] != "R1" {
		t.Fatalf("envelope = %+v", e)
	}
	changed := e
	changed.Role = "validator"
	if err := ValidateDispatchEnvelope(changed); err == nil || !strings.Contains(err.Error(), "DIGEST") {
		t.Fatalf("changed envelope accepted: %v", err)
	}
	if DispatchEnvelopeDigest(e) != e.EnvelopeDigest {
		t.Fatal("digest not stable")
	}
	if core.DispatchDigest(e) != e.EnvelopeDigest {
		t.Fatal("core/orchestration digest mismatch")
	}
}

func TestDispatchEnvelopeRejectsUnsafeScope(t *testing.T) {
	m := validMission()
	m.DeclaredFiles = []string{"../outside.go"}
	if _, err := NewDispatchEnvelope("/repo", m); err == nil {
		t.Fatal("unsafe declared path accepted")
	}
}
