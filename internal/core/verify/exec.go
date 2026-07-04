package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

type Options struct {
	Command       string
	Dir           string
	Sandbox       bool
	SandboxBinary string
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
	if opts.Sandbox {
		binary := opts.SandboxBinary
		if binary == "" {
			binary = "bwrap"
		}
		if _, err := exec.LookPath(binary); err != nil {
			return Result{ExitCode: 127}, fmt.Errorf("sandbox binary %q unavailable: %w", binary, err)
		}
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", opts.Command)
	cmd.Dir = opts.Dir
	cmd.Env = scrubbedEnv(os.Environ())
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
