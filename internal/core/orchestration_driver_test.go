package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const passingRequirements = `# Requirements — Auth

## Requirement 1 — Login
**User story:** As a user, I want to log in, so that I can access my account.

**Acceptance criteria:**
1. WHEN a user submits valid credentials the system SHALL grant access
`

const passingDesign = `# Design — Auth

## Overview
Auth feature overview.

## Architecture
Layered architecture.

## Components and interfaces
Handlers and stores.

## Data models
User and Session.

## Error handling
Errors are returned, not panicked.

## Verification strategy
go test ./...

## Risks and open questions
Token rotation is open.
`

const passingTasks = `# Tasks — Auth

## Wave 1
- [ ] T1 — Build login
  - why: Requirement 1 needs a handler
  - role: builder
  - files: auth.go
  - contract: Implement login
  - acceptance: returns 200
  - verify: go test ./...
  - depends: —
  - requirements: 1
`

// TestDriveOrchestrationBeginningToDelivery is the Milestone A golden test: a
// fresh spec at `requirements` with no planning artifacts is driven by the
// reference loop, under the planning policy, through authoring → advance →
// executing → verifying → complete, with a stub worker that does the creative
// work. It proves the dispatch→spawn handoff with zero model call in core.
func TestDriveOrchestrationBeginningToDelivery(t *testing.T) {
	root := t.TempDir()
	slug := "auth"
	if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
		t.Fatal(err)
	}
	state := InitialState(slug, "Auth")
	state.Status = StatusRequirements
	state.Phase = PhaseForStatus(StatusRequirements)
	if err := SaveState(root, slug, &state); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig.Orchestration
	policy, err := NewOrchestrationPolicy(cfg)
	if err != nil {
		t.Fatal(err)
	}
	policy.ApprovalPolicy = "planning"
	sessionID := strings.Repeat("7", 32)
	if _, err := StartOrchestrationSession(root, slug, sessionID, "tester", policy); err != nil {
		t.Fatal(err)
	}

	authored := map[string]bool{}
	dispatched := []string{}
	worker := func(d DriverDispatch) error {
		dispatched = append(dispatched, string(d.Decision.Action)+":"+d.Decision.TaskID)
		switch d.Decision.Action {
		case OrchestrationDispatchAuthor:
			authored[d.Decision.Artifact] = true
			return os.WriteFile(filepath.Join(SpecDir(root, slug), d.Decision.Artifact), []byte(artifactFixture(d.Decision.Artifact)), 0o644)
		case OrchestrationDispatch:
			// Stand in for a real worker: complete the task with evidence and
			// move the spec to verifying once the frontier is empty.
			st, err := LoadState(root, slug)
			if err != nil {
				return err
			}
			ts := st.Tasks[d.Decision.TaskID]
			ts.Status = TaskComplete
			ts.Verification = &VerificationRecord{Verified: true, ExitCode: 0, Command: "go test ./..."}
			st.Tasks[d.Decision.TaskID] = ts
			st.Status = StatusVerifying
			st.Phase = PhaseForStatus(StatusVerifying)
			return SaveState(root, slug, st)
		}
		return nil
	}

	result, err := DriveOrchestration(root, slug, sessionID, policy, cfg, DriverOptions{MaxSteps: 40, Worker: worker})
	if err != nil {
		t.Fatalf("drive failed: %v (dispatched=%v)", err, dispatched)
	}
	if result.Outcome != DriverComplete {
		t.Fatalf("outcome = %s, want complete (dispatched=%v)", result.Outcome, dispatched)
	}
	for _, want := range []string{"requirements.md", "design.md", "tasks.md"} {
		if !authored[want] {
			t.Fatalf("planning artifact %s was never authored (dispatched=%v)", want, dispatched)
		}
	}
	// The execution task must have been handed to the worker too.
	sawExec := false
	for _, d := range dispatched {
		if strings.HasPrefix(d, "dispatch:T1") {
			sawExec = true
		}
	}
	if !sawExec {
		t.Fatalf("execution task never dispatched (dispatched=%v)", dispatched)
	}
}

func artifactFixture(artifact string) string {
	switch artifact {
	case "requirements.md":
		return passingRequirements
	case "design.md":
		return passingDesign
	case "tasks.md":
		return passingTasks
	}
	return ""
}
