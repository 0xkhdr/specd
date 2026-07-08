package core

import (
	"os"
	"path/filepath"
	"testing"
)

// program_lease_more_cov_test.go closes the cheap, deterministic branches the
// lifecycle fixtures leave open: the sort tie-breaks (pure), and
// LoadProgramChildLeases' non-directory skip and corrupt-lease error paths
// (planted-file injection).

func TestSortProgramChildLeasesTieBreaks(t *testing.T) {
	leases := []ProgramChildLease{
		{Slug: "b", ParentSessionID: "p1", ChildSessionID: "c1"},
		{Slug: "a", ParentSessionID: "p2", ChildSessionID: "c9"}, // wins on slug
		{Slug: "a", ParentSessionID: "p1", ChildSessionID: "c2"}, // same slug → parent tie-break
		{Slug: "a", ParentSessionID: "p1", ChildSessionID: "c1"}, // same slug+parent → child tie-break
	}
	sortProgramChildLeases(leases)

	order := make([]string, len(leases))
	for i, l := range leases {
		order[i] = l.Slug + "/" + l.ParentSessionID + "/" + l.ChildSessionID
	}
	want := []string{"a/p1/c1", "a/p1/c2", "a/p2/c9", "b/p1/c1"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("sorted[%d] = %s, want %s (full: %v)", i, order[i], want[i], order)
		}
	}
}

func TestLoadProgramChildLeasesSkipsAndRejects(t *testing.T) {
	root := t.TempDir()
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	childrenDir, err := paths.ProgramChildrenDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(childrenDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// A stray regular file in the children dir is not a child slug → skipped.
	if err := os.WriteFile(filepath.Join(childrenDir, "stray"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadProgramChildLeases(root)
	if err != nil {
		t.Fatalf("load with stray file should skip it, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no leases, got %#v", got)
	}

	// A child slug dir holding a corrupt lease surfaces a load error.
	badPath, err := paths.ProgramChildLeasePath("bad")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(badPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProgramChildLeases(root); err == nil {
		t.Fatal("corrupt child lease should surface a load error")
	}
}
