package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

// TestStatusGuideJSON pins spec 01 R6.1: `status --guide --json` emits the
// machine driving guidance with the phase, the required artifact, the
// machine-legal commands, and approval kept in the human-only set.
func TestStatusGuideJSON(t *testing.T) {
	root := newDemoSpec(t)
	out, err := captureStdout(t, func() error {
		return Run(root, "status", []string{"demo"}, map[string]string{"guide": "", "json": ""})
	})
	if err != nil {
		t.Fatalf("status --guide --json: %v", err)
	}
	var g core.Guidance
	if err := json.Unmarshal([]byte(out), &g); err != nil {
		t.Fatalf("guide json: %v (out=%q)", err, out)
	}
	if g.Phase != core.PhasePerceive || g.RequiredArtifact != "requirements.md" {
		t.Fatalf("guidance = %+v", g)
	}
	if !containsStr(g.HumanOnly, "approve") || containsStr(g.LegalCommands, "approve") {
		t.Fatalf("approve must be human-only, never machine-legal: %+v", g)
	}
}

// TestStatusGuideSuppressesTaskVerify pins spec 01 R6.2: with no executable
// task, the guidance does not suggest task verify (nor agent self-approval).
func TestStatusGuideSuppressesTaskVerify(t *testing.T) {
	root := newDemoSpec(t)
	g, err := guidanceForSpec(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if containsStr(g.LegalCommands, "verify") {
		t.Fatalf("verify must not be suggested without an executable task: %v", g.LegalCommands)
	}
}

func TestModeAndCriterionReadSurfaces(t *testing.T) {
	root := newCriterionSpec(t)
	writeTasks(t, root, "demo", "| ✅ T1 | scout | spec.md | - | true | R1.1 |")
	statePath := core.StatePath(root, "demo")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	mode, err := captureStdout(t, func() error { return Run(root, "mode", []string{"demo"}, nil) })
	if err != nil || mode != "mode: default\n" {
		t.Fatalf("mode read = %q, %v", mode, err)
	}
	operation, ok := core.ResolveOperation("mode", []string{"demo"}, nil)
	if !ok || operation.ID != "mode.read" || operation.Effect != core.EffectRead {
		t.Fatalf("mode read operation = %+v, found=%v", operation, ok)
	}

	text, err := captureStdout(t, func() error { return Run(root, "status", []string{"demo"}, nil) })
	if err != nil || !strings.Contains(text, "mode: default") {
		t.Fatalf("status mode = %q, %v", text, err)
	}
	jsonStatus, err := captureStdout(t, func() error {
		return Run(root, "status", []string{"demo"}, map[string]string{"json": ""})
	})
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}
	var status struct {
		Mode core.Mode `json:"mode"`
	}
	if err := json.Unmarshal([]byte(jsonStatus), &status); err != nil || status.Mode != core.ModeDefault {
		t.Fatalf("status json mode = %+v, %v", status, err)
	}

	verifyOperation, ok := core.OperationByID("verify.criterion")
	if !ok {
		t.Fatal("verify.criterion missing from palette")
	}
	verifyRoute := strings.NewReplacer("<slug>", "demo", "<r>.<n>", "1.1").Replace(verifyOperation.Usage)
	guideText, err := captureStdout(t, func() error {
		return Run(root, "status", []string{"demo"}, map[string]string{"guide": ""})
	})
	if err != nil || !strings.Contains(guideText, "(status tasks, mode default)") || !strings.Contains(guideText, verifyRoute) {
		t.Fatalf("status guide = %q, %v; want route %q", guideText, err, verifyRoute)
	}
	guideJSON, err := captureStdout(t, func() error {
		return Run(root, "status", []string{"demo"}, map[string]string{"guide": "", "json": ""})
	})
	if err != nil {
		t.Fatalf("status --guide --json: %v", err)
	}
	var guide core.Guidance
	if err := json.Unmarshal([]byte(guideJSON), &guide); err != nil {
		t.Fatalf("guide json: %v", err)
	}
	if guide.Mode != core.ModeDefault || !strings.Contains(strings.Join(guide.Blockers, "\n"), verifyRoute) {
		t.Fatalf("guide = %+v, want mode and route %q", guide, verifyRoute)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("mode/status reads mutated state")
	}

	verifyCommand, ok := core.CommandByName("verify")
	if !ok {
		t.Fatal("verify missing from palette")
	}
	for _, flags := range []map[string]string{nil, {"criterion": "1.1"}} {
		err := runVerify(root, nil, flags)
		if !errors.Is(err, ErrUsage) || !strings.Contains(err.Error(), verifyCommand.Usage) {
			t.Fatalf("verify arity error = %v, want palette usage %q", err, verifyCommand.Usage)
		}
	}

	if err := Run(root, "verify", []string{"demo"}, map[string]string{
		"criterion": "1.1",
		"status":    "pass",
		"evidence":  "covered",
	}); err != nil {
		t.Fatalf("cover criterion: %v", err)
	}
	covered, err := captureStdout(t, func() error {
		return Run(root, "status", []string{"demo"}, map[string]string{"guide": ""})
	})
	if err != nil || strings.Contains(covered, verifyRoute) {
		t.Fatalf("covered criterion guide = %q, %v; route must be silent", covered, err)
	}
}

