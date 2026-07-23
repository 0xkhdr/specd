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
	// NoTestsExitCode marks a `go test -run` selector whose pattern matched no
	// test in any package it reached (spec R2.3). go test still exits 0 in that
	// case, so Run rewrites the exit code to this and names the selector in
	// stderr; the recorder then refuses an empty run as passing evidence instead
	// of banking a green that proved nothing.
	NoTestsExitCode = 126
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
		if selector, empty := runSelectorMatchedNothing(opts.Command, result.Stdout+result.Stderr); empty {
			result.ExitCode = NoTestsExitCode
			result.Stderr += fmt.Sprintf("\n[specd: run selector %q matched no tests in any package]\n", selector)
		}
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

// runSelectorMatchedNothing reports whether command is a `go test -run` selector
// whose output shows every package it reached ran no test, returning the -run
// pattern so the refusal can name what matched nothing (spec R2.3). Pure over
// the captured output: an `ok` line without the `[no tests to run]` marker
// proves at least one package executed a selected test, so a multi-package
// command stays valid when any package runs matching tests. Non-selector and
// non-`go test` commands are never refused here.
// ponytail: line-prefix scan of go's host-agnostic summary lines; a change to
// go's `ok`/`[no tests to run]` wording is the single upgrade point.
func runSelectorMatchedNothing(command, output string) (string, bool) {
	if !strings.Contains(command, "go test") {
		return "", false
	}
	selector, ok := goTestRunPattern(command)
	if !ok {
		return "", false
	}
	var sawPackage, anyExecuted bool
	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "ok "):
			sawPackage = true
			if !strings.Contains(line, "[no tests to run]") {
				anyExecuted = true
			}
		case strings.HasPrefix(line, "?"):
			sawPackage = true
		}
	}
	return selector, sawPackage && !anyExecuted
}

// goTestRunPattern extracts the `-run <pattern>` (or `-run=<pattern>`) value from
// a shell command, stripping one layer of surrounding quotes.
func goTestRunPattern(command string) (string, bool) {
	fields := strings.Fields(command)
	for i, f := range fields {
		if v, ok := strings.CutPrefix(f, "-run="); ok {
			return strings.Trim(v, `"'`), true
		}
		if f == "-run" && i+1 < len(fields) {
			return strings.Trim(fields[i+1], `"'`), true
		}
	}
	return "", false
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
