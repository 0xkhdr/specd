package cmd_test

import (
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testutil"
)

// argv is a small helper to build parsed args the way main.go does.
func argv(tokens ...string) cli.Args { return cli.ParseArgs(tokens) }

func TestRunNewExitCodes(t *testing.T) {
	t.Run("rejects_path_traversal_slug_with_usage_code", func(t *testing.T) {
		root := testutil.NewTempSpecdRoot(t)
		testutil.Chdir(t, root)
		if got := cmd.RunNew(argv("../../etc/passwd")); got != core.ExitUsage {
			t.Errorf("RunNew(traversal) = %d, want %d", got, core.ExitUsage)
		}
	})

	t.Run("missing_slug_is_usage_error", func(t *testing.T) {
		root := testutil.NewTempSpecdRoot(t)
		testutil.Chdir(t, root)
		if got := cmd.RunNew(argv()); got != core.ExitUsage {
			t.Errorf("RunNew() = %d, want %d", got, core.ExitUsage)
		}
	})

	t.Run("creates_then_rejects_duplicate", func(t *testing.T) {
		root := testutil.NewTempSpecdRoot(t)
		testutil.Chdir(t, root)
		if got := cmd.RunNew(argv("auth")); got != core.ExitOK {
			t.Fatalf("RunNew(auth) = %d, want %d", got, core.ExitOK)
		}
		if !core.SpecExists(root, "auth") {
			t.Fatal("spec auth not created")
		}
		if got := cmd.RunNew(argv("auth")); got != core.ExitGate {
			t.Errorf("RunNew(auth) duplicate = %d, want %d", got, core.ExitGate)
		}
	})
}

func TestRunCheckExitCodes(t *testing.T) {
	t.Run("unknown_spec_is_not_found", func(t *testing.T) {
		root := testutil.NewTempSpecdRoot(t)
		testutil.Chdir(t, root)
		if got := cmd.RunCheck(argv("ghost")); got != core.ExitNotFound {
			t.Errorf("RunCheck(ghost) = %d, want %d", got, core.ExitNotFound)
		}
	})

	t.Run("traversal_slug_is_usage_error", func(t *testing.T) {
		root := testutil.NewTempSpecdRoot(t)
		testutil.Chdir(t, root)
		if got := cmd.RunCheck(argv("../../../etc")); got != core.ExitUsage {
			t.Errorf("RunCheck(traversal) = %d, want %d", got, core.ExitUsage)
		}
	})

	t.Run("missing_slug_is_usage_error", func(t *testing.T) {
		root := testutil.NewTempSpecdRoot(t)
		testutil.Chdir(t, root)
		if got := cmd.RunCheck(argv()); got != core.ExitUsage {
			t.Errorf("RunCheck() = %d, want %d", got, core.ExitUsage)
		}
	})
}

func TestRunVerifyAndDispatchExitCodes(t *testing.T) {
	t.Run("verify_missing_slug_is_usage_error", func(t *testing.T) {
		root := testutil.NewTempSpecdRoot(t)
		testutil.Chdir(t, root)
		if got := cmd.RunVerify(argv()); got != core.ExitUsage {
			t.Errorf("RunVerify() = %d, want %d", got, core.ExitUsage)
		}
	})

	t.Run("dispatch_unknown_spec_is_not_found", func(t *testing.T) {
		root := testutil.NewTempSpecdRoot(t)
		testutil.Chdir(t, root)
		if got := cmd.RunDispatch(argv("ghost")); got != core.ExitNotFound {
			t.Errorf("RunDispatch(ghost) = %d, want %d", got, core.ExitNotFound)
		}
	})

	t.Run("dispatch_traversal_slug_is_usage_error", func(t *testing.T) {
		root := testutil.NewTempSpecdRoot(t)
		testutil.Chdir(t, root)
		if got := cmd.RunDispatch(argv("../../etc")); got != core.ExitUsage {
			t.Errorf("RunDispatch(traversal) = %d, want %d", got, core.ExitUsage)
		}
	})
}

func TestRunWithoutSpecdRootIsNotFound(t *testing.T) {
	// Arrange: a bare temp dir with no .specd/ anywhere up the tree.
	bare := t.TempDir()
	testutil.Chdir(t, bare)

	// Act + Assert: commands needing a root fail with the not-found code.
	if got := cmd.RunCheck(argv("x")); got != core.ExitNotFound {
		t.Errorf("RunCheck without root = %d, want %d", got, core.ExitNotFound)
	}
}