// TestStageConditionMigrationCompatibilityProjection pins spec 03 R2.3/R6.4: a
// schema-1 state.json and its migrated schema-2 form drive identical guidance,
// so legacy projects keep working while the canonical pair becomes the truth.
func TestStageConditionMigrationCompatibilityProjection(t *testing.T) {
	root := newDemoSpec(t)
	statePath := core.StatePath(root, "demo")
	current, err := captureStdout(t, func() error {
		return Run(root, "status", []string{"demo"}, map[string]string{"guide": "", "json": ""})
	})
	if err != nil {
		t.Fatalf("status --guide --json: %v", err)
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	fields["schema_version"] = json.RawMessage("1")
	delete(fields, "cycle")
	delete(fields, "stage")
	delete(fields, "condition")
	legacy, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	projected, err := captureStdout(t, func() error {
		return Run(root, "status", []string{"demo"}, map[string]string{"guide": "", "json": ""})
	})
	if err != nil {
		t.Fatalf("legacy status --guide --json: %v", err)
	}
	if projected != current {
		t.Fatalf("legacy guidance = %q, want the schema-2 guidance %q", projected, current)
	}
	state, err := core.LoadState(statePath)
	if err != nil {
		t.Fatalf("load legacy state: %v", err)
	}
	if state.Cycle != 1 || state.Stage != core.StageRequirements || state.Condition != core.ConditionActive {
		t.Fatalf("legacy state projection = %+v", state)
	}
}

// TestTaskActivityReadinessStatusProjection pins spec 03 R3.1/R3.3/R3.4 at the
// CLI edge: `status` reports each task's activity and readiness as separate
// values, names the derived dependency wait with its refs, and lists the pending
// tasks that block parent completion.
func TestTaskActivityReadinessStatusProjection(t *testing.T) {
	root := newDemoSpec(t)
	writeTasks(t, root, "demo", "| T1 | scout | spec.md | - | true | ok |\n| T2 | scout | spec.md | T1 | true | ok |")
	out, err := captureStdout(t, func() error {
		return Run(root, "status", []string{"demo"}, map[string]string{"json": ""})
	})
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}
	var model core.ReportModel
	if err := json.Unmarshal([]byte(out), &model); err != nil {
		t.Fatalf("status json: %v (out=%q)", err, out)
	}
	if len(model.Tasks) != 2 {
		t.Fatalf("tasks = %#v, want two", model.Tasks)
	}
	if model.Tasks[0].Activity != core.ActivityPending || model.Tasks[0].Readiness != core.ReadinessReady {
		t.Fatalf("T1 = %#v, want pending and ready", model.Tasks[0])
	}
	waits := model.Tasks[1].Waits
	if model.Tasks[1].Readiness != core.ReadinessWaitingDependency || len(waits) != 1 || waits[0].Code != core.WaitDependencyIncomplete {
		t.Fatalf("T2 = %#v, want a named dependency wait", model.Tasks[1])
	}
	if len(waits[0].Refs) != 1 || waits[0].Refs[0] != "T1" {
		t.Fatalf("dependency wait refs = %v, want T1", waits[0].Refs)
	}
	if !containsStr(model.PendingBlockers, "T1") || !containsStr(model.PendingBlockers, "T2") {
		t.Fatalf("pending blockers = %v, want both pending tasks", model.PendingBlockers)
	}
}

