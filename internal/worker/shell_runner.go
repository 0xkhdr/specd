package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
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

	ctx, cancel := deadlineContext(parent, m.Deadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", m.Command)
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

var outputMu sync.Mutex

type lineWriter struct {
	prefix string
	dst    io.Writer
	buf    []byte
}

func newLineWriter(prefix string, dst io.Writer) *lineWriter {
	return &lineWriter{prefix: prefix, dst: dst}
}

func (w *lineWriter) Write(p []byte) (int, error) {
	outputMu.Lock()
	defer outputMu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		if _, err := fmt.Fprint(w.dst, w.prefix); err != nil {
			return 0, err
		}
		if _, err := w.dst.Write(w.buf[:i+1]); err != nil {
			return 0, err
		}
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

// Flush emits any trailing partial line (no newline) so nothing is dropped when
// the worker exits.
func (w *lineWriter) Flush() {
	outputMu.Lock()
	defer outputMu.Unlock()
	if len(w.buf) > 0 {
		fmt.Fprint(w.dst, w.prefix)
		w.dst.Write(w.buf)
		fmt.Fprintln(w.dst)
		w.buf = nil
	}
}
