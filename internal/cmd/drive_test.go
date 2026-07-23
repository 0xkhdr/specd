package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// driveFixture builds a spec approved through to execution with one runnable
// task, which is the only state in which `drive` has work to select.
func driveFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := core.WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("requirements.md", "# Requirements — demo\n\n- **R1** When drive runs, the system shall return one envelope.\n")
	write("design.md", "# Design — demo\n\n## Modules\nThe CLI projects existing primitives.\n\n## On-disk contracts\nState lives under the spec directory.\n\n## Invariants\nCompletion requires passing evidence.\n")
	// A write task needs a verify line that actually exercises its change; a
	// trivial one is a gate blocker, and a blocked fixture would not exercise
	// the envelope's happy path.
	write("tasks.md", "# Tasks — demo\n\n| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | drive-proof.txt | - | test -f drive-proof.txt | R1 |\n")

	state := core.State{
		SchemaVersion: 1,
		Slug:          "demo",
		Mode:          "default",
		Status:        core.StatusExecuting,
		Phase:         core.PhaseExecute,
		Revision:      3,
		Records:       map[string]json.RawMessage{},
	}
	for _, gate := range []string{"requirements", "design", "tasks"} {
		state.Records["approval:"+gate] = json.RawMessage(`{"kind":"approval","gate":"` + gate + `","actor":"tester"}`)
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(core.StatePath(root, "demo"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// R1.1: one call returns every field a host needs to act. Asserted field by
// field, because an envelope that silently drops one sends the host back to the
// granular commands `drive` exists to replace.
func TestDriveEnvelopeCarriesEveryRequiredField(t *testing.T) {
	root := driveFixture(t)

	out, err := captureStdout(t, func() error {
		return Run(root, "drive", []string{"demo"}, map[string]string{"json": "true"})
	})
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	var got DriveEnvelope
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("drive JSON invalid: %v\n%s", err, out)
	}

	if got.ProtocolVersion != core.DriverProtocolVersion {
		t.Errorf("protocol_version = %q, want %q", got.ProtocolVersion, core.DriverProtocolVersion)
	}
	if got.RequestMode.Mode != core.RequestModeManaged || got.RequestMode.SelectedSpec != "demo" || !got.RequestMode.HandshakeRequired {
		t.Errorf("request routing = %+v", got.RequestMode)
	}
	if got.ExecutionMode != "default" {
		t.Errorf("execution_mode = %q, want default", got.ExecutionMode)
	}
	if got.Slug != "demo" {
		t.Errorf("spec_slug = %q, want demo", got.Slug)
	}
	if got.Revision != 3 {
		t.Errorf("revision = %d, want 3", got.Revision)
	}
	if got.Phase != core.PhaseExecute || got.Status != core.StatusExecuting {
		t.Errorf("phase/status = %s/%s, want execute/executing", got.Phase, got.Status)
	}
	if got.Assurance == "" {
		t.Error("assurance is empty; a host cannot tell a governed session from an advisory one")
	}
	if got.PermittedActor == "" {
		t.Error("permitted_actor is empty")
	}
	if len(got.LegalOperations) == 0 {
		t.Error("legal_operations is empty")
	}
	if got.HumanOnly == nil {
		t.Error("human_only is nil; the human-only set must be stated, not inferred")
	}
	if got.SelectedTask == nil || got.SelectedTask.ID != "T1" {
		t.Fatalf("selected_task = %+v, want T1", got.SelectedTask)
	}
	if got.SelectedTask.Role != "craftsman" || got.SelectedTask.Verify == "" || len(got.SelectedTask.DeclaredFiles) == 0 {
		t.Errorf("selected_task incomplete: %+v", got.SelectedTask)
	}
	if got.Authority == nil {
		t.Fatal("authority is nil; a host cannot derive permissions from an absent packet")
	}
	if got.Authority.TaskID != "T1" || got.Authority.Mode != "write" {
		t.Errorf("authority not bound to the selected task: %+v", got.Authority)
	}
	if got.ContextManifestDigest == "" {
		t.Error("context_manifest_digest is empty")
	}
	if got.Blockers == nil {
		t.Error("blockers is nil; an empty set must serialize as [] so absent and none are distinguishable")
	}
	if got.NextOperation == "" {
		t.Error("next_operation is empty")
	}
}

// R1.1: the envelope names an exact command, not a description of one.
func TestDriveEnvelopeNamesRunnableNextOperation(t *testing.T) {
	root := driveFixture(t)
	envelope, err := buildDriveEnvelope(root, "demo", false, time.Now())
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if !strings.HasPrefix(envelope.NextOperation, "specd ") {
		t.Fatalf("next_operation %q is not a runnable command", envelope.NextOperation)
	}
	// With no session open, the precondition for all mutable work is opening
	// one, so that must be what drive names first.
	if !strings.Contains(envelope.NextOperation, "session open") {
		t.Fatalf("next_operation = %q, want the session-open precondition", envelope.NextOperation)
	}
	if envelope.SessionID != "" {
		t.Fatalf("session_id = %q, want empty with no session open", envelope.SessionID)
	}
}

// R1.1: once a session is open, drive reports it and moves on to real work.
func TestDriveEnvelopeReportsOpenSession(t *testing.T) {
	root := driveFixture(t)
	now := time.Now()
	session, err := core.OpenDriverSession(root, "demo", "test-host", "handshake-digest", "", 3, now)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := buildDriveEnvelope(root, "demo", false, now)
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if envelope.SessionID != session.ID || envelope.Driver != "test-host" {
		t.Fatalf("session not reported: %+v", envelope)
	}
	if strings.Contains(envelope.NextOperation, "session open") {
		t.Fatalf("drive still asks to open a session that is already open: %q", envelope.NextOperation)
	}
}

// An expired session must read as absent. A host that sees an id will use it,
// and this one would refuse on every operation.
func TestDriveEnvelopeTreatsExpiredSessionAsAbsent(t *testing.T) {
	root := driveFixture(t)
	issued := time.Now()
	if _, err := core.OpenDriverSession(root, "demo", "test-host", "handshake-digest", "", 3, issued); err != nil {
		t.Fatal(err)
	}
	envelope, err := buildDriveEnvelope(root, "demo", false, issued.Add(core.DriverSessionTTL+time.Minute))
	if err != nil {
		t.Fatalf("drive: %v", err)
	}
	if envelope.SessionID != "" {
		t.Fatalf("expired session reported as open: %q", envelope.SessionID)
	}
	if !strings.Contains(envelope.NextOperation, "session open") {
		t.Fatalf("next_operation = %q, want a fresh session-open", envelope.NextOperation)
	}
}

// R5.4: a host that declares no sandbox cannot be reported as fully governed,
// and nothing it declares can raise the level on its own.
func TestDriveEnvelopeAssuranceCappedByHostCapabilities(t *testing.T) {
	root := driveFixture(t)
	now := time.Now()

	advisory, err := buildDriveEnvelope(root, "demo", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if advisory.Assurance != core.AssuranceAdvisory {
		t.Fatalf("assurance = %q with no sandbox, want advisory", advisory.Assurance)
	}
	if advisory.Authority == nil || advisory.Authority.SandboxProfile != "none" {
		t.Fatalf("authority claims a sandbox the host never declared: %+v", advisory.Authority)
	}

	// --sandbox alone does NOT buy a governed label. A CLI invocation asserts
	// one control; the host contract requires seven more, and specd will not
	// present a session as governed on the strength of a flag.
	sandboxed, err := buildDriveEnvelope(root, "demo", true, now)
	if err != nil {
		t.Fatal(err)
	}
	if sandboxed.Assurance != core.AssuranceAdvisory {
		t.Fatalf("assurance = %q from --sandbox alone; a flag must not buy a governed label", sandboxed.Assurance)
	}
	if len(sandboxed.UnmetControls) == 0 {
		t.Fatal("advisory session names no unmet control, so an operator cannot see what to fix")
	}
	// Sandbox is a ceiling, not one of the seven controls, so it never appears
	// in the unmet set and declaring it changes nothing a CLI host can claim.
	// That is the point: a command-line flag is not an attestation, and the
	// full contract is declared over MCP where the host speaks for itself.
	for _, unmet := range sandboxed.UnmetControls {
		if strings.Contains(unmet, "sandbox") {
			t.Errorf("sandbox reported as a control rather than a ceiling: %v", sandboxed.UnmetControls)
		}
	}
	if len(advisory.UnmetControls) != len(sandboxed.UnmetControls) {
		t.Errorf("the sandbox flag changed the control set: %v vs %v", advisory.UnmetControls, sandboxed.UnmetControls)
	}
	// Every unmet entry cites its clause, so the label is actionable.
	for _, unmet := range sandboxed.UnmetControls {
		if !strings.HasPrefix(unmet, "R5.") {
			t.Errorf("unmet control %q cites no requirement clause", unmet)
		}
	}
}

// R1.2: a spec that cannot be driven refuses with a typed refusal naming the
// actor who unblocks it, rather than returning an envelope with nothing in it.
func TestDriveRefusesNonDriveableSpec(t *testing.T) {
	root := t.TempDir()
	if err := core.WriteScaffold(root); err != nil {
		t.Fatal(err)
	}
	_, err := buildDriveEnvelope(root, "missing", false, time.Now())
	if err == nil {
		t.Fatal("drive returned an envelope for a spec that does not exist")
	}
	refusal, ok := core.AsRefusal(err)
	if !ok {
		t.Fatalf("got bare error %v, want a typed refusal", err)
	}
	if refusal.ActorRequired == "" {
		t.Fatalf("refusal names no actor to unblock it: %+v", refusal)
	}
	if refusal.RecoveryCommand == "" {
		t.Fatalf("refusal names no recovery command: %+v", refusal)
	}
}

// R1.3: drive is a projection, so its view must agree with the granular command
// it summarizes. A second source of truth is the failure this pins against.
func TestDriveAgreesWithGranularGuide(t *testing.T) {
	root := driveFixture(t)
	envelope, err := buildDriveEnvelope(root, "demo", false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	guide, err := driverGuideForSpec(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Phase != guide.Phase || envelope.Status != guide.Status {
		t.Fatalf("drive reports %s/%s, guide reports %s/%s", envelope.Phase, envelope.Status, guide.Phase, guide.Status)
	}
	if len(envelope.LegalOperations) != len(guide.NextActions) {
		t.Fatalf("drive lists %d legal operations, guide lists %d", len(envelope.LegalOperations), len(guide.NextActions))
	}
	if len(guide.Frontier) > 0 {
		if envelope.SelectedTask == nil || envelope.SelectedTask.ID != guide.Frontier[0] {
			t.Fatalf("drive selected %+v, frontier head is %s", envelope.SelectedTask, guide.Frontier[0])
		}
	}
}

// TestWorkerColumnDriveDisposition pins spec R6.3 in the cmd layer: drive's
// selected-task projection renders the worker disposition, a dash row stays
// host-chooses (dispatch unchanged), and a continued worker is reported.
func TestWorkerColumnDriveDisposition(t *testing.T) {
	tasks := []core.TaskRow{
		{ID: "T1", Worker: "w1"},
		{ID: "T2", Worker: "w1", DependsOn: []string{"T1"}},
		{ID: "T3", Worker: "-", DependsOn: []string{"T1"}},
	}
	// T1 complete → w1 active, so T2 continues; T3 dash → host-chooses.
	cases := map[string]string{
		"T2": "worker=w1 (continues)",
		"T3": "host-chooses",
	}
	// selectedFrontierTask derives from status; simulate T1 complete via marker.
	tasks[0].Marker = "✅"
	for id, want := range cases {
		got := core.WorkerDisposition(selectedFrontierTask(tasks, id))
		if got != want {
			t.Fatalf("drive disposition for %s = %q, want %q", id, got, want)
		}
	}
}
