package testharness

import (
	"bytes"
	"io"
	"os"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
)

// Result is the captured outcome of a single CLI invocation.
type Result struct {
	Code   int    // process exit code
	Stdout string // everything written to os.Stdout during the command
	Stderr string // everything written to os.Stderr during the command
}

// OK reports whether the command exited 0.
func (r Result) OK() bool { return r.Code == core.ExitOK }

// Out is Stdout+Stderr joined, for substring assertions that don't care which
// stream a message landed on.
func (r Result) Out() string { return r.Stdout + r.Stderr }

// Run executes a specd subcommand in-process — the same dispatch main.go
// performs — while capturing stdout, stderr and the integer exit code. Flags and
// positionals are parsed exactly like the real CLI via cli.ParseArgs, so
// `h.Run("task", "auth", "T1", "--status", "running")` mirrors the shell.
//
// Run mutates os.Stdout/os.Stderr for the duration of the call; tests using it
// must not run in parallel.
func (h *Harness) Run(command string, args ...string) Result {
	h.T.Helper()
	parsed := cli.ParseArgs(args)
	out, errOut, code := capture(func() int { return dispatch(command, parsed) })
	return Result{Code: code, Stdout: out, Stderr: errOut}
}

// RunExpect runs the command and fails the test if the exit code differs from
// want, surfacing the captured streams for diagnosis.
func (h *Harness) RunExpect(want int, command string, args ...string) Result {
	h.T.Helper()
	res := h.Run(command, args...)
	if res.Code != want {
		h.T.Fatalf("specd %s %s: exit = %d, want %d\nstdout: %s\nstderr: %s",
			command, strings.Join(args, " "), res.Code, want, res.Stdout, res.Stderr)
	}
	return res
}

// dispatch routes through the same cmd.Registry as main.go, so test runs
// exercise the production dispatch table (no parallel switch to drift).
func dispatch(command string, args cli.Args) int {
	if code, ok := cmd.Dispatch(command, args); ok {
		return code
	}
	core.Error("unknown command: " + command)
	return core.ExitUsage
}

// capture redirects os.Stdout and os.Stderr through pipes for the duration of
// fn, draining each pipe in its own goroutine so large outputs (reports, JSON)
// never deadlock on a full pipe buffer.
func capture(fn func() int) (stdout, stderr string, code int) {
	origOut, origErr := os.Stdout, os.Stderr

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr

	outCh := drain(rOut)
	errCh := drain(rErr)

	defer func() {
		// Restore even if fn panics, so a failing test doesn't corrupt the
		// stdout of the rest of the suite.
		os.Stdout, os.Stderr = origOut, origErr
	}()

	code = fn()

	_ = wOut.Close()
	_ = wErr.Close()
	return <-outCh, <-errCh, code
}

func drain(r *os.File) <-chan string {
	ch := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		ch <- buf.String()
	}()
	return ch
}
