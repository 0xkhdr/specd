package core

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// SelectRunner resolves a sandbox name to a Runner, failing closed. The default
// ("none"/"") returns the historical shell runner. Any isolating backend whose
// underlying tool is absent from PATH is a hard error rather than a silent
// fallback — a verify run that asked for isolation must never quietly execute
// unisolated. The verify command surfaces the error and refuses to run.
func SelectRunner(name string) (Runner, error) {
	switch strings.TrimSpace(name) {
	case "", "none":
		return NewShRunner(), nil
	case "bwrap":
		return newBwrapRunner()
	case "container":
		return newContainerRunner()
	default:
		return nil, GateError(fmt.Sprintf("unknown verify sandbox %q (known: none, bwrap, container)", name))
	}
}

// bwrapRunner isolates the verify command with bubblewrap (bwrap): a read-only
// root, a writable bind of the workspace, no network, and a private /proc and
// /dev. It is fail-closed — constructed only when `bwrap` is on PATH.
type bwrapRunner struct{ bin string }

func newBwrapRunner() (Runner, error) {
	bin, err := exec.LookPath("bwrap")
	if err != nil {
		return nil, GateError("verify sandbox \"bwrap\": bubblewrap not found on PATH — refusing to run unisolated (install bubblewrap or set verify.sandbox to \"none\")")
	}
	return bwrapRunner{bin: bin}, nil
}

func (bwrapRunner) Name() string { return "bwrap" }

func (r bwrapRunner) Run(parent context.Context, spec RunSpec) RunResult {
	// Read-only root, writable workspace, isolated namespaces, no network. The
	// workspace bind is writable so the verify command (e.g. a test run that
	// writes coverage files) behaves as it does today, while the rest of the
	// filesystem cannot be mutated.
	args := []string{
		"--die-with-parent",
		"--unshare-all", // includes --unshare-net: no network inside the sandbox
		"--ro-bind", "/", "/",
		"--bind", spec.Root, spec.Root,
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--chdir", spec.Root,
		spec.Shell, "-c", spec.Command,
	}
	return runIsolated(parent, r.bin, args, spec)
}

// containerRunner isolates the verify command in a throwaway OCI container via
// docker or podman: network disabled, the workspace bind-mounted read-write at
// its original path, and the scrubbed env forwarded explicitly. It is
// fail-closed — constructed only when a container engine and an image are both
// available. The image comes from SPECD_SANDBOX_IMAGE.
type containerRunner struct {
	engine string
	image  string
}

func newContainerRunner() (Runner, error) {
	var engine string
	for _, cand := range []string{"docker", "podman"} {
		if p, err := exec.LookPath(cand); err == nil {
			engine = p
			break
		}
	}
	if engine == "" {
		return nil, GateError("verify sandbox \"container\": neither docker nor podman found on PATH — refusing to run unisolated (install a container engine or set verify.sandbox to \"none\")")
	}
	image := strings.TrimSpace(os.Getenv("SPECD_SANDBOX_IMAGE"))
	if image == "" {
		return nil, GateError("verify sandbox \"container\": SPECD_SANDBOX_IMAGE is unset — refusing to run without a pinned image (set SPECD_SANDBOX_IMAGE, e.g. \"golang:1.26\")")
	}
	return containerRunner{engine: engine, image: image}, nil
}

func (containerRunner) Name() string { return "container" }

func (r containerRunner) Run(parent context.Context, spec RunSpec) RunResult {
	args := []string{
		"run", "--rm",
		"--network", "none",
		"--volume", spec.Root + ":" + spec.Root,
		"--workdir", spec.Root,
	}
	// Forward the already-scrubbed env explicitly; do not inherit the host env
	// into the container (the scrubbed slice is the whole contract).
	for _, kv := range spec.Env {
		args = append(args, "--env", kv)
	}
	args = append(args, r.image, spec.Shell, "-c", spec.Command)
	// The engine itself runs on the host; the container inherits only what we
	// pass via --env, so the host env passed to exec is irrelevant here.
	return runIsolated(parent, r.engine, args, RunSpec{
		Root:    spec.Root,
		Timeout: spec.Timeout,
		Env:     spec.Env,
	})
}

// runIsolated runs an isolator binary with the given args under the spec's
// timeout, folding the outcome into the same RunResult shape shRunner produces.
// Timeouts and non-ExitError failures map to exit 124, identical to the default
// runner, so sandbox selection never changes the result vocabulary.
func runIsolated(parent context.Context, bin string, args []string, spec RunSpec) RunResult {
	ctx, cancel := context.WithTimeout(parent, spec.Timeout)
	defer cancel()

	t0 := time.Now()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = spec.Root
	cmd.Env = spec.Env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	durationMs := time.Since(t0).Milliseconds()

	timedOut := ctx.Err() == context.DeadlineExceeded
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
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
