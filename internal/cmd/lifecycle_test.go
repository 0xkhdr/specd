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
	writeTasks(t, root, "demo", "| T1 | scout | spec.md | - | true | ok |")
	authorDemoSpec(t, root, "demo")
	return root
}

func TestLifecycleContextMissingDesignFailsClosed(t *testing.T) {
	root := newDemoSpec(t)
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatal(err)
	}
	design := filepath.Join(core.SpecdDir(root), "specs", "demo", "design.md")
	if err := os.Remove(design); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(t, func() error { return Run(root, "context", []string{"demo", "T1"}, map[string]string{"json": ""}) }); err == nil || !strings.Contains(err.Error(), "design.md") {
		t.Fatalf("missing design must fail closed with source identity: %v", err)
	}
}

func TestStubProductionAuthoringContract(t *testing.T) {
	requirements := requirementsStub("demo")
	for _, want := range []string{"R1.1", "owner:", "priority:", "risk:", "EARS", "Edge and failure", "Non-goals"} {
		if !strings.Contains(requirements, want) {
			t.Errorf("requirements stub missing %q", want)
		}
	}
	design := designStub("demo")
	for _, want := range []string{"references:", "## Boundaries", "## Interfaces", "## Invariants", "## Failure", "## Integration", "## Alternatives", "disposition:", "owner:", "## Verification", "## Deployment", "## Rollback"} {
		if !strings.Contains(design, want) {
			t.Errorf("design stub missing %q", want)
		}
	}
	tasks := tasksStub("demo")
	for _, want := range []string{"refs", "kind", "risk", "complexity", "capabilities", "context", "evidence", "checks", "may be omitted"} {
		if !strings.Contains(tasks, want) {
			t.Errorf("tasks stub missing %q", want)
		}
	}
	parsed, err := core.ParseTasksMd([]byte(tasks))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Tasks) != 0 {
		t.Fatalf("fresh task stub contains fake runnable tasks: %+v", parsed.Tasks)
	}
}

func TestNewDesignScaffoldParsesReferences(t *testing.T) {
	root := t.TempDir()
	if _, err := captureStdout(t, func() error { return runNew(root, []string{"demo"}, nil) }); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(core.SpecdDir(root), "specs", "demo", "design.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := core.ParseDesign(raw).Refs; len(got) != 2 || got[0] != "R1" || got[1] != "R1.1" {
		t.Fatalf("generated design references = %v, want [R1 R1.1]", got)
	}
}

func TestApproveModeOperationIsSeparate(t *testing.T) {
	root := newDemoSpec(t)
	statePath := core.StatePath(root, "demo")
	before, err := core.LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}

	if err := Run(root, "mode", []string{"demo", "orchestrated"}, nil); err == nil || !strings.Contains(err.Error(), "orchestration.enabled") {
		t.Fatalf("disabled transition error = %v", err)
	}
	unchanged, err := core.LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Revision != before.Revision || unchanged.Mode != before.Mode {
		t.Fatalf("refused transition mutated state: before=%+v after=%+v", before, unchanged)
	}

	if err := os.WriteFile(filepath.Join(root, "project.yml"), []byte("orchestration:\n  enabled: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "mode", []string{"demo", "orchestrated"}, nil); err != nil {
		t.Fatalf("mode orchestrated: %v", err)
	}
	after, err := core.LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if after.Mode != core.ModeOrchestrated || after.Revision != before.Revision+1 {
		t.Fatalf("transition = mode %q revision %d, want orchestrated/%d", after.Mode, after.Revision, before.Revision+1)
	}
	var approval core.Record
	if err := json.Unmarshal(after.Records["approval:orchestrated"], &approval); err != nil {
		t.Fatalf("approval record: %v", err)
	}
	if approval.Gate != "orchestrated" || approval.ApprovedRevision != before.Revision {
		t.Fatalf("approval = %+v", approval)
	}
}

