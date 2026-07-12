package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
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

	t.Run("scan_input_digest_and_trust_are_stable", func(t *testing.T) {
		input := NewScanInput("a.md", ScanKindSource, TrustUntrustedData, []byte("hello"))
		if input.ItemRef != "a.md" || input.Digest == "" || input.Trust != TrustUntrustedData {
			t.Fatalf("input = %+v", input)
		}
		if again := NewScanInput("a.md", ScanKindSource, TrustUntrustedData, []byte("hello")); again.Digest != input.Digest {
			t.Fatalf("digest not stable: %q != %q", input.Digest, again.Digest)
		}
	})

	t.Run("exclusions_are_scanner_specific", func(t *testing.T) {
		input := NewScanInput(".specd/specs/demo/tasks.md", ScanKindSpec, TrustUntrustedData, []byte("text"))
		if (injectionScanner{}).Exclude(input) {
			t.Fatal("injection scanner must inspect runtime specs")
		}
		if !(slopsquatScanner{}).Exclude(input) {
			t.Fatal("slopsquat scanner should exclude non-manifest runtime specs")
		}
		control := NewScanInput(".specd/security/allow.json", ScanKindUntracked, TrustUntrustedData, []byte("state"))
		if !(secretsScanner{}).Exclude(control) || !(injectionScanner{}).Exclude(control) {
			t.Fatal("scanners must explicitly exclude harness security control state")
		}
	})
}

func TestInputEnumerationFailureIsFinding(t *testing.T) {
	result := Analyze(t.TempDir(), core.SecurityConfig{Injection: "error"})
	if !hasFinding(result.Findings, "input", "enumeration-failed") {
		t.Fatalf("missing enumeration error: %+v", result.Findings)
	}
}

func TestInputReadFailureIsFinding(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "unreadable.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	inputs, findings := readScanInputs(root, []scanRef{{Path: "unreadable.md", Kind: ScanKindSource}})
	if len(inputs) != 0 || !hasFinding(findings, "input", "read-failed") {
		t.Fatalf("inputs=%+v findings=%+v", inputs, findings)
	}
}
