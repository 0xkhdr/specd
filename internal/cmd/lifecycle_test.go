package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

// newDemoSpec initializes a project and a "demo" spec whose single task has a
// trivially-passing verify command, then returns the root.
func newDemoSpec(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := Run(root, "init", nil, nil); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := Run(root, "new", []string{"demo"}, nil); err != nil {
		t.Fatalf("new: %v", err)
	}
	writeTasks(t, root, "demo", "| T1 | craftsman | spec.md | - | true | ok |")
	authorDemoSpec(t, root, "demo")
	return root
}

func writeTasks(t *testing.T, root, slug, row string) {
	t.Helper()
	path := filepath.Join(core.SpecdDir(root), "specs", slug, "tasks.md")
	body := "# Tasks\n\n| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n" + row + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := fn()
	w.Close()
	os.Stdout = orig
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, readErr := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if readErr != nil {
			break
		}
	}
	return string(buf), err
}

func TestApproveGatesE2E(t *testing.T) {
	root := newDemoSpec(t)
	statePath := core.StatePath(root, "demo")

	// Green readiness: approve ratchets to design and records approval.
	if err := Run(root, "approve", []string{"demo", "design"}, nil); err != nil {
		t.Fatalf("approve (green): %v", err)
	}
	state, err := core.LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != core.StatusDesign {
		t.Fatalf("status = %q, want design", state.Status)
	}
	if _, ok := state.Records["approval:design"]; !ok {
		t.Fatal("approval record not written")
	}

	// Red readiness: a dangling dependency makes the dag gate error; approve
	// must refuse and leave state unchanged.
	writeTasks(t, root, "demo", "| T1 | craftsman | spec.md | T9 | true | ok |")
	before, _ := core.LoadState(statePath)
	if err := Run(root, "approve", []string{"demo", "tasks"}, nil); err == nil {
		t.Fatal("approve should refuse on red readiness gates")
	}
	after, _ := core.LoadState(statePath)
	if after.Status != before.Status || after.Revision != before.Revision {
		t.Fatalf("state mutated on refused approve: %+v -> %+v", before, after)
	}
}

func TestMidreqDecisionAppend(t *testing.T) {
	root := newDemoSpec(t)
	statePath := core.StatePath(root, "demo")
	before, _ := core.LoadState(statePath)

	if err := Run(root, "midreq", []string{"demo"}, map[string]string{"text": "raise budget"}); err != nil {
		t.Fatalf("midreq: %v", err)
	}
	if err := Run(root, "decision", []string{"demo"}, map[string]string{"text": "use CAS", "scope": "state"}); err != nil {
		t.Fatalf("decision: %v", err)
	}
	state, _ := core.LoadState(statePath)
	if _, ok := state.Records["midreq:0"]; !ok {
		t.Fatal("midreq record missing")
	}
	var rec core.Record
	if err := json.Unmarshal(state.Records["decision:0"], &rec); err != nil {
		t.Fatalf("decision record: %v", err)
	}
	if rec.Text != "use CAS" || rec.Scope != "state" {
		t.Fatalf("decision content not round-tripped: %+v", rec)
	}
	if rec.Timestamp == "" || rec.Actor == "" || rec.GitHead == "" {
		t.Fatalf("decision record not stamped: %+v", rec)
	}
	if state.Status != before.Status {
		t.Fatalf("unrelated field mutated: status %q -> %q", before.Status, state.Status)
	}
	if state.Revision <= before.Revision {
		t.Fatal("revision not advanced via CAS")
	}
}

// TestDecisionRequiresText asserts decision/midreq without --text is a usage
// error and writes nothing — a recorded decision that records no content
// records nothing (R3.1).
func TestDecisionRequiresText(t *testing.T) {
	root := newDemoSpec(t)
	statePath := core.StatePath(root, "demo")

	for _, verb := range []string{"decision", "midreq"} {
		if err := Run(root, verb, []string{"demo"}, nil); err == nil {
			t.Fatalf("%s without --text: want usage error, got nil", verb)
		}
		if err := Run(root, verb, []string{"demo"}, map[string]string{"text": "  "}); err == nil {
			t.Fatalf("%s with blank --text: want usage error, got nil", verb)
		}
	}
	state, _ := core.LoadState(statePath)
	if len(state.Records) != 0 {
		t.Fatalf("failed decision/midreq wrote records: %v", state.Records)
	}
}

