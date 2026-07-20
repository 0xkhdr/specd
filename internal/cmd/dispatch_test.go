package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

// TestDispatchPhase pins spec 03 R2: an execution verb invoked against a spec
// still in an early phase fails closed (exit 2 via ErrUsage) naming the verb,
// the current phase, and the allowed phases — before any side effect, so
// state.json is untouched.
func TestDispatchPhase(t *testing.T) {
	root := t.TempDir()
	statePath := core.StatePath(root, "demo")
	if err := os.MkdirAll(strings.TrimSuffix(statePath, "/state.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	// InitialState is status=requirements ⇒ phase=perceive, which verify
	// (allowed: plan/execute/verify) must reject.
	if err := core.SaveState(statePath, core.InitialState("demo")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	err = Run(root, "verify", []string{"demo", "T1"}, nil)
	if err == nil {
		t.Fatal("verify in perceive phase succeeded, want fail-closed rejection")
	}
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("error does not wrap ErrUsage (exit 2): %v", err)
	}
	for _, want := range []string{"verify", "perceive", "plan"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("rejection message %q missing %q", err.Error(), want)
		}
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("state.json mutated on a rejected out-of-phase dispatch")
	}
}

func TestProductionTaskAuthorityUsesLifecycleProfileAndMissionScope(t *testing.T) {
	root := newDemoSpec(t)
	writeTasks(t, root, "demo", "| T1 | craftsman | spec.md | - | true | ok |")
	if err := os.WriteFile(filepath.Join(root, "project.yml"), []byte("profile: production\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	state.Status, state.Phase = core.StatusTasks, core.PhaseForStatus(core.StatusTasks)
	if err := core.SaveStateCAS(core.StatePath(root, "demo"), state.Revision, state); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "complete-task", []string{"demo", "T1"}, nil); err == nil || !strings.Contains(err.Error(), "requires AuthorityV1") {
		t.Fatalf("raw production completion error = %v", err)
	}

	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "specd@example.test")
	runGit(t, root, "config", "user.name", "specd")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "fixture")
	now := time.Now().UTC()
	head := gitHead(root)
	task := core.TaskRow{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"spec.md"}}
	authority, err := core.BuildAuthority(task, "controller", "worker", "demo", string(core.PhaseForStatus(core.StatusTasks)), head, "policy", "required", now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	mission := orchestration.MissionV1{MissionID: "mission-1", SpecSlug: "demo", TaskID: "T1", Role: "craftsman", SubjectHead: head, DeclaredFiles: []string{"spec.md"}}
	lease := orchestration.Lease{MissionID: mission.MissionID, TaskID: "T1", Authority: authority}
	sessionPath := filepath.Join(core.SpecdDir(root), "specs", "demo", "session.json")
	if err := orchestration.SaveSessionCAS(root, sessionPath, 0, orchestration.Session{Missions: []orchestration.MissionV1{mission}, Leases: []orchestration.Lease{lease}}); err != nil {
		t.Fatal(err)
	}
	err = RunAuthorized(root, "complete-task", []string{"demo", "T1"}, nil, authority, nil, now)
	if err == nil || strings.Contains(err.Error(), "authority denied") || !strings.Contains(err.Error(), "requires passing evidence") {
		t.Fatalf("authorized route did not reach evidence gate: %v", err)
	}
	if _, err := mcpExecutor(root)("complete-task", []string{"demo", "T1"}, nil, &authority, now); err == nil || strings.Contains(err.Error(), "requires AuthorityV1") || !strings.Contains(err.Error(), "requires passing evidence") {
		t.Fatalf("MCP executor dropped authority packet: %v", err)
	}
	outside := filepath.Join(root, "outside.go")
	if err := os.WriteFile(outside, []byte("package outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RunAuthorized(root, "complete-task", []string{"demo", "T1"}, nil, authority, nil, now); err == nil || !strings.Contains(err.Error(), "PATH_WRITE_DENIED: outside.go") {
		t.Fatalf("mission-derived scope accepted outside path: %v", err)
	}
	if err := os.Remove(outside); err != nil {
		t.Fatal(err)
	}

	for name, mutate := range map[string]func(*core.AuthorityV1){
		"stale": func(a *core.AuthorityV1) { a.ExpiresAt = now },
		"spec":  func(a *core.AuthorityV1) { a.SpecID = "other" },
		"task":  func(a *core.AuthorityV1) { a.TaskID = "T2" },
		"role":  func(a *core.AuthorityV1) { a.Role, a.Mode = "validator", "read_only" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := authority
			mutate(&bad)
			if name != "stale" {
				bad.Digest = ""
				if err := core.FinalizeAuthority(&bad); err != nil {
					t.Fatal(err)
				}
			}
			err := RunAuthorized(root, "complete-task", []string{"demo", "T1"}, nil, bad, nil, now)
			refusal, ok := core.AsRefusal(err)
			if err == nil || !ok || refusal.Code != "AUTHORITY_DENIED" {
				t.Fatalf("%s mismatch accepted: %v", name, err)
			}
		})
	}
}

