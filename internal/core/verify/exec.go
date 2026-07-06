package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Options struct {
	Command       string
	Dir           string
	Sandbox       bool
	SandboxBinary string
	// Stdin, when non-empty, is streamed to the command's standard input. The
	// terminal `submit` verb uses it to pipe the PR summary to the
	// operator-configured command through this one exec path (spec 08 R2) — no
	// second exec implementation.
	Stdin string
}

type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

func Run(ctx context.Context, opts Options) (Result, error) {
	if opts.Command == "" {
		return Result{ExitCode: 2}, errors.New("verify command is required")
	}
	name, argv := wrapArgv("", opts.Dir, opts.Command)
	if opts.Sandbox {
		binary := opts.SandboxBinary
		if binary == "" {
			binary = "bwrap"
		}
		resolved, err := exec.LookPath(binary)
		if err != nil {
			return Result{ExitCode: 127}, fmt.Errorf("sandbox binary %q unavailable: %w", binary, err)
		}
		name, argv = wrapArgv(resolved, opts.Dir, opts.Command)
	}
	cmd := exec.CommandContext(ctx, name, argv...)
	cmd.Dir = opts.Dir
	cmd.Env = scrubbedEnv(os.Environ())
	if opts.Stdin != "" {
		cmd.Stdin = strings.NewReader(opts.Stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
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

// wrapArgv builds the (name, argv) to execute command. With binary empty the
// command runs directly under /bin/sh. With binary set the command is wrapped in
// a bwrap-style sandbox: read-only root, private /tmp, no network
// (--unshare-all), and dir bind-mounted writable as the working directory. The
// sandbox binary must accept bwrap-compatible arguments (bwrap by default, or a
// bwrap-compatible wrapper via --sandbox-binary). Kept pure and side-effect free
// so the isolation contract is unit-tested without spawning a process.
func wrapArgv(binary, dir, command string) (string, []string) {
	if binary == "" {
		return "/bin/sh", []string{"-c", command}
	}
	args := []string{
		"--die-with-parent", "--unshare-all",
		"--ro-bind", "/", "/",
		"--dev", "/dev", "--proc", "/proc", "--tmpfs", "/tmp",
	}
	if dir != "" {
		args = append(args, "--bind", dir, dir, "--chdir", dir)
	}
	args = append(args, "/bin/sh", "-c", command)
	return binary, args
}

func scrubbedEnv(env []string) []string {
	allowed := map[string]bool{"HOME": true, "PATH": true, "TMPDIR": true}
	out := make([]string, 0, len(env))
	for _, item := range env {
		for key := range allowed {
			if len(item) > len(key) && item[:len(key)+1] == key+"=" {
				out = append(out, item)
			}
		}
	}
	return out
}