func TestStatusNextVerifyOnRealSpec(t *testing.T) {
	root := newDemoSpec(t)
	for _, verb := range []struct {
		name string
		args []string
	}{
		{"status", []string{"demo"}},
		{"report", []string{"demo"}},
	} {
		if _, err := captureStdout(t, func() error { return Run(root, verb.name, verb.args, nil) }); err != nil {
			t.Fatalf("%s: %v", verb.name, err)
		}
	}
	if err := Run(root, "approve", []string{"demo", "requirements"}, nil); err != nil {
		t.Fatalf("approve requirements: %v", err)
	}
	if err := Run(root, "approve", []string{"demo", "design"}, nil); err != nil {
		t.Fatalf("approve design: %v", err)
	}
	// context is an execution verb: gated out of the requirements (perceive)
	// phase, allowed once requirements are approved (spec 03 R2).
	if _, err := captureStdout(t, func() error { return Run(root, "context", []string{"demo", "T1"}, nil) }); err != nil {
		t.Fatalf("context: %v", err)
	}
	if _, err := captureStdout(t, func() error { return Run(root, "next", []string{"demo"}, nil) }); err != nil {
		t.Fatalf("next: %v", err)
	}
	if _, err := captureStdout(t, func() error { return Run(root, "verify", []string{"demo", "T1"}, nil) }); err != nil {
		t.Fatalf("verify: %v", err)
	}

	// verify wrote an evidence record for T1.
	evidence, err := core.LoadEvidence(core.EvidencePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := evidence["T1"]; !ok {
		t.Fatal("verify did not record evidence for T1")
	}
}

func TestTaskShowsDetails(t *testing.T) {
	root := newDemoSpec(t)
	out, err := captureStdout(t, func() error { return Run(root, "task", []string{"T1"}, nil) })
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	for _, want := range []string{"T1", "craftsman", "true"} {
		if !strings.Contains(out, want) {
			t.Fatalf("task output missing %q:\n%s", want, out)
		}
	}
}

func TestBrainDispatchesFrontierViaCLI(t *testing.T) {
	root := newDemoSpec(t)
	sessionPath := filepath.Join(core.SpecdDir(root), "specs", "demo", "session.json")
	acpPath := filepath.Join(core.SpecdDir(root), "specs", "demo", "acp.jsonl")

	// brain is an execution verb: advance past the requirements (perceive)
	// phase before stepping the controller (spec 03 R2).
	if err := Run(root, "approve", []string{"demo", "requirements"}, nil); err != nil {
		t.Fatalf("approve requirements: %v", err)
	}
	if err := Run(root, "approve", []string{"demo", "design"}, nil); err != nil {
		t.Fatalf("approve design: %v", err)
	}

	// Fail-closed: no authority => wait, no lease, no evidence.
	if _, err := captureStdout(t, func() error { return Run(root, "brain", []string{"step", "demo"}, nil) }); err != nil {
		t.Fatalf("brain step (no authority): %v", err)
	}
	session, _ := orchestration.LoadSession(sessionPath)
	if len(session.Leases) != 0 {
		t.Fatal("dispatched without authority (should fail closed)")
	}
	if events, _ := orchestration.ReadACP(acpPath); len(events) != 0 {
		t.Fatal("wrote evidence without authority")
	}

	// With authority: dispatch frontier, write session lease + ACP evidence.
	flags := map[string]string{"authority": "true"}
	if _, err := captureStdout(t, func() error { return Run(root, "brain", []string{"step", "demo"}, flags) }); err != nil {
		t.Fatalf("brain step (authority): %v", err)
	}
	session, _ = orchestration.LoadSession(sessionPath)
	if len(session.Leases) != 1 || session.Leases[0].TaskID != "T1" {
		t.Fatalf("expected one lease on T1, got %+v", session.Leases)
	}
	events, _ := orchestration.ReadACP(acpPath)
	if len(events) != 1 || events[0].Kind != "dispatch" || events[0].TaskID != "T1" {
		t.Fatalf("expected one dispatch event for T1, got %+v", events)
	}
}
