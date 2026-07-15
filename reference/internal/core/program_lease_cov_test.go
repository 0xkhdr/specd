package core

import (
	"strings"
	"testing"
)

// program_lease_cov_test.go covers AcquireProgramChildLease's validation and
// ownership branches, lease re-acquire idempotence, and the complete-children
// release sweep.

func TestAcquireProgramChildLeaseValidation(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	cfg, _ := programTestPolicy(t)
	parentID := strings.Repeat("a", 32)

	// Bad parent ID, bad slug, and bad config each fail before any write.
	if _, err := AcquireProgramChildLease(root, "short", "a", cfg); err == nil {
		t.Error("bad parent ID should error")
	}
	if _, err := AcquireProgramChildLease(root, parentID, "../bad", cfg); err == nil {
		t.Error("bad slug should error")
	}
	if _, err := AcquireProgramChildLease(root, parentID, "a", OrchestrationCfg{}); err == nil {
		t.Error("invalid config should error")
	}

	// First acquire succeeds; re-acquire by the same parent is idempotent.
	first, err := AcquireProgramChildLease(root, parentID, "a", cfg)
	if err != nil {
		t.Fatal(err)
	}
	again, err := AcquireProgramChildLease(root, parentID, "a", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if again.ChildSessionID != first.ChildSessionID {
		t.Fatalf("re-acquire produced a new lease: %s vs %s", again.ChildSessionID, first.ChildSessionID)
	}

	// A different parent cannot steal an active lease.
	if _, err := AcquireProgramChildLease(root, strings.Repeat("b", 32), "a", cfg); err == nil {
		t.Error("active lease held by another parent should not be acquirable")
	}
}

func TestReleaseCompleteProgramChildren(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusComplete)
	scaffoldSpec(t, root, "b", StatusExecuting)
	cfg, _ := programTestPolicy(t)
	parentID := strings.Repeat("c", 32)

	if _, err := AcquireProgramChildLease(root, parentID, "a", cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireProgramChildLease(root, parentID, "b", cfg); err != nil {
		t.Fatal(err)
	}

	graph, err := BuildProgram(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := releaseCompleteProgramChildren(root, graph); err != nil {
		t.Fatal(err)
	}

	leases, err := LoadProgramChildLeases(root)
	if err != nil {
		t.Fatal(err)
	}
	byslug := map[string]ProgramChildLease{}
	for _, l := range leases {
		byslug[l.Slug] = l
	}
	// Complete spec "a" was released; in-flight "b" stays active.
	if byslug["a"].Status != ProgramChildLeaseReleased {
		t.Errorf("complete child a status = %s, want released", byslug["a"].Status)
	}
	if byslug["b"].Status != ProgramChildLeaseActive {
		t.Errorf("active child b status = %s, want active", byslug["b"].Status)
	}
}

func TestReleaseProgramChildLeaseAnyParentNoLease(t *testing.T) {
	root := t.TempDir()
	// No lease for "ghost" → returns zero value, no error.
	released, err := releaseProgramChildLeaseAnyParent(root, "ghost")
	if err != nil {
		t.Fatalf("missing lease should be a no-op, got %v", err)
	}
	if released.Slug != "" {
		t.Errorf("expected zero lease, got %#v", released)
	}
}
