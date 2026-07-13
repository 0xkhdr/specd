package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Options struct {
	Command        string
	Dir            string
	Sandbox        bool
	RequireSandbox bool
	SandboxBinary  string
	Stdin          string
	TimeoutSecs    int
	Limits         Limits
	Secrets        []string
	Adapter        *SandboxAdapterV1
}

const (
	TimeoutExitCode = 124
	LimitExitCode   = 125
)

type Result struct {
	ExitCode       int
	Stdout, Stderr string
}

func Run(ctx context.Context, opts Options) (Result, error) {
	if opts.Command == "" {
		return Result{ExitCode: 2}, errors.New("verify command is required")
	}
	limits := opts.Limits
	if opts.RequireSandbox && limits == (Limits{}) {
		limits = DefaultLimits
	}
	timeout := opts.TimeoutSecs
	if limits.WallSeconds > 0 && (timeout <= 0 || limits.WallSeconds < timeout) {
		timeout = limits.WallSeconds
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}
	runCtx, stop := context.WithCancel(ctx)
	defer stop()

	env := os.Environ()
	secrets := append(environmentSecrets(env), opts.Secrets...)
	name, argv := sandboxArgv("", opts.Dir, "", opts.Command, limits)
	sandbox := opts.Sandbox || opts.RequireSandbox
	if sandbox {
		binary := opts.SandboxBinary
		if opts.Adapter != nil {
			if err := opts.Adapter.Validate(opts.RequireSandbox); err != nil {
				return Result{ExitCode: 127}, fmt.Errorf("sandbox adapter refused: %w", err)
			}
			if opts.Adapter.Binary != "" {
				binary = opts.Adapter.Binary
			}
		}
		if binary == "" {
			binary = "bwrap"
		}
		resolved, err := exec.LookPath(binary)
		if err != nil {
			return Result{ExitCode: 127}, fmt.Errorf("sandbox binary %q unavailable: %w", binary, err)
		}
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return Result{ExitCode: 127}, errors.New("sandbox cannot resolve host home for credential isolation")
		}
		name, argv = sandboxArgv(resolved, opts.Dir, home, opts.Command, limits)
	}
	cmd := exec.CommandContext(runCtx, name, argv...)
	cmd.Dir = opts.Dir
	cmd.Env = scrubbedEnv(env, sandbox)
	if opts.Stdin != "" {
		cmd.Stdin = strings.NewReader(opts.Stdin)
	}
	var stdout, stderr bytes.Buffer
	budget := newOutputBudget(limits.OutputBytes, stop)
	cmd.Stdout = budget.writer(&stdout)
	cmd.Stderr = budget.writer(&stderr)
	err := cmd.Run()
	r := NewRedactor(secrets)
	result := Result{Stdout: r.String(stdout.String()), Stderr: r.String(stderr.String())}
	if budget.exceededLimit() {
		result.ExitCode = LimitExitCode
		result.Stderr += "\n[specd: output limit exceeded]\n"
		return result, nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		result.ExitCode = TimeoutExitCode
		result.Stderr += fmt.Sprintf("\n[specd: verify timed out after %ds]\n", timeout)
		return result, nil
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	result.ExitCode = 1
	return result, err
}

func sandboxArgv(binary, dir, hostHome, command string, limits Limits) (string, []string) {
	command = limits.shellPrefix() + command
	if binary == "" {
		return "/bin/sh", []string{"-c", command}
	}
	args := []string{"--die-with-parent", "--unshare-all", "--new-session", "--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc", "--tmpfs", "/tmp", "--dir", "/tmp/specd-home"}
	if hostHome != "" && hostHome != "/" {
		args = append(args, "--tmpfs", hostHome)
		if rel, err := filepath.Rel(hostHome, dir); err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			current := hostHome
			for _, part := range strings.Split(rel, string(filepath.Separator)) {
				current = filepath.Join(current, part)
				args = append(args, "--dir", current)
			}
		}
	}
	if dir != "" {
		args = append(args, "--bind", dir, dir, "--chdir", dir)
	}
	args = append(args, "--setenv", "HOME", "/tmp/specd-home", "--setenv", "TMPDIR", "/tmp", "--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin", "/bin/sh", "-c", command)
	return binary, args
}

type outputBudget struct {
	mu        sync.Mutex
	remaining int
	limited   bool
	exceeded  bool
	cancel    context.CancelFunc
}

func newOutputBudget(limit int, cancel context.CancelFunc) *outputBudget {
	return &outputBudget{remaining: limit, limited: limit > 0, cancel: cancel}
}

func (b *outputBudget) writer(dst *bytes.Buffer) *budgetWriter {
	return &budgetWriter{budget: b, dst: dst}
}

func (b *outputBudget) exceededLimit() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.exceeded
}

type budgetWriter struct {
	budget *outputBudget
	dst    *bytes.Buffer
}

func (w *budgetWriter) Write(p []byte) (int, error) {
	w.budget.mu.Lock()
	defer w.budget.mu.Unlock()
	original := len(p)
	if !w.budget.limited {
		_, _ = w.dst.Write(p)
		return original, nil
	}
	if len(p) > w.budget.remaining {
		p = p[:w.budget.remaining]
		w.budget.exceeded = true
	}
	_, _ = w.dst.Write(p)
	w.budget.remaining -= len(p)
	if w.budget.exceeded {
		w.budget.cancel()
	}
	return original, nil
}

func scrubbedEnv(env []string, sandboxMode ...bool) []string {
	if len(sandboxMode) > 0 && sandboxMode[0] {
		return []string{"HOME=/tmp/specd-home", "PATH=/usr/local/bin:/usr/bin:/bin", "TMPDIR=/tmp"}
	}
	allowed := map[string]bool{"HOME": true, "PATH": true, "TMPDIR": true}
	var out []string
	for _, item := range env {
		key, _, ok := strings.Cut(item, "=")
		if ok && allowed[key] {
			out = append(out, item)
		}
	}
	return out
}
