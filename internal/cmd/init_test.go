package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// TestInitSkillTemplatesExist asserts every skill in the scaffold manifest
// ships an embedded SKILL.md template with matching frontmatter.
func TestInitSkillTemplatesExist(t *testing.T) {
	for _, asset := range core.DefaultScaffoldManifest() {
		if !strings.HasPrefix(asset.Target, ".specd/skills/") {
			continue
		}
		content, err := core.ReadTemplate(asset.Template)
		if err != nil {
			t.Errorf("skill template %q is missing: %v", asset.Template, err)
			continue
		}
		parts := strings.Split(asset.Target, "/")
		name := parts[len(parts)-2]
		if !strings.Contains(content, "name:") {
			t.Errorf("skill template %q missing frontmatter name: key", asset.Template)
		}
		if !strings.Contains(content, "name: "+name) {
			t.Errorf("skill template %q frontmatter name does not match dir %q", asset.Template, name)
		}
	}
}

func TestInitRequiredWriteFailure(t *testing.T) {
	t.Run("human output fails closed", func(t *testing.T) {
		root := initTestRoot(t)

		stdout, stderr, code := captureInitOutput(
			t,
			cli.Args{Flags: map[string]string{}},
			initWriteFailure(".specd/steering/reasoning.md"),
		)

		if code != core.ExitGate {
			t.Fatalf("exit = %d, want %d", code, core.ExitGate)
		}
		if !strings.Contains(stderr, ".specd/steering/reasoning.md") {
			t.Fatalf("failed path missing from stderr: %q", stderr)
		}
		if strings.Contains(strings.ToLower(stdout+stderr), "ready, 0 failed") {
			t.Fatalf("failure output claimed readiness: stdout=%q stderr=%q", stdout, stderr)
		}
		if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
			t.Fatalf("AGENTS.md should not be merged after required write failure: %v", err)
		}
	})

	t.Run("json output is one failed result", func(t *testing.T) {
		initTestRoot(t)

		stdout, stderr, code := captureInitOutput(
			t,
			cli.Args{Flags: map[string]string{"json": "true"}},
			initWriteFailure(".specd/steering/reasoning.md"),
		)

		if code != core.ExitGate {
			t.Fatalf("exit = %d, want %d", code, core.ExitGate)
		}
		if stderr != "" {
			t.Fatalf("JSON mode wrote diagnostics to stderr: %q", stderr)
		}
		var result core.InitResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout)
		}
		if result.Status != "failed" {
			t.Fatalf("status = %q, want failed", result.Status)
		}
		if len(result.Files.Failed) != 1 || result.Files.Failed[0] != ".specd/steering/reasoning.md" {
			t.Fatalf("failed paths = %#v", result.Files.Failed)
		}
		if strings.Contains(stdout, "\x1b[") {
			t.Fatalf("JSON contains ANSI escapes: %q", stdout)
		}
	})
}

func initTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	t.Setenv("NO_COLOR", "1")
	t.Setenv("SPECD_JSON", "")
	return root
}

func initWriteFailure(target string) core.InitExecutor {
	executor := core.DefaultInitExecutor()
	executor.WriteFile = func(path, content string) error {
		if strings.HasSuffix(filepath.ToSlash(path), target) {
			return errors.New("injected write failure")
		}
		return core.AtomicWrite(path, content)
	}
	return executor
}

func captureInitOutput(t *testing.T, args cli.Args, executor core.InitExecutor) (stdout, stderr string, code int) {
	t.Helper()
	originalOut, originalErr := os.Stdout, os.Stderr
	readOut, writeOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	readErr, writeErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = writeOut, writeErr
	defer func() { os.Stdout, os.Stderr = originalOut, originalErr }()

	outCh := make(chan string, 1)
	errCh := make(chan string, 1)
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, readOut)
		outCh <- b.String()
	}()
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, readErr)
		errCh <- b.String()
	}()

	code = runInit(args, executor)
	_ = writeOut.Close()
	_ = writeErr.Close()
	stdout, stderr = <-outCh, <-errCh
	_ = readOut.Close()
	_ = readErr.Close()
	return stdout, stderr, code
}
