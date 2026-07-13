package security

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRegressCorpusValidAndDeterministic(t *testing.T) {
	corpus, err := LoadIncidentCorpus(filepath.Join("testdata", "incidents.json"), "policy-current")
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Incidents) != 2 {
		t.Fatalf("incident count = %d", len(corpus.Incidents))
	}
	a := corpus.Trend()
	b := corpus.Trend()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("trend not deterministic: %#v != %#v", a, b)
	}
	if len(a) != 2 || a[0].Rule > a[1].Rule {
		t.Fatalf("trend not sorted: %#v", a)
	}
}

func TestRegressPolicyChangeInvalidatesAttestation(t *testing.T) {
	_, err := LoadIncidentCorpus(filepath.Join("testdata", "incidents.json"), "policy-changed")
	if err == nil || !strings.Contains(err.Error(), "stale policy attestation") {
		t.Fatalf("error = %v", err)
	}
}

func TestRegressRejectsSensitiveOrMismatchedFixture(t *testing.T) {
	for name, data := range map[string]string{
		"secret":   `{"schema_version":"security-regression/v1","policy_digest":"p","incidents":[{"id":"i","provenance":"token=ghp_12345678901234567890","input":{"path":"x","kind":"source","trust":"untrusted_data","content":"rm -rf /"},"expected":{"scanner":"dangerous","rule":"destructive-shell"}}]}`,
		"mismatch": `{"schema_version":"security-regression/v1","policy_digest":"p","incidents":[{"id":"i","provenance":"redacted:ticket-1","input":{"path":"x","kind":"source","trust":"untrusted_data","content":"safe"},"expected":{"scanner":"dangerous","rule":"destructive-shell"}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "incident.json")
			if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadIncidentCorpus(p, "p"); err == nil {
				t.Fatal("invalid corpus accepted")
			}
		})
	}
}
