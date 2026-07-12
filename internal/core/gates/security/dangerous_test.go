package security

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core/scope"
)

// dangerRepo writes files into a temp root and returns a scope.Diff naming them
// as added changes, mirroring what scope.Derive produces from a real diff.
func dangerRepo(t *testing.T, files map[string]string) (string, scope.Diff) {
	t.Helper()
	root := t.TempDir()
	var changes []scope.Change
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		changes = append(changes, scope.Change{Path: rel, Kind: "A"})
	}
	return root, scope.Diff{Baseline: "HEAD", Changes: changes}
}

func TestDangerous(t *testing.T) {
	t.Run("destructive_shell_detected", func(t *testing.T) {
		root, diff := dangerRepo(t, map[string]string{"deploy.sh": "#!/bin/sh\nrm -rf /\n"})
		if f := ScanDangerous(root, diff, "prototype"); !hasFinding(f, "dangerous", "destructive-shell") {
			t.Fatalf("destructive command must be flagged: %+v", f)
		}
	})

	t.Run("world_writable_mode_detected", func(t *testing.T) {
		root, _ := dangerRepo(t, map[string]string{"x": "data"})
		diff := scope.Diff{Changes: []scope.Change{{Path: "x", Kind: "M", OldMode: "100644", NewMode: "100777"}}}
		if f := ScanDangerous(root, diff, "production"); !hasFinding(f, "dangerous", "world-writable") {
			t.Fatalf("world-writable mode must be flagged: %+v", f)
		}
	})

	t.Run("unexpected_exec_mode_detected", func(t *testing.T) {
		root, _ := dangerRepo(t, map[string]string{"notes.txt": "hi"})
		diff := scope.Diff{Changes: []scope.Change{{Path: "notes.txt", Kind: "M", OldMode: "100644", NewMode: "100755"}}}
		if f := ScanDangerous(root, diff, "production"); !hasFinding(f, "dangerous", "exec-mode") {
			t.Fatalf("new exec bit on a non-script must be flagged: %+v", f)
		}
	})

	t.Run("authz_change_detected", func(t *testing.T) {
		root, diff := dangerRepo(t, map[string]string{".github/CODEOWNERS": "* @attacker\n"})
		if f := ScanDangerous(root, diff, "prototype"); !hasFinding(f, "dangerous", "authz-change") {
			t.Fatalf("authz/ownership change must be flagged: %+v", f)
		}
	})

	t.Run("generated_secret_file_detected", func(t *testing.T) {
		root, diff := dangerRepo(t, map[string]string{"deploy/id_rsa": "-----BEGIN PRIVATE KEY-----\n"})
		f := ScanDangerous(root, diff, "prototype")
		if !hasFinding(f, "dangerous", "generated-secret") {
			t.Fatalf("generated secret material must be flagged: %+v", f)
		}
	})

	t.Run("symlink_escape_detected", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		link := filepath.Join(root, "escape")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("symlinks unsupported: %v", err)
		}
		diff := scope.Diff{Changes: []scope.Change{{Path: "escape", Kind: "A"}}}
		if f := ScanDangerous(root, diff, "prototype"); !hasFinding(f, "dangerous", "symlink-escape") {
			t.Fatalf("symlink escaping the repo must be flagged: %+v", f)
		}
	})

	t.Run("benign_change_is_clean", func(t *testing.T) {
		root, diff := dangerRepo(t, map[string]string{"internal/core/foo.go": "package core\n\nfunc Foo() {}\n"})
		if f := ScanDangerous(root, diff, "production"); len(f) != 0 {
			t.Fatalf("benign source change must not trip a control (false positive): %+v", f)
		}
	})

	t.Run("deletions_are_not_scanned", func(t *testing.T) {
		root := t.TempDir()
		diff := scope.Diff{Changes: []scope.Change{{Path: "id_rsa", Kind: "D"}}}
		if f := ScanDangerous(root, diff, "production"); len(f) != 0 {
			t.Fatalf("deletions carry no danger content: %+v", f)
		}
	})

	t.Run("severity_follows_profile", func(t *testing.T) {
		root, diff := dangerRepo(t, map[string]string{"deploy.sh": "rm -rf /\n"})
		prod := ScanDangerous(root, diff, "production")
		proto := ScanDangerous(root, diff, "prototype")
		if !hasSeverity(prod, "error") {
			t.Fatalf("production must raise to error: %+v", prod)
		}
		if !hasSeverity(proto, "warn") {
			t.Fatalf("prototype must warn: %+v", proto)
		}
	})
}

func hasSeverity(findings []Finding, sev string) bool {
	for _, f := range findings {
		if f.Severity == sev {
			return true
		}
	}
	return false
}
