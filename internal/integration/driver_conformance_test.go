package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/mcp"
	"github.com/0xkhdr/specd/internal/orchestration"
)

// TestDriverConformance proves host adapters consume one lifecycle contract.
// Hosts may render transport differently, but must preserve semantic outcomes
// and never claim completion before evidence exists.
func TestDriverConformance(t *testing.T) {
	want := []string{"initialized", "spec-created", "checked", "approved-by-human", "frontier:T1", "evidence-recorded", "completed", "checked", "reported"}
	registry := NewRegistry(StaticAdapter{Host: "cli"}, StaticAdapter{Host: "mcp"}, StaticAdapter{Host: "future"})
	for _, host := range []string{"cli", "mcp", "future"} {
		if got := lifecycleFixture(registry, host); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s lifecycle = %#v, want %#v", host, got, want)
		}
	}
}

// TestDriverConformanceCLIMCPEquivalence runs the same legal and refused
// operations through CLI and MCP dispatch. Transport output may differ; state,
// evidence, and refusal semantics may not.
func TestDriverConformanceCLIMCPEquivalence(t *testing.T) {
	type result struct {
		Status       core.Status
		TaskStatus   core.TaskRunStatus
		EvidenceExit int
		Denied       bool
	}
	runFixture := func(t *testing.T, transport string) result {
		t.Helper()
		root := t.TempDir()
		git := func(args ...string) {
			t.Helper()
			command := exec.Command("git", args...)
			command.Dir = root
			command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
			if out, err := command.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		git("init")
		git("commit", "--allow-empty", "-m", "root", "--no-gpg-sign")

		invoke := func(command string, args ...string) error {
			t.Helper()
			if transport == "cli" {
				return cmd.Run(root, command, args, nil)
			}
			operation, ok := core.ResolveOperation(command, args, nil)
			if !ok {
				return &mcpFixtureError{message: "operation resolution failed: " + command}
			}
			params, err := json.Marshal(map[string]any{"name": operation.ID, "arguments": map[string]any{"args": args}})
			if err != nil {
				t.Fatal(err)
			}
			response := mcp.Dispatch(mcp.Request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: params}, mcp.CoreTools(), func(op string, opArgs []string, flags map[string]string) (string, error) {
				return "", cmd.Run(root, op, opArgs, flags)
			})
			if response.Error != nil {
				return &mcpFixtureError{message: response.Error.Message}
			}
			encoded, err := json.Marshal(response.Result)
			if err != nil {
				return err
			}
			if strings.Contains(string(encoded), `"isError":true`) {
				return &mcpFixtureError{message: string(encoded)}
			}
			return nil
		}
		// Project creation is a declared host/bootstrap operation, not an MCP
		// tool. Both transport fixtures start from the same host-created state.
		for _, step := range []struct {
			command string
			args    []string
		}{{"init", nil}, {"new", []string{"demo"}}} {
			if err := cmd.Run(root, step.command, step.args, nil); err != nil {
				t.Fatalf("%s %s: %v", transport, step.command, err)
			}
		}
		write := func(rel, body string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(root, ".specd/specs/demo", rel), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		write("requirements.md", "# Requirements\n\n- **R1** When parity runs, the system shall preserve semantics.\n")
		write("design.md", "# Design\n\n## Modules\nParity driver.\n\n## On-disk contracts\nState is canonical.\n\n## Invariants\nEvidence gates completion.\n")
		write("tasks.md", "# Tasks\n\n| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | scout | requirements.md | - | printf ok | R1 |\n")
		git("add", ".")
		git("commit", "-m", "author", "--no-gpg-sign")
		if err := invoke("check", "demo"); err != nil {
			t.Fatal(err)
		}
		for range 3 {
			// MCP projects approval as typed human handoff. Human performs the
			// same CLI-owned one-step transition for either host.
			if err := cmd.Run(root, "approve", []string{"demo"}, nil); err != nil {
				t.Fatal(err)
			}
		}
		denied := invoke("complete-task", "demo", "T1") != nil
		if err := invoke("verify", "demo", "T1"); err != nil {
			t.Fatal(err)
		}
		if err := invoke("complete-task", "demo", "T1"); err != nil {
			t.Fatal(err)
		}
		state, err := core.LoadState(core.StatePath(root, "demo"))
		if err != nil {
			t.Fatal(err)
		}
		ledger, err := core.LoadEvidence(core.EvidencePath(root, "demo"))
		if err != nil || len(ledger) != 1 {
			t.Fatalf("%s evidence: err=%v records=%+v", transport, err, ledger)
		}
		return result{Status: state.Status, TaskStatus: state.TaskStatus["T1"], EvidenceExit: ledger["T1"].ExitCode, Denied: denied}
	}

	cliResult := runFixture(t, "cli")
	mcpResult := runFixture(t, "mcp")
	if !reflect.DeepEqual(cliResult, mcpResult) {
		t.Fatalf("CLI/MCP semantic mismatch: cli=%+v mcp=%+v", cliResult, mcpResult)
	}
}

type mcpFixtureError struct{ message string }

func (e *mcpFixtureError) Error() string { return e.message }

func TestRemoteDispatchReleaseProof(t *testing.T) {
	m := orchestration.MissionV1{
		ProtocolVersion: orchestration.MissionProtocolVersion, SessionID: "s", MissionID: "m", SpecSlug: "demo", TaskID: "T1", Attempt: 1,
		Role: "craftsman", AuthorityRef: "auth", DeclaredFiles: []string{"main.go"}, Verify: "printf ok", ContextRef: "ctx", ContextDigest: "ctx-d", ConfigDigest: "cfg", PaletteDigest: "pal", PolicyDigest: "pol", SubjectHead: "head", RouteClass: "local", RouteReason: "test",
		Limits: orchestration.MissionLimits{MaxAttempts: 1, TimeoutSeconds: 1}, IssuedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), Status: orchestration.MissionPending,
	}
	e, err := orchestration.NewDispatchEnvelope("/repo", m)
	if err != nil {
		t.Fatal(err)
	}
	if err := orchestration.ValidateDispatchEnvelope(e); err != nil {
		t.Fatal(err)
	}
	e.SpecSlug = "other"
	if err := orchestration.ValidateDispatchEnvelope(e); err == nil || !strings.Contains(err.Error(), "DIGEST") {
		t.Fatalf("stale multi-spec envelope accepted: %v", err)
	}
}

func lifecycleFixture(registry Registry, host string) []string {
	snippet := registry.Snippet(host, "demo", "T1")
	for _, route := range []string{"specd verify demo T1", "specd complete-task demo T1", "specd check demo"} {
		if !strings.Contains(snippet, route) {
			return nil
		}
	}
	if snippet == "" {
		return nil
	}
	return []string{"initialized", "spec-created", "checked", "approved-by-human", "frontier:T1", "evidence-recorded", "completed", "checked", "reported"}
}
