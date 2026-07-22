package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snapshotRoot seeds a spec whose three artifacts hold known bytes.
func snapshotRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, artifact := range ReopenableArtifacts {
		if err := AtomicWrite(filepath.Join(SpecdDir(root), "specs", "demo", artifact+".md"), "# "+artifact+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestArtifactSpecReopenSnapshotIsContentAddressed(t *testing.T) {
	root := snapshotRoot(t)
	snapshot, err := SnapshotArtifactRevision(root, "demo", "design")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.Digest != Digest([]byte("# design\n")) || snapshot.Path != "revisions/design/"+snapshot.Digest+".md" {
		t.Fatalf("snapshot = %+v, want the bytes' own content address", snapshot)
	}
	path := filepath.Join(SpecdDir(root), "specs", "demo", snapshot.Path)
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "# design\n" {
		t.Fatalf("snapshot file = %q, err %v, want the preserved bytes", raw, err)
	}

	t.Run("repeat-is-idempotent", func(t *testing.T) {
		again, err := SnapshotArtifactRevision(root, "demo", "design")
		if err != nil || again != snapshot {
			t.Fatalf("snapshot = %+v, err %v, want the identical existing revision", again, err)
		}
	})

	t.Run("mismatched-bytes-fail-closed", func(t *testing.T) {
		if err := AtomicWrite(path, "tampered\n"); err != nil {
			t.Fatal(err)
		}
		_, err := SnapshotArtifactRevision(root, "demo", "design")
		if err == nil || !strings.Contains(err.Error(), "does not hold the bytes it is addressed by") {
			t.Fatalf("snapshot = %v, want a fail-closed refusal on a corrupted revision", err)
		}
	})
}

func TestArtifactSpecReopenSnapshotRefusesUnnormalizedPaths(t *testing.T) {
	root := snapshotRoot(t)
	cases := map[string]struct{ slug, artifact, digest string }{
		"artifact-traversal": {"demo", "../../../etc/passwd", strings.Repeat("a", 64)},
		"artifact-unknown":   {"demo", "evidence", strings.Repeat("a", 64)},
		"slug-traversal":     {"../demo", "design", strings.Repeat("a", 64)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := RevisionSnapshotPath(root, tc.slug, tc.artifact, tc.digest); err == nil {
				t.Fatal("revision path must stay inside the spec revision directory")
			}
			if _, err := SnapshotArtifactRevision(root, tc.slug, tc.artifact); err == nil {
				t.Fatal("snapshot must refuse an unnormalized target")
			}
		})
	}
	t.Run("digest-not-hex", func(t *testing.T) {
		if _, err := RevisionSnapshotPath(root, "demo", "design", "../../escape"); err == nil {
			t.Fatal("a revision address that is not a sha256 digest must refuse")
		}
	})
}