func writeTasks(t *testing.T, root, slug, row string) {
	t.Helper()
	path := filepath.Join(core.SpecdDir(root), "specs", slug, "tasks.md")
	body := "# Tasks\n\n| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n" + row + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it
// wrote. The reader is drained by a goroutine started *before* fn runs: a pipe
// buffers ~64KiB, and reading only after fn returns deadlocks the moment a verb
// prints more than that (`help --json` is already past it). Same shape as
// captureRunOutput in mcp_exec.go, for the same reason.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	err := fn()
	w.Close()
	os.Stdout = orig
	out := <-done
	r.Close()
	return out, err
}

func TestApproveGatesE2E(t *testing.T) {
	root := newDemoSpec(t)
	statePath := core.StatePath(root, "demo")

	// Green readiness: caller names only the spec; approve computes the exact
	// successor, ratchets to design, and records the actual target.
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatalf("approve (green): %v", err)
	}
	state, err := core.LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != core.StatusDesign {
		t.Fatalf("status = %q, want design", state.Status)
	}
	if _, ok := state.Records["approval:requirements"]; !ok {
		t.Fatal("approval record not written")
	}
	var approval core.Record
	if err := json.Unmarshal(state.Records["approval:requirements"], &approval); err != nil {
		t.Fatal(err)
	}
	if approval.Gate != "requirements" || approval.Text != "requirements → design" {
		t.Fatalf("approval transition = %+v", approval)
	}

	// Red readiness: a dangling dependency makes the dag gate error; approve
	// must refuse and leave state unchanged.
	writeTasks(t, root, "demo", "| T1 | scout | spec.md | T9 | true | ok |")
	before, _ := core.LoadState(statePath)
	if err := Run(root, "approve", []string{"demo"}, nil); err == nil {
		t.Fatal("approve should refuse on red readiness gates")
	}
	after, _ := core.LoadState(statePath)
	if after.Status != before.Status || after.Revision != before.Revision {
		t.Fatalf("state mutated on refused approve: %+v -> %+v", before, after)
	}
}

func TestCommandApprovalOperationsAreSeparate(t *testing.T) {
	approve, ok := core.CommandByName("approve")
	if !ok || approve.Usage != "specd approve <spec>" {
		t.Fatalf("approve metadata = %+v, found=%v", approve, ok)
	}
	for _, name := range []string{"mode", "exception"} {
		command, ok := core.CommandByName(name)
		if !ok || !command.HumanOnly {
			t.Errorf("%s metadata = %+v, found=%v; want separate human-only operation", name, command, ok)
		}
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

// TestSpikeRecordsWithoutBypass pins spec 01 R7.3: `specd spike` records a
// bounded learning record and authorizes nothing. A spike moves no lifecycle
// status, completes no task, and adds no approval record; the bound (question,
// scope, future expiry) is mandatory, and a rejected spike writes nothing.
func TestSpikeRecordsWithoutBypass(t *testing.T) {
	root := newDemoSpec(t)
	statePath := core.StatePath(root, "demo")
	before, _ := core.LoadState(statePath)

	flags := map[string]string{
		"question": "is webhook retry idempotent?",
		"scope":    "demo/webhook",
		"expiry":   "2099-01-01T00:00:00Z",
		"output":   "spike-notes.md",
	}
	if err := Run(root, "spike", []string{"demo"}, flags); err != nil {
		t.Fatalf("spike: %v", err)
	}

	state, _ := core.LoadState(statePath)
	spikes, err := state.Spikes()
	if err != nil {
		t.Fatalf("Spikes: %v", err)
	}
	if len(spikes) != 1 || spikes[0].Question != flags["question"] || spikes[0].OutputRef != "spike-notes.md" {
		t.Fatalf("spike not recorded: %+v", spikes)
	}
	if spikes[0].Timestamp == "" || spikes[0].Actor == "" || spikes[0].GitHead == "" {
		t.Fatalf("spike not stamped: %+v", spikes[0])
	}

	// No bypass: the spike neither advanced the lifecycle nor completed a task
	// nor approved anything. Only the CAS revision moved.
	if state.Status != before.Status || state.Phase != before.Phase {
		t.Fatalf("spike moved lifecycle: %q/%q -> %q/%q", before.Status, before.Phase, state.Status, state.Phase)
	}
	if len(state.TaskStatus) != 0 {
		t.Fatalf("spike completed a task: %+v", state.TaskStatus)
	}
	for key := range state.Records {
		if strings.HasPrefix(key, "approval:") {
			t.Fatalf("spike wrote an approval record %q", key)
		}
	}
	if state.Revision != before.Revision+1 {
		t.Fatalf("revision = %d, want %d (single CAS write)", state.Revision, before.Revision+1)
	}

	// The bound is mandatory: a spike missing a required field is a usage error
	// and writes nothing.
	revBefore := state.Revision
	if err := Run(root, "spike", []string{"demo"}, map[string]string{"scope": "s", "expiry": "2099-01-01T00:00:00Z"}); err == nil {
		t.Fatal("spike without --question: want error, got nil")
	}
	after, _ := core.LoadState(statePath)
	if after.Revision != revBefore {
		t.Fatal("rejected spike mutated state")
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
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatalf("approve next: %v", err)
	}
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
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

func TestTaskCompleteNarrowRouteKeepsVerifySeparate(t *testing.T) {
	root := newDemoSpec(t)
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "specd@example.test")
	runGit(t, root, "config", "user.name", "specd")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "fixture")
	for range 2 {
		if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
			t.Fatalf("approve: %v", err)
		}
	}
	out, err := captureStdout(t, func() error { return Run(root, "verify", []string{"demo", "T1"}, nil) })
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(out, "evidence recorded") || !strings.Contains(out, "task not complete") {
		t.Fatalf("verify output did not distinguish evidence from completion: %q", out)
	}
	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if state.TaskStatus["T1"] == core.TaskComplete {
		t.Fatal("verify completed task")
	}
	if err := Run(root, "complete-task", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("complete-task: %v", err)
	}
	state, err = core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if state.TaskStatus["T1"] != core.TaskComplete {
		t.Fatalf("task status = %q", state.TaskStatus["T1"])
	}
	if err := Run(root, "task", []string{"complete", "demo", "T1"}, nil); err == nil {
		t.Fatal("broad task command still exposes completion")
	}
}

