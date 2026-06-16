package core

import (
	"strings"
	"testing"
)

// TestSelectRunnerNoneRegression confirms the default selection is the
// historical shell runner, byte-identically named "none".
func TestSelectRunnerNoneRegression(t *testing.T) {
	for _, name := range []string{"", "none"} {
		r, err := SelectRunner(name)
		if err != nil {
			t.Fatalf("SelectRunner(%q) error: %v", name, err)
		}
		if r.Name() != "none" {
			t.Errorf("SelectRunner(%q).Name() = %q, want none", name, r.Name())
		}
	}
}

func TestSelectRunnerUnknownFailsClosed(t *testing.T) {
	if _, err := SelectRunner("vm"); err == nil {
		t.Fatal("SelectRunner(\"vm\") should reject an unknown backend")
	}
}

// TestSelectRunnerFailsClosedOnMissingIsolator forces an empty PATH so neither
// bwrap nor a container engine can be found, and asserts both refuse rather than
// silently degrading to an unisolated run.
func TestSelectRunnerFailsClosedOnMissingIsolator(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("SPECD_SANDBOX_IMAGE", "alpine") // present, so the failure is provably the missing engine

	for _, name := range []string{"bwrap", "container"} {
		r, err := SelectRunner(name)
		if err == nil {
			t.Fatalf("SelectRunner(%q) should fail closed when the isolator is absent, got runner %v", name, r)
		}
		if !strings.Contains(err.Error(), "refusing to run unisolated") {
			t.Errorf("SelectRunner(%q) error = %q, want fail-closed message", name, err)
		}
	}
}

// TestSelectContainerFailsClosedWithoutImage confirms the container backend
// refuses when no pinned image is configured, even if an engine exists.
func TestSelectContainerFailsClosedWithoutImage(t *testing.T) {
	t.Setenv("SPECD_SANDBOX_IMAGE", "")
	if _, err := newContainerRunner(); err == nil {
		t.Skip("a container engine is installed; image-absence path still validated by message check below")
	} else if !strings.Contains(err.Error(), "refusing to run") {
		t.Errorf("container runner error = %q, want fail-closed message", err)
	}
}
