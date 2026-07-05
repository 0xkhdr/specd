package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFixture(t *testing.T, rel string) TrackedFile {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", rel))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	return TrackedFile{Path: rel, Content: content}
}

func TestScannerFramework(t *testing.T) {
	t.Run("fingerprint_is_deterministic_and_path_scoped", func(t *testing.T) {
		a := fingerprint("rule", "a.txt", "secret")
		b := fingerprint("rule", "a.txt", "secret")
		c := fingerprint("rule", "b.txt", "secret")
		if a != b {
			t.Fatal("fingerprint not deterministic")
		}
		if a == c {
			t.Fatal("fingerprint must include path")
		}
	})

	t.Run("fingerprint_changes_when_content_edited", func(t *testing.T) {
		if fingerprint("r", "p", "AKIAABCDEFGHIJKLMNOP") == fingerprint("r", "p", "AKIAABCDEFGHIJKLMNOQ") {
			t.Fatal("editing the match must change the fingerprint")
		}
	})

	t.Run("redact_masks_middle", func(t *testing.T) {
		got := redact("AKIAABCDEFGHIJKLMNOP")
		if strings.Contains(got, "ABCDEFGHIJKL") {
			t.Fatalf("redact leaked the secret body: %q", got)
		}
		if !strings.HasPrefix(got, "AKIA") || !strings.HasSuffix(got, "MNOP") {
			t.Fatalf("redact should keep first/last 4: %q", got)
		}
	})

	t.Run("short_candidate_fully_masked", func(t *testing.T) {
		if redact("short") != "****" {
			t.Fatalf("short candidate not fully masked: %q", redact("short"))
		}
	})

	t.Run("scan_boundary_excludes_fixtures_and_checksums", func(t *testing.T) {
		for _, rel := range []string{"go.sum", "testdata/secrets/leak.txt", "vendor/x/y.go", ".specd/security/allow.json", "reference/foo.go"} {
			if !excludedFromScan(rel) {
				t.Errorf("expected %s excluded from scan", rel)
			}
		}
		for _, rel := range []string{"main.go", "internal/core/state.go", "docs/CHEATSHEET.md"} {
			if excludedFromScan(rel) {
				t.Errorf("expected %s scanned", rel)
			}
		}
	})
}