func TestTaskCompleteRollsBackStateWhenTasksWriteFails(t *testing.T) {
	root := newDemoSpec(t)
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "specd@example.test")
	runGit(t, root, "config", "user.name", "specd")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "fixture")
	for range 2 {
		if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
		t.Fatal(err)
	}
	tasksPath := filepath.Join(core.SpecdDir(root), "specs", "demo", "tasks.md")
	beforeTasks, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	original := taskCompleteAtomicWrite
	taskCompleteAtomicWrite = func(string, string) error { return errors.New("injected tasks write failure") }
	t.Cleanup(func() { taskCompleteAtomicWrite = original })

	err = Run(root, "complete-task", []string{"demo", "T1"}, nil)
	if err == nil || !strings.Contains(err.Error(), "injected tasks write failure") {
		t.Fatalf("completion error = %v", err)
	}
	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if state.TaskStatus["T1"] == core.TaskComplete {
		t.Fatalf("failed completion left state complete: %+v", state.TaskStatus)
	}
	afterTasks, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterTasks) != string(beforeTasks) {
		t.Fatal("failed completion changed tasks.md")
	}
}

func TestTaskCompleteEnforcesQualityEvidence(t *testing.T) {
	root := newDemoSpec(t)
	dir := filepath.Join(core.SpecdDir(root), "specs", "demo")
	tasks := "# Tasks\n\n| id | role | files | depends-on | verify | acceptance | evidence | checks |\n|---|---|---|---|---|---|---|---|\n| T1 | craftsman | spec.md | - | go test ./... | ok | output_eval/rubric-v1 | rubric-v1 |\n"
	if err := os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "specd@example.test")
	runGit(t, root, "config", "user.name", "specd")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "fixture")
	for range 2 {
		if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
			t.Fatalf("approve: %v", err)
		}
	}
	head := gitHead(root)
	if err := core.AppendEvidence(core.EvidencePath(root, "demo"), core.EvidenceRecord{TaskID: "T1", Command: "go test ./...", ExitCode: 0, GitHead: head}); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "complete-task", []string{"demo", "T1"}, nil); err == nil || !strings.Contains(err.Error(), "EVIDENCE_MISSING") {
		t.Fatalf("missing quality evidence error = %v", err)
	}
	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if state.TaskStatus["T1"] == core.TaskComplete {
		t.Fatal("quality refusal completed task")
	}
}

