//go:build !windows

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/obs"
)

const waitDelay = 5 * time.Second

// ShellRunner runs missions with the POSIX shell contract used by Brain.
type ShellRunner struct {
	Stdout io.Writer
	Stderr io.Writer
}

// Run executes m.Command as `sh -c`, passing mission state through a temp JSON
// file and the stable SPECD_* environment contract.
func (r ShellRunner) Run(parent context.Context, m Mission) (Result, error) {
	if endSpan := obs.StartSpan("worker.run"); endSpan != nil {
		defer endSpan()
	}

	start := time.Now()
	payload := m.Payload
	if payload == nil {
		payload = m
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return Result{Duration: time.Since(start)}, err
	}
	f, err := os.CreateTemp("", "specd-mission-*.json")
	if err != nil {
		return Result{Duration: time.Since(start)}, err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		return Result{Duration: time.Since(start)}, err
	}
	if err := f.Close(); err != nil {
		return Result{Duration: time.Since(start)}, err
	}

	// Persist a durable, inspectable mission record at a deterministic spec-scoped
	// runtime path so re-issued attempts overwrite rather than duplicate, and
	// concurrent specs never collide on disk. The temp file above remains the
	// subprocess transport; this is the canonical record (R3.3, R3.4). Best-effort
	// when Root is unset (legacy callers keep the temp-file-only contract).
	if m.Root != "" {
		paths, err := core.NewACPRuntimePaths(m.Root)
		if err != nil {
			return Result{Duration: time.Since(start)}, fmt.Errorf("mission runtime paths: %w", err)
		}
		missionPath, err := paths.MissionPath(m.Spec, m.TaskID, m.Attempt)
		if err != nil {
			return Result{Duration: time.Since(start)}, fmt.Errorf("mission path: %w", err)
		}
		if err := core.AtomicWrite(missionPath, string(raw)); err != nil {
			return Result{Duration: time.Since(start)}, fmt.Errorf("persist mission: %w", err)
		}
	}

	ctx, cancel := deadlineContext(parent, m.Deadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", m.Command) //nolint:gosec // worker command is operator-supplied by design; see SECURITY.md
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = waitDelay

	stdout := r.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := r.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	prefix := "[" + m.WorkerID + "] "
	outW := newLineWriter(prefix, stdout)
	errW := newLineWriter(prefix, stderr)
	cmd.Stdout, cmd.Stderr = outW, errW
	cmd.Env = append(os.Environ(),
		"SPECD_MISSION="+f.Name(),
		"SPECD_SESSION="+m.SessionID,
		"SPECD_WORKER="+m.WorkerID,
		"SPECD_SPEC="+m.Spec,
		"SPECD_TASK="+m.TaskID,
		"SPECD_ROLE="+m.Role,
		"SPECD_ARTIFACT="+artifactHint(m.Files),
	)
	runErr := cmd.Run()
	outW.Flush()
	errW.Flush()

	res := Result{ExitErr: runErr, Duration: time.Since(start)}
	if ctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.ExitErr = fmt.Errorf("worker %s for %s timed out at deadline %s", m.WorkerID, m.TaskID, m.Deadline)
		return res, res.ExitErr
	}
	return res, runErr
}

// deadlineContext builds a context bounded by the mission deadline. An
// unparseable or already-past deadline yields a cancelable context rather than
// an instant cancel, so a clock-skewed deadline still lets the worker start.
func deadlineContext(parent context.Context, deadline string) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	when, err := time.Parse(time.RFC3339Nano, deadline)
	if err != nil || !when.After(time.Now()) {
		return context.WithCancel(parent)
	}
	return context.WithDeadline(parent, when)
}

func artifactHint(files []string) string {
	bases := make([]string, 0, len(files))
	for _, f := range files {
		bases = append(bases, filepath.Base(f))
	}
	return strings.Join(bases, ",")
}