func TestDispatchPausesOnAmendmentWithoutRewind(t *testing.T) {
	root := newDemoSpec(t)
	for range 2 {
		if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
			t.Fatalf("approve next: %v", err)
		}
	}
	before, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "midreq", []string{"demo"}, map[string]string{"text": "change R1", "scope": "R1"}); err != nil {
		t.Fatalf("midreq: %v", err)
	}
	if err := Run(root, "next", []string{"demo"}, nil); err == nil || !strings.Contains(err.Error(), "dispatch paused") {
		t.Fatalf("stale dispatch accepted: %v", err)
	}
	after, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != before.Status {
		t.Fatalf("amendment rewound status from %q to %q", before.Status, after.Status)
	}
	if _, ok := after.Records["amendment:0"]; !ok {
		t.Fatal("midreq did not append amendment record")
	}
}

func TestDispatchAuthorityDeniesReadOnlyWrite(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	task := core.TaskRow{ID: "T1", Role: "validator", DeclaredFiles: []string{"a.go"}}
	a, err := core.BuildAuthority(task, "controller", "w", "demo", "execute", "abc", "policy", "required", now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	a.AllowedTools = append(a.AllowedTools, core.ToolAuthority{ID: "complete-task"})
	a.Digest = ""
	if err := core.FinalizeAuthority(&a); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	err = RunAuthorized(root, "complete-task", []string{"demo", "T1"}, nil, a, nil, now)
	if err == nil || (!strings.Contains(err.Error(), "ROLE_WRITE_DENIED") && !strings.Contains(err.Error(), "authority denied")) {
		t.Fatalf("err=%v", err)
	}
	raw, readErr := os.ReadFile(filepath.Join(root, ".specd/specs/demo/authority-denials.jsonl"))
	if readErr != nil || !strings.Contains(string(raw), `"tool_id":"complete-task"`) {
		t.Fatalf("denial record=%q err=%v", raw, readErr)
	}
}

// TestDispatchPhaseAllowed confirms an in-phase spec passes the phase gate; any
// resulting error is a downstream handler error, never the ErrUsage gate.
func TestDispatchPhaseAllowed(t *testing.T) {
	root := t.TempDir()
	statePath := core.StatePath(root, "demo")
	if err := os.MkdirAll(strings.TrimSuffix(statePath, "/state.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := core.InitialState("demo")
	state.Status = core.StatusTasks
	state.Phase = core.PhaseForStatus(core.StatusTasks)
	if err := core.SaveState(statePath, state); err != nil {
		t.Fatal(err)
	}
	err := Run(root, "verify", []string{"demo", "T1"}, nil)
	if errors.Is(err, ErrUsage) {
		t.Fatalf("in-phase verify rejected by phase gate: %v", err)
	}
}

// TestDispatchHelpPalette pins spec R4.1: --help on a multi-operation verb
// prints the verb's palette operations (usage, flags, examples) and exits 0
// instead of failing closed.
func TestDispatchHelpPalette(t *testing.T) {
	cases := map[string]string{
		"brain":     "brain.start",
		"eval":      "eval.import",
		"exception": "exception.approve",
		"agents":    "agents.doctor",
	}
	for name, operationID := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			out, err := captureStdout(t, func() error {
				return Run(root, name, nil, map[string]string{"help": "true"})
			})
			if err != nil {
				t.Fatalf("%s --help: %v", name, err)
			}
			for _, want := range []string{"usage:", "operations:", operationID} {
				if !strings.Contains(out, want) {
					t.Errorf("%s --help output %q missing %q", name, out, want)
				}
			}
		})
	}
}