func TestTaskShowsDetails(t *testing.T) {
	root := newDemoSpec(t)
	out, err := captureStdout(t, func() error { return Run(root, "task", []string{"T1"}, nil) })
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	for _, want := range []string{"T1", "scout", "true"} {
		if !strings.Contains(out, want) {
			t.Fatalf("task output missing %q:\n%s", want, out)
		}
	}
}

func TestBrainDispatchesFrontierViaCLI(t *testing.T) {
	root := newDemoSpec(t)
	if err := core.WriteScaffold(root, "pinky"); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(core.SpecdDir(root), "specs", "demo", "session.json")
	acpPath := filepath.Join(core.SpecdDir(root), "specs", "demo", "acp.jsonl")

	// brain is an execution verb: advance past the requirements (perceive)
	// phase before stepping the controller (spec 03 R2).
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatalf("approve next: %v", err)
	}
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
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

	// With authority: record pending mission + ACP evidence; no worker lease.
	flags := map[string]string{"authority": "true"}
	if _, err := captureStdout(t, func() error { return Run(root, "brain", []string{"step", "demo"}, flags) }); err != nil {
		t.Fatalf("brain step (authority): %v", err)
	}
	session, _ = orchestration.LoadSession(sessionPath)
	if len(session.Leases) != 0 || len(session.PendingMissions) != 1 || session.PendingMissions[0].TaskID != "T1" {
		t.Fatalf("expected pending mission without lease, got %+v", session)
	}
	events, _ := orchestration.ReadACP(acpPath)
	if len(events) != 1 || events[0].Kind != "dispatch" || events[0].TaskID != "T1" {
		t.Fatalf("expected one dispatch event for T1, got %+v", events)
	}
}

// TestDesignApprovalRefusesUnknownRef proves the live approve path refuses a
// design that traces to a requirement that does not exist (spec 01 R2.2) and,
// on a resolvable design, pins the design source digest into the approval record
// (spec 01 R2.1 "and digest").
func TestDesignApprovalRefusesUnknownRef(t *testing.T) {
	root := newDemoSpec(t)
	dir := filepath.Join(core.SpecdDir(root), "specs", "demo")
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("requirements.md", "# Requirements — demo\n\n### R1 — Thing\n\n- R1.1: When x runs, the system shall y.\n")
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatalf("approve requirements: %v", err)
	}
	// Design traces to a requirement that does not exist → refused.
	write("design.md", "# Design — demo\n\n## Modules\nThe module runs gates.\n\n- references: R9\n")
	if err := Run(root, "approve", []string{"demo"}, nil); err == nil {
		t.Fatal("approve design accepted an unknown requirement reference")
	}

	// Design tracing to a real requirement → accepted, digest pinned.
	good := "# Design — demo\n\n## Modules\nThe module runs gates.\n\n- references: R1.1\n"
	write("design.md", good)
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatalf("approve design (known ref): %v", err)
	}
	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	var rec core.Record
	if err := json.Unmarshal(state.Records["approval:design"], &rec); err != nil {
		t.Fatalf("approval record: %v", err)
	}
	if rec.SourceDigest != core.Digest([]byte(good)) {
		t.Fatalf("design approval did not pin the source digest: %q", rec.SourceDigest)
	}
}

func TestApproveRejectsExplicitTarget(t *testing.T) {
	root := newDemoSpec(t)
	for _, target := range []string{"design", "tasks", "complete"} {
		if err := Run(root, "approve", []string{"demo", target}, nil); err == nil {
			t.Errorf("approve with explicit target %q accepted; want usage error", target)
		}
	}
}
