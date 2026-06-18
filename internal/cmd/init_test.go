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
		normalized := filepath.ToSlash(path)
		stageTarget := strings.TrimPrefix(target, ".specd/")
		if strings.HasSuffix(normalized, target) || strings.HasSuffix(normalized, "/"+stageTarget) {
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

func TestInitDryRunWritesNothingAndListsActions(t *testing.T) {
	root := initTestRoot(t)
	stdout, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{"--dry-run"}), core.DefaultInitExecutor())
	if code != core.ExitOK || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".specd")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote .specd: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote AGENTS.md: %v", err)
	}
	for _, want := range []string{"would write: .specd/config.json", "would update: AGENTS.md"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, stdout)
		}
	}
}

func TestInitRepairAndRefreshPreserveUserContent(t *testing.T) {
	root := initTestRoot(t)
	if _, _, code := captureInitOutput(t, cli.Args{Flags: map[string]string{}}, core.DefaultInitExecutor()); code != core.ExitOK {
		t.Fatalf("initial init exit=%d", code)
	}
	product := filepath.Join(root, ".specd", "steering", "product.md")
	role := filepath.Join(root, ".specd", "roles", "builder.md")
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(product, []byte("user product\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(role); err != nil {
		t.Fatal(err)
	}
	currentAgents, _ := os.ReadFile(agents)
	if err := os.WriteFile(agents, []byte("preamble\n"+string(currentAgents)+"postamble\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{"--repair"}), core.DefaultInitExecutor()); code != core.ExitOK {
		t.Fatalf("repair exit=%d stderr=%q", code, stderr)
	}
	if got, _ := os.ReadFile(product); string(got) != "user product\n" {
		t.Fatalf("repair overwrote product.md: %q", got)
	}
	if _, err := os.Stat(role); err != nil {
		t.Fatalf("repair did not restore role: %v", err)
	}

	if _, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{"--refresh"}), core.DefaultInitExecutor()); code != core.ExitOK {
		t.Fatalf("refresh exit=%d stderr=%q", code, stderr)
	}
	if got, _ := os.ReadFile(product); string(got) != "user product\n" {
		t.Fatalf("refresh overwrote authored product.md: %q", got)
	}
	gotAgents, _ := os.ReadFile(agents)
	if !strings.HasPrefix(string(gotAgents), "preamble\n") || !strings.HasSuffix(string(gotAgents), "postamble\n") {
		t.Fatalf("refresh lost AGENTS.md user content:\n%s", gotAgents)
	}
}

func TestInitModeConflictReturnsUsage(t *testing.T) {
	initTestRoot(t)
	_, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{"--repair", "--refresh"}), core.DefaultInitExecutor())
	if code != core.ExitUsage {
		t.Fatalf("exit=%d want=%d", code, core.ExitUsage)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestInitRefreshRejectsMalformedAgentsWithoutWrite(t *testing.T) {
	root := initTestRoot(t)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("<!-- SPECD INIT: BEGIN v1 (do not edit between markers) -->\nbroken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	_, _, code := captureInitOutput(t, cli.ParseArgs([]string{"--refresh"}), core.DefaultInitExecutor())
	if code != core.ExitGate {
		t.Fatalf("exit=%d want=%d", code, core.ExitGate)
	}
	after, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(after) != string(before) {
		t.Fatal("malformed AGENTS.md changed")
	}
	if _, err := os.Stat(filepath.Join(root, ".specd")); !os.IsNotExist(err) {
		t.Fatalf("preflight failure wrote .specd: %v", err)
	}
}