// TestDispatchEmptySubcommandPalette pins spec R4.1 for the empty-subcommand
// form: a multi-operation verb with no bare operation prints its palette and
// exits 0 where it previously failed closed.
func TestDispatchEmptySubcommandPalette(t *testing.T) {
	for _, name := range []string{"brain", "eval", "exception"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			out, err := captureStdout(t, func() error {
				return Run(root, name, nil, nil)
			})
			if err != nil {
				t.Fatalf("bare %s: %v", name, err)
			}
			if !strings.Contains(out, "operations:") || !strings.Contains(out, name+".") {
				t.Errorf("bare %s output %q is not the operation palette", name, out)
			}
		})
	}
}

// TestDispatchUnknownSubcommandStillFailsClosed pins that the help surface is
// additive: an unknown subcommand on a multi-operation verb still exits 2.
func TestDispatchUnknownSubcommandStillFailsClosed(t *testing.T) {
	for _, name := range []string{"brain", "eval", "exception", "agents"} {
		t.Run(name, func(t *testing.T) {
			err := Run(t.TempDir(), name, []string{"bogus"}, nil)
			if !errors.Is(err, ErrUsage) {
				t.Fatalf("%s bogus err = %v, want ErrUsage", name, err)
			}
		})
	}
}

// TestDispatchAgentsInspectAlias pins spec R4.2: `specd agents inspect`
// aliases bare `specd agents`, matching the palette id agents.inspect.
func TestDispatchAgentsInspectAlias(t *testing.T) {
	root := t.TempDir()
	if err := core.WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	bare, err := captureStdout(t, func() error {
		return Run(root, "agents", nil, map[string]string{"json": "true"})
	})
	if err != nil {
		t.Fatalf("bare agents: %v", err)
	}
	aliased, err := captureStdout(t, func() error {
		return Run(root, "agents", []string{"inspect"}, map[string]string{"json": "true"})
	})
	if err != nil {
		t.Fatalf("agents inspect: %v", err)
	}
	if bare != aliased {
		t.Fatalf("agents inspect output %q differs from bare agents %q", aliased, bare)
	}
}

// TestFlagEnum pins spec 03 R3: an enum-declared flag given an out-of-enum
// value fails closed (exit 2) naming the flag and allowed values.
func TestFlagEnum(t *testing.T) {
	root := t.TempDir()
	err := Run(root, "memory", []string{"demo", "add"}, map[string]string{"criticality": "bogus"})
	if err == nil {
		t.Fatal("out-of-enum flag value accepted")
	}
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("enum rejection does not wrap ErrUsage: %v", err)
	}
	if !strings.Contains(err.Error(), "criticality") {
		t.Errorf("message %q does not name the flag", err.Error())
	}

	// A valid enum value passes the enum gate (handler may still fail for
	// unrelated reasons, but not via ErrUsage).
	err = Run(root, "memory", []string{"demo", "add"}, map[string]string{"criticality": "critical"})
	if errors.Is(err, ErrUsage) {
		t.Fatalf("valid enum value rejected by enum gate: %v", err)
	}
}