func TestProgramStatusUsesLifecycleState(t *testing.T) {
	root := t.TempDir()
	if err := Run(root, "init", nil, nil); err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, slug := range []string{"root", "middle", "later"} {
		if err := Run(root, "new", []string{slug}, nil); err != nil {
			t.Fatalf("new %s: %v", slug, err)
		}
	}
	if err := Run(root, "link", []string{"later", "middle"}, nil); err != nil {
		t.Fatalf("link later: %v", err)
	}
	if err := Run(root, "link", []string{"middle", "root"}, nil); err != nil {
		t.Fatalf("link middle: %v", err)
	}

	out, err := captureStdout(t, func() error {
		return Run(root, "status", nil, map[string]string{"program": ""})
	})
	if err != nil {
		t.Fatalf("status --program: %v", err)
	}
	for _, want := range []string{
		"root  phase=perceive mode=default",
		"depends on middle [pending]",
		"depends on root [pending]",
		"program frontier (actionable now): root",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("program status missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "(complete)") {
		t.Fatalf("requirements-stage spec reported complete:\n%s", out)
	}
}

// TestApprovalRequestIntegrationStatusProjection pins spec 03 R5.3/R5.4:
// `status` projects the immutable approval-request identity — id, current
// transition, entity, pinned identities, expiry — in both renderings, and once
// the pinned artifact drifts the projection names the drift so the next
// approval attempt's refusal is visible before it is attempted.
func TestApprovalRequestIntegrationStatusProjection(t *testing.T) {
	root := newDemoSpec(t)
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatalf("approve: %v", err)
	}

	states := statusApprovalRequests(t, root)
	if len(states) != 1 {
		t.Fatalf("status projected %d approval requests, want 1: %+v", len(states), states)
	}
	projected := states[0]
	if projected.ID != "approve:requirements" || projected.State != core.ApprovalApproved || projected.Entity != "spec:demo" {
		t.Fatalf("approval request projection = %+v", projected)
	}
	if projected.ExpiresAt == "" || projected.Pins.ArtifactDigest == "" || projected.Pins.PlanDigest == "" {
		t.Fatalf("projection dropped the pinned identity: %+v", projected)
	}
	if len(projected.Drift) != 0 {
		t.Fatalf("fresh request reported as drifted: %+v", projected)
	}
	text, err := captureStdout(t, func() error { return Run(root, "status", []string{"demo"}, nil) })
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(text, "Approval requests:") || !strings.Contains(text, "approve:requirements — approved (spec:demo)") {
		t.Fatalf("human status missing approval request identity: %s", text)
	}

	// Amending the approved artifact drifts the identity the request pinned.
	path := filepath.Join(core.SpecdDir(root), "specs", "demo", "requirements.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, []byte("\n- **R2** When a user amends, the system shall drift.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	drifted := statusApprovalRequests(t, root)
	if len(drifted) != 1 || len(drifted[0].Drift) != 1 || drifted[0].Drift[0] != "artifact digest" {
		t.Fatalf("drifted artifact not projected: %+v", drifted)
	}
	if drifted[0].ID != projected.ID || drifted[0].Pins != projected.Pins {
		t.Fatalf("drift rewrote the immutable request identity: %+v -> %+v", projected, drifted[0])
	}
	text, err = captureStdout(t, func() error { return Run(root, "status", []string{"demo"}, nil) })
	if err != nil {
		t.Fatalf("status after drift: %v", err)
	}
	if !strings.Contains(text, "approved inputs drifted (artifact digest)") || !strings.Contains(text, "new or superseding request") {
		t.Fatalf("human status did not name the drift: %s", text)
	}
}

// statusApprovalRequests reads the approval-request projection out of
// `status --json`.
func statusApprovalRequests(t *testing.T, root string) []gates.ApprovalRequestState {
	t.Helper()
	out, err := captureStdout(t, func() error {
		return Run(root, "status", []string{"demo"}, map[string]string{"json": ""})
	})
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}
	var payload struct {
		ApprovalRequests []gates.ApprovalRequestState `json:"approval_requests"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("status json: %v (out=%q)", err, out)
	}
	return payload.ApprovalRequests
}

// TestControllerApprovalHandoffStatusSection pins the status surface of R4.1: a
// spec with no halted controller renders exactly what it always did, and a
// halted one names the gate and both routes out of it.
func TestControllerApprovalHandoffStatusSection(t *testing.T) {
	if got := waitingApprovalGate(t.TempDir(), "demo"); got != "" {
		t.Fatalf("a project with no session reports a halt: %q", got)
	}
	if got := renderWaitingApproval(""); got != "" {
		t.Fatalf("status added a section with nothing to report: %q", got)
	}
	section := renderWaitingApproval("tasks")
	for _, want := range []string{"waiting_approval", "tasks", "specd approve", "specd delegate approve"} {
		if !strings.Contains(section, want) {
			t.Errorf("halt section %q omits %q", section, want)
		}
	}
}
