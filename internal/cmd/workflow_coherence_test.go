package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestWorkflowCoherenceAgentCompletionGuidance(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, nil, map[string]string{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "verify alone does not complete task") || !strings.Contains(string(body), "specd complete-task <slug> <task>") {
		t.Fatalf("completion guidance is not truthful/executable:\n%s", body)
	}
	if !core.ForbiddenTool("task") {
		t.Fatal("broad task command unexpectedly exposed through tool contracts")
	}
}

func TestGuidanceDispatchParity(t *testing.T) {
	for _, operation := range core.Operations {
		if _, ok := Registry[operation.Command]; !ok {
			t.Errorf("operation %s has no CLI dispatcher", operation.ID)
		}
	}
	g := core.GuidanceForRoutes(core.StatusTasks, nil, core.RouteContext{
		Transport: core.RouteCLI, Actor: core.ActorAgent, Authority: core.RouteAuthorityMissing,
	})
	for _, command := range g.LegalCommands {
		if _, ok := Registry[command]; !ok {
			t.Errorf("guidance advertises undispatchable command %s", command)
		}
		for _, forbidden := range []string{"approve", "verify", "complete-task"} {
			if command == forbidden {
				t.Errorf("guidance advertises %s without its actor/authority precondition", command)
			}
		}
	}
}
