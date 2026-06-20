package cmd

// Concern (cross-cutting): first-run onboarding UX for init — agent selection,
// consent/scope, and the human-facing receipt/next-action budget.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/integration"
	"github.com/0xkhdr/specd/internal/mcp"
)

type onboardingAdapter struct {
	name       string
	detected   bool
	scopes     []integration.Scope
	registered bool
	owned      bool
	installs   int
}

func (a *onboardingAdapter) Name() string { return a.name }
func (a *onboardingAdapter) Scopes() []integration.Scope {
	return append([]integration.Scope(nil), a.scopes...)
}
func (a *onboardingAdapter) Detect(string) integration.Detection {
	return integration.Detection{
		Host:       a.name,
		Detected:   a.detected,
		Scopes:     a.Scopes(),
		Confidence: integration.ConfidenceHigh,
		Reason:     "test detection",
	}
}
func (a *onboardingAdapter) Plan(root string, scope integration.Scope) (integration.HostPlan, error) {
	return integration.HostPlan{
		Host: a.name, Root: root, Scope: scope,
		Actions:  []integration.HostAction{{Kind: "test", Target: filepath.Join(root, "."+a.name), Description: "configure test host"}},
		Warnings: []string{},
	}, nil
}
func (a *onboardingAdapter) Install(context.Context, integration.HostPlan) (integration.HostResult, error) {
	a.installs++
	a.registered = true
	a.owned = true
	return integration.HostResult{
		Host: a.name, Status: "configured", Changed: true,
		Targets: []string{}, Backups: []string{}, Warnings: []string{},
	}, nil
}
func (a *onboardingAdapter) Inspect(_ string, scope integration.Scope) (integration.HostState, error) {
	return integration.HostState{
		Host: a.name, Scope: scope, Registered: a.registered, Owned: a.owned,
		Reason: "test registration state",
	}, nil
}
func (a *onboardingAdapter) Verify(string) integration.Verification {
	if !a.registered {
		return integration.Verification{
			Host: a.name, Status: "fail", Reason: "not registered",
			Remedy: "run `specd doctor --fix`",
		}
	}
	return integration.Verification{Host: a.name, Status: "pass", Reason: "registered"}
}

func passingProbe(context.Context, mcp.Dispatcher, time.Duration) (mcp.ProbeResult, error) {
	return mcp.ProbeResult{
		ProtocolVersion:    "2025-11-25",
		ToolCount:          24,
		RequiredTools:      []string{"specd_init", "specd_status", "specd_brain", "specd_pinky"},
		OrchestrationTools: []string{"specd_brain", "specd_pinky"},
	}, nil
}

