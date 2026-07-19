package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestGuideForSpec pins spec 01 R6.1: a spec's driving guidance names the phase
// and required artifact, lists machine-legal commands, and keeps approval in the
// human-only set — never as a self-serve command for an agent.
func TestGuideForSpec(t *testing.T) {
	root := newDemoSpec(t)
	g, err := guidanceForSpec(root, "demo")
	if err != nil {
		t.Fatalf("guidanceForSpec: %v", err)
	}
	if g.Phase != core.PhasePerceive || g.RequiredArtifact != "requirements.md" {
		t.Fatalf("guidance = %+v", g)
	}
	if !containsStr(g.HumanOnly, "approve") {
		t.Fatalf("approve must be human-only, got %v", g.HumanOnly)
	}
	if containsStr(g.LegalCommands, "approve") {
		t.Fatalf("approve must not be a machine-legal command, got %v", g.LegalCommands)
	}
}

func containsStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// TestEveryCommandHasHandler is the parity guard (R13.2): every verb in
// core.Commands must resolve to a non-nil handler or carry Deferred:true.
func TestEveryCommandHasHandler(t *testing.T) {
	for _, command := range core.Commands {
		if command.Deferred {
			continue
		}
		if executable[command.Name] == nil {
			t.Errorf("command %q has no handler and is not marked Deferred", command.Name)
		}
	}
}

func TestRegistryEveryOperationHasHandler(t *testing.T) {
	for _, operation := range core.Operations {
		command, ok := core.CommandByName(operation.Command)
		if !ok {
			t.Errorf("operation %q has unknown command %q", operation.ID, operation.Command)
			continue
		}
		if !command.Deferred && executable[operation.Command] == nil {
			t.Errorf("operation %q has no handler", operation.ID)
		}
	}
}

func TestOperationResolutionFailsClosed(t *testing.T) {
	tests := []struct {
		command string
		args    []string
		flags   map[string]string
		want    string
	}{
		{"eval", []string{"import", "demo", "eval.jsonl"}, nil, "eval.import"},
		{"eval", []string{"status", "demo"}, nil, "eval.status"},
		{"task", []string{"T1"}, map[string]string{"override": "", "reason": "human"}, "task.override"},
		{"complete-task", []string{"demo", "T1"}, nil, "complete-task"},
	}
	for _, tt := range tests {
		op, ok := core.ResolveOperation(tt.command, tt.args, tt.flags)
		if !ok || op.ID != tt.want {
			t.Errorf("ResolveOperation(%q, %v) = %q, %t; want %q", tt.command, tt.args, op.ID, ok, tt.want)
		}
	}
	if _, ok := core.ResolveOperation("eval", []string{"erase", "demo"}, nil); ok {
		t.Fatal("unknown eval operation resolved instead of failing closed")
	}
}

// TestUnknownCommandFailsClosed guards R13.1: an unregistered verb returns
// ErrUnknownCommand so the dispatcher can exit 2 instead of 0.
func TestUnknownCommandFailsClosed(t *testing.T) {
	err := Run(".", "bogusverb", nil, nil)
	if !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("unknown verb must wrap ErrUnknownCommand (exit 2), got %v", err)
	}
}

func TestRegistryAgentsDoctor(t *testing.T) {
	root := t.TempDir()
	out, err := captureStdout(t, func() error { return Run(root, "agents", []string{"doctor"}, map[string]string{"json": ""}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "LAYOUT_MISSING") {
		t.Fatalf("doctor false-pass: %s", out)
	}
}

func TestRegistryAgentsGuideV1(t *testing.T) {
	root := newDemoSpec(t)
	out, err := captureStdout(t, func() error { return Run(root, "agents", []string{"guide", "demo"}, map[string]string{"json": ""}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"protocol_version": "1"`) || !strings.Contains(out, `"actor": "human"`) {
		t.Fatalf("driver guide missing contract/human action: %s", out)
	}
}
