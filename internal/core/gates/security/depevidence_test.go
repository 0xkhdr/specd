package security

import (
	"os"
	"path/filepath"
	"testing"
)

func depFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "depevidence", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestDepEvidence(t *testing.T) {
	t.Run("valid_matching_digest_yields_advisories", func(t *testing.T) {
		f := ScanDepEvidence(depFixture(t, "valid.json"), "PINNED")
		if !hasFinding(f, "depevidence", "advisory") {
			t.Fatalf("valid pinned artifact must surface advisories: %+v", f)
		}
		if hasFinding(f, "depevidence", "evidence-malformed") || hasFinding(f, "depevidence", "evidence-stale") {
			t.Fatalf("valid artifact must not fail: %+v", f)
		}
	})

	t.Run("malformed_fails_closed", func(t *testing.T) {
		f := ScanDepEvidence(depFixture(t, "malformed.json"), "PINNED")
		if !hasFinding(f, "depevidence", "evidence-malformed") {
			t.Fatalf("malformed artifact must fail closed: %+v", f)
		}
		if f[0].Severity != "error" {
			t.Fatalf("malformed must be error severity: %+v", f)
		}
	})

	t.Run("wrong_schema_fails", func(t *testing.T) {
		if f := ScanDepEvidence(depFixture(t, "wrong_schema.json"), "PINNED"); !hasFinding(f, "depevidence", "evidence-malformed") {
			t.Fatalf("unknown schema must fail: %+v", f)
		}
	})

	t.Run("empty_artifact_fails", func(t *testing.T) {
		if f := ScanDepEvidence(nil, "PINNED"); !hasFinding(f, "depevidence", "evidence-malformed") {
			t.Fatalf("empty artifact must fail closed: %+v", f)
		}
	})

	t.Run("stale_digest_fails", func(t *testing.T) {
		f := ScanDepEvidence(depFixture(t, "valid.json"), "DIFFERENT")
		if !hasFinding(f, "depevidence", "evidence-stale") {
			t.Fatalf("digest mismatch must be stale: %+v", f)
		}
		if hasFinding(f, "depevidence", "advisory") {
			t.Fatalf("stale artifact must not surface its advisories: %+v", f)
		}
	})

	t.Run("scan_is_pure_deterministic", func(t *testing.T) {
		a := ScanDepEvidence(depFixture(t, "valid.json"), "PINNED")
		b := ScanDepEvidence(depFixture(t, "valid.json"), "PINNED")
		if len(a) != len(b) {
			t.Fatalf("scan not deterministic: %d != %d", len(a), len(b))
		}
	})

	t.Run("manifest_digest_matches_concatenation", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "go.sum"), []byte("h\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := ManifestDigest(root)
		if err != nil {
			t.Fatal(err)
		}
		if want := digest([]byte("module x\nh\n")); got != want {
			t.Fatalf("ManifestDigest = %q want %q", got, want)
		}
	})
}