func TestInitAgentSelectionConsentAndScope(t *testing.T) {
	t.Run("ambiguous_non_interactive_auto_mutates_no_host", func(t *testing.T) {
		initTestRoot(t)
		codex := &onboardingAdapter{name: "codex", detected: true, scopes: []integration.Scope{integration.ScopeProject}}
		claude := &onboardingAdapter{name: "claude-code", detected: true, scopes: []integration.Scope{integration.ScopeProject}}
		runtime := onboardingRuntime{
			Registry: integration.MustRegistry(codex, claude),
			Probe:    passingProbe,
			Input:    strings.NewReader(""),
			Interactive: func() bool {
				return false
			},
		}

		stdout, _, code := captureOutput(t, func() int {
			return runInitWithRuntime(cli.ParseArgs([]string{"--non-interactive", "--json"}), core.DefaultInitExecutor(), runtime)
		})
		if code != core.ExitOK {
			t.Fatalf("exit=%d stdout=%s", code, stdout)
		}
		if codex.installs != 0 || claude.installs != 0 {
			t.Fatalf("ambiguous auto installed hosts: codex=%d claude=%d", codex.installs, claude.installs)
		}
		var result core.InitResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Agents.Configured) != 0 || len(result.Agents.Manual) != 2 {
			t.Fatalf("agents=%+v", result.Agents)
		}
	})

	t.Run("explicit_non_interactive_host_installs", func(t *testing.T) {
		initTestRoot(t)
		host := &onboardingAdapter{name: "codex", detected: true, scopes: []integration.Scope{integration.ScopeProject}}
		runtime := onboardingRuntime{Registry: integration.MustRegistry(host), Probe: passingProbe, Input: strings.NewReader(""), Interactive: func() bool { return false }}
		_, _, code := captureOutput(t, func() int {
			return runInitWithRuntime(cli.ParseArgs([]string{"--agent", "codex", "--non-interactive"}), core.DefaultInitExecutor(), runtime)
		})
		if code != core.ExitOK || host.installs != 1 {
			t.Fatalf("exit=%d installs=%d", code, host.installs)
		}
	})

	t.Run("interactive_ambiguity_honors_selected_host", func(t *testing.T) {
		initTestRoot(t)
		codex := &onboardingAdapter{name: "codex", detected: true, scopes: []integration.Scope{integration.ScopeProject}}
		claude := &onboardingAdapter{name: "claude-code", detected: true, scopes: []integration.Scope{integration.ScopeProject}}
		runtime := onboardingRuntime{
			Registry:    integration.MustRegistry(codex, claude),
			Probe:       passingProbe,
			Input:       strings.NewReader("claude-code\n"),
			Interactive: func() bool { return true },
		}
		_, _, code := captureOutput(t, func() int {
			return runInitWithRuntime(cli.Args{Flags: map[string]string{}}, core.DefaultInitExecutor(), runtime)
		})
		if code != core.ExitOK || claude.installs != 1 || codex.installs != 0 {
			t.Fatalf("exit=%d codex=%d claude=%d", code, codex.installs, claude.installs)
		}
	})

	t.Run("unavailable_explicit_host_fails_before_scaffold", func(t *testing.T) {
		root := initTestRoot(t)
		host := &onboardingAdapter{name: "codex", detected: false, scopes: []integration.Scope{integration.ScopeProject}}
		runtime := onboardingRuntime{Registry: integration.MustRegistry(host), Probe: passingProbe, Interactive: func() bool { return false }}
		_, _, code := captureOutput(t, func() int {
			return runInitWithRuntime(cli.ParseArgs([]string{"--agent", "codex", "--non-interactive"}), core.DefaultInitExecutor(), runtime)
		})
		if code != core.ExitUsage {
			t.Fatalf("exit=%d want=%d", code, core.ExitUsage)
		}
		if _, ok := core.FindSpecdRoot(root); ok {
			t.Fatal("unavailable host selection wrote scaffold")
		}
	})

	t.Run("global_non_interactive_requires_yes", func(t *testing.T) {
		initTestRoot(t)
		host := &onboardingAdapter{name: "codex", detected: true, scopes: []integration.Scope{integration.ScopeProject, integration.ScopeGlobal}}
		runtime := onboardingRuntime{Registry: integration.MustRegistry(host), Probe: passingProbe, Interactive: func() bool { return false }}
		_, _, code := captureOutput(t, func() int {
			return runInitWithRuntime(cli.ParseArgs([]string{"--agent", "codex", "--scope", "global", "--non-interactive"}), core.DefaultInitExecutor(), runtime)
		})
		if code != core.ExitUsage || host.installs != 0 {
			t.Fatalf("exit=%d installs=%d", code, host.installs)
		}
	})
}

func TestInitHumanReceiptNextActionBudget(t *testing.T) {
	initTestRoot(t)
	runtime := onboardingRuntime{Registry: integration.MustRegistry(), Probe: passingProbe, Interactive: func() bool { return false }}
	stdout, stderr, code := captureOutput(t, func() int {
		return runInitWithRuntime(cli.ParseArgs([]string{"--agent", "none", "--non-interactive"}), core.DefaultInitExecutor(), runtime)
	})
	if code != core.ExitOK || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	next := -1
	for i, line := range lines {
		if strings.Contains(line, "Next:") {
			next = i + 1
			break
		}
	}
	if next == -1 || next > 12 {
		t.Fatalf("next action line=%d output:\n%s", next, stdout)
	}
}

func captureOutput(t *testing.T, run func() int) (stdout, stderr string, code int) {
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

	code = run()
	_ = writeOut.Close()
	_ = writeErr.Close()
	stdout, stderr = <-outCh, <-errCh
	_ = readOut.Close()
	_ = readErr.Close()
	return stdout, stderr, code
}
