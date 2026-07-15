package runner

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"

	"github.com/0xkhdr/specd/internal/obs"
)

// RunSpec is the fully-resolved description of one verify execution. The caller
// (cmd/verify) owns policy — env scrubbing, NUL rejection, shell selection — and
// hands the Runner a ready-to-execute spec. Runners must not re-derive policy;
// they only decide *how* the process is isolated.
type RunSpec struct {
	Root    string        // working directory
	Shell   string        // shell binary, e.g. "sh"
	Command string        // command passed to `shell -c`
	Env     []string      // exact environment (already scrubbed)
	Timeout time.Duration // hard wall-clock budget
	Stdin   []byte        // optional bytes fed to the process's stdin (nil = none)
}

// RunResult is the raw outcome of an execution, before it is folded into a
// VerificationRecord. It is presentation- and persistence-free.
type RunResult struct {
	ExitCode   int
	TimedOut   bool
	Stdout     string
	Stderr     string
	DurationMs int64
}

// Runner executes a verify command under some isolation backend. Name reports
// the sandbox identity recorded as evidence ("none" for the default shell
// runner; "bwrap"/"container" for the isolating backends added in a later wave).
// A Runner performs no state IO and renders nothing.
type Runner interface {
	Name() string
	Run(ctx context.Context, spec RunSpec) RunResult
}

// shRunner is the default backend: it reproduces the historical verify execution
// exactly — `shell -c command` in Root with the supplied Env and a timeout — so
// selecting the "none" sandbox is byte-for-byte identical to pre-sandbox specd.
type shRunner struct{}

// NewShRunner returns the default, non-isolating runner.
func NewShRunner() Runner { return shRunner{} }

func (shRunner) Name() string { return "none" }

func (shRunner) Run(parent context.Context, spec RunSpec) RunResult {
	started := time.Now()
	defer func() { obs.RecordDuration("verify_run_duration", time.Since(started)) }()

	ctx, cancel := context.WithTimeout(parent, spec.Timeout)
	defer cancel()

	t0 := time.Now()
	cmd := exec.CommandContext(ctx, spec.Shell, "-c", spec.Command) //nolint:gosec // running operator-authored task commands is the worker's purpose; sandboxing is the operator's responsibility (see SECURITY.md)
	cmd.Dir = spec.Root
	cmd.Env = spec.Env
	if len(spec.Stdin) > 0 {
		cmd.Stdin = bytes.NewReader(spec.Stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	durationMs := time.Since(t0).Milliseconds()

	timedOut := ctx.Err() == context.DeadlineExceeded
	exitCode := 0
	if runErr != nil {
		exitErr := &exec.ExitError{}
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 124
		}
	}
	if timedOut {
		exitCode = 124
	}

	return RunResult{
		ExitCode:   exitCode,
		TimedOut:   timedOut,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMs: durationMs,
	}
}
