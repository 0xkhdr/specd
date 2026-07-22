package cmd

import (
	"encoding/json"
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
