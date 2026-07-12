package security

import "testing"

func mod(content string) TrackedFile { return TrackedFile{Path: "go.mod", Content: []byte(content)} }
func sum(content string) TrackedFile { return TrackedFile{Path: "go.sum", Content: []byte(content)} }

func TestManifest(t *testing.T) {
	t.Run("new_dep_requires_reason_and_source", func(t *testing.T) {
		f := ScanManifest([]TrackedFile{mod("module x\n\nrequire github.com/pkg/errors v1.0.0\n")}, DependencyPolicy{Profile: "prototype"})
		if !hasFinding(f, "manifest", "require-reason") {
			t.Fatalf("undeclared dependency must require a reason/source: %+v", f)
		}
	})

	t.Run("declared_dep_with_reason_and_source_passes", func(t *testing.T) {
		policy := DependencyPolicy{Profile: "prototype", Declared: map[string]DeclaredDep{
			"github.com/pkg/errors": {Reason: "error wrapping", Source: "vendored, reviewed"},
		}}
		f := ScanManifest([]TrackedFile{mod("module x\n\nrequire github.com/pkg/errors v1.0.0\n")}, policy)
		if len(f) != 0 {
			t.Fatalf("declared dependency should pass: %+v", f)
		}
	})

	t.Run("declared_without_source_still_fails", func(t *testing.T) {
		policy := DependencyPolicy{Profile: "prototype", Declared: map[string]DeclaredDep{
			"github.com/pkg/errors": {Reason: "error wrapping"},
		}}
		if f := ScanManifest([]TrackedFile{mod("module x\n\nrequire github.com/pkg/errors v1.0.0\n")}, policy); !hasFinding(f, "manifest", "require-reason") {
			t.Fatalf("missing source must fail: %+v", f)
		}
	})

	t.Run("unknown_registry_fails", func(t *testing.T) {
		policy := DependencyPolicy{Profile: "prototype", RegistryAllowlist: []string{"github.com/"}, Declared: map[string]DeclaredDep{
			"gitlab.com/evil/pkg": {Reason: "r", Source: "s"},
		}}
		if f := ScanManifest([]TrackedFile{mod("module x\n\nrequire gitlab.com/evil/pkg v1.0.0\n")}, policy); !hasFinding(f, "manifest", "unknown-registry") {
			t.Fatalf("dependency outside registry allowlist must fail: %+v", f)
		}
	})

	t.Run("lockfile_only_change_is_inspected", func(t *testing.T) {
		// go.sum with a checksum that is not the expected h1: algorithm — a
		// lockfile-only change the slopsquat scanner used to exclude entirely.
		f := ScanManifest([]TrackedFile{sum("github.com/pkg/errors v1.0.0 badalgo:deadbeef\n")}, DependencyPolicy{Profile: "prototype"})
		if !hasFinding(f, "manifest", "unknown-checksum") {
			t.Fatalf("lockfile checksum must be inspected: %+v", f)
		}
	})

	t.Run("valid_lockfile_checksum_passes", func(t *testing.T) {
		f := ScanManifest([]TrackedFile{sum("github.com/pkg/errors v1.0.0 h1:abc\ngithub.com/pkg/errors v1.0.0/go.mod h1:def\n")}, DependencyPolicy{Profile: "prototype"})
		if len(f) != 0 {
			t.Fatalf("well-formed h1: checksums should pass: %+v", f)
		}
	})

	t.Run("severity_follows_profile", func(t *testing.T) {
		in := []TrackedFile{mod("module x\n\nrequire github.com/pkg/errors v1.0.0\n")}
		if f := ScanManifest(in, DependencyPolicy{Profile: "production"}); len(f) == 0 || f[0].Severity != "error" {
			t.Fatalf("production must raise to error: %+v", f)
		}
		if f := ScanManifest(in, DependencyPolicy{Profile: "prototype"}); len(f) == 0 || f[0].Severity != "warn" {
			t.Fatalf("prototype must warn: %+v", f)
		}
	})

	t.Run("non_manifest_files_skipped", func(t *testing.T) {
		if f := ScanManifest([]TrackedFile{{Path: "notes.txt", Content: []byte("require evil v1\n")}}, DependencyPolicy{Profile: "production"}); len(f) != 0 {
			t.Fatalf("only manifests/lockfiles are inspected: %+v", f)
		}
	})
}
