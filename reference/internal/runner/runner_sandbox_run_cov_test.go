package runner

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// runner_sandbox_run_cov_test.go exercises runIsolated and the bwrap/container
// Run wrappers directly with stand-in binaries, so the isolation result-folding
// (exit code, timeout→124, non-exec→124) is covered without a real sandbox.

func lookOrSkip(t *testing.T, bin string) string {
	t.Helper()
	p, err := exec.LookPath(bin)
	if err != nil {
		t.Skipf("%s not on PATH", bin)
	}
	return p
}

func TestRunIsolatedFoldsOutcomes(t *testing.T) {
	root := t.TempDir()

	t.Run("clean_exit", func(t *testing.T) {
		bin := lookOrSkip(t, "true")
		res := runIsolated(context.Background(), bin, nil, RunSpec{Root: root, Timeout: 5 * time.Second})
		if res.ExitCode != 0 || res.TimedOut {
			t.Fatalf("clean exit = %+v", res)
		}
	})

	t.Run("nonzero_exit_verbatim", func(t *testing.T) {
		bin := lookOrSkip(t, "false")
		res := runIsolated(context.Background(), bin, nil, RunSpec{Root: root, Timeout: 5 * time.Second})
		if res.ExitCode != 1 || res.TimedOut {
			t.Fatalf("false exit = %+v, want exit 1", res)
		}
	})

	t.Run("missing_binary_maps_to_124", func(t *testing.T) {
		res := runIsolated(context.Background(), "/nonexistent/isolator-bin", nil, RunSpec{Root: root, Timeout: 5 * time.Second})
		if res.ExitCode != 124 || res.TimedOut {
			t.Fatalf("missing binary = %+v, want exit 124", res)
		}
	})

	t.Run("timeout_maps_to_124", func(t *testing.T) {
		bin := lookOrSkip(t, "sleep")
		res := runIsolated(context.Background(), bin, []string{"5"}, RunSpec{Root: root, Timeout: 50 * time.Millisecond})
		if res.ExitCode != 124 || !res.TimedOut {
			t.Fatalf("timeout = %+v, want exit 124 timed out", res)
		}
	})
}

// TestSandboxRunWrappersBuildArgs runs the bwrap/container wrappers with a
// stand-in binary that ignores its arguments, covering Name() and the arg
// assembly + delegation to runIsolated.
func TestSandboxRunWrappersBuildArgs(t *testing.T) {
	root := t.TempDir()
	echo := lookOrSkip(t, "true") // ignores args, exits 0

	bw := bwrapRunner{bin: echo}
	if bw.Name() != "bwrap" {
		t.Errorf("bwrap name = %q", bw.Name())
	}
	if res := bw.Run(context.Background(), RunSpec{Root: root, Shell: "sh", Command: "echo hi", Timeout: 5 * time.Second}); res.ExitCode != 0 {
		t.Errorf("bwrap run exit = %d", res.ExitCode)
	}

	cr := containerRunner{engine: echo, image: "img:latest"}
	if cr.Name() != "container" {
		t.Errorf("container name = %q", cr.Name())
	}
	res := cr.Run(context.Background(), RunSpec{
		Root:    root,
		Shell:   "sh",
		Command: "echo hi",
		Env:     []string{"FOO=bar"},
		Timeout: 5 * time.Second,
	})
	if res.ExitCode != 0 {
		t.Errorf("container run exit = %d", res.ExitCode)
	}
}
