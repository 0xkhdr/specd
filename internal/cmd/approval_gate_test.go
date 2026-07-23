package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

// authorDemoSpec replaces the scaffold stubs with real requirements + design so
// the W4 EARS and design-stub gates pass. Sections carry content (design gate
// rejects empty sections) and requirements are EARS-shaped (no shape warnings).
func authorDemoSpec(t *testing.T, root, slug string) {
	t.Helper()
	dir := filepath.Join(root, ".specd", "specs", slug)
	reqs := "# Requirements — " + slug + "\n\n- **R1** When a user runs check, the system shall validate the spec.\n"
	design := "# Design — " + slug + "\n\n## Modules\nThe check module runs gates.\n\n## On-disk contracts\nstate.json holds status.\n\n## Invariants\nOutput is deterministic.\n"
	if err := os.WriteFile(filepath.Join(dir, "requirements.md"), []byte(reqs), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "design.md"), []byte(design), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNextGatedOnApproval(t *testing.T) {
	root := t.TempDir()
	if err := Run(root, "new", []string{"demo"}, nil); err != nil {
		t.Fatalf("new: %v", err)
	}
	authorDemoSpec(t, root, "demo")
	writeTasks(t, root, "demo", "| T1 | scout | requirements.md | - | printf ok | approval-gated fixture |")

	// In the requirements (perceive) phase the coarse phase gate fails closed:
	// next/verify are execution verbs with no approved DAG to act on, so they
	// exit 2 before the handler runs (spec 03 R2).
	if err := Run(root, "next", []string{"demo"}, map[string]string{"json": "1"}); err == nil {
		t.Fatalf("next in perceive phase succeeded (want phase-gate rejection)")
	}
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err == nil {
		t.Fatalf("verify before approval succeeded")
	}

	// First approval advances exactly requirements→design and records approval
	// of requirements. Execution stays blocked until design is approved.
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatalf("approve next: %v", err)
	}
	out, err := captureStdout(t, func() error { return Run(root, "next", []string{"demo"}, map[string]string{"json": "1"}) })
	if err != nil {
		t.Fatalf("next after requirements approval: %v", err)
	}
	if strings.Contains(out, `"id": "T1"`) {
		t.Fatalf("next exposed task before design approval: %s", out)
	}
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatalf("approve design: %v", err)
	}

	out, err = captureStdout(t, func() error {
		return Run(root, "next", []string{"demo"}, map[string]string{"json": "1"})
	})
	if err != nil {
		t.Fatalf("next after approval: %v", err)
	}
	if !strings.Contains(out, `"id": "T1"`) {
		t.Fatalf("next after approval = %s", out)
	}
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("verify after approval: %v", err)
	}
}

// TestApprovalRequestIntegrationCompatibility asserts the interactive approve
// path records an explicit request identity and an immutable two-transition
// history pinned to the identities current at approval time (R5.1, R5.4),
// without dropping the legacy approval record older projections still read.
func TestApprovalRequestIntegrationCompatibility(t *testing.T) {
	root := newDemoSpec(t)
	if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
		t.Fatalf("approve: %v", err)
	}
	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.Records["approval:requirements"]; !ok {
		t.Fatalf("legacy approval record dropped: %v", state.Records)
	}
	requests, err := state.ApprovalRequests()
	if err != nil {
		t.Fatalf("approval requests: %v", err)
	}
	if len(requests) != 2 || requests[0].Transition != core.ApprovalRequested || requests[1].Transition != core.ApprovalApproved {
		t.Fatalf("compatibility approval history = %+v", requests)
	}
	created := requests[0]
	if created.ID == "" || created.ID != requests[1].ID {
		t.Fatalf("approval transitions do not share one request identity: %+v", requests)
	}
	if created.EntityKind != core.ApprovalEntitySpec || created.EntityID != "demo" || created.EntityVersion != "requirements" {
		t.Fatalf("request entity identity = %+v", created)
	}
	if created.Requester == "" || created.ExpiresAt == "" {
		t.Fatalf("request missing requester/expiry: %+v", created)
	}
	raw, err := os.ReadFile(filepath.Join(core.SpecdDir(root), "specs", "demo", "requirements.md"))
	if err != nil {
		t.Fatal(err)
	}
	if created.Pins.ArtifactDigest != core.Digest(raw) {
		t.Fatalf("request pinned artifact digest %q, want %q", created.Pins.ArtifactDigest, core.Digest(raw))
	}
	if created.Pins.PlanDigest == "" || created.Pins.ConfigDigest == "" || created.Pins.StateRevision != 0 {
		t.Fatalf("request pins incomplete: %+v", created.Pins)
	}
	// The approved transition inherits the pins verbatim: approval appends, it
	// never rewrites what the request governs.
	if requests[1].Pins != created.Pins || requests[1].ExpiresAt != created.ExpiresAt {
		t.Fatalf("approval rewrote pinned identity: %+v -> %+v", created, requests[1])
	}
}

func TestApprovalReopenedCycleRetainsHistoryAndApprovesFresh(t *testing.T) {
	root := reopenCLISpec(t)
	flags := map[string]string{"reason": "requirements changed for a new cycle", "expect-revision": reopenRevision(t, root)}
	if _, err := artifactReopenCLI(t, root, []string{"demo", "spec"}, flags); err != nil {
		t.Fatalf("reopen spec: %v", err)
	}
	for _, gate := range []string{"requirements", "design", "tasks"} {
		if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
			t.Fatalf("approve %s in cycle 2: %v", gate, err)
		}
	}
	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if state.Cycle != 2 || state.Status != core.StatusExecuting {
		t.Fatalf("state = %+v, want cycle 2 executing", state)
	}
	for _, gate := range []string{"requirements", "design", "tasks"} {
		if _, ok := state.Records["approval:"+gate+":cycle:1"]; !ok {
			t.Fatalf("records = %+v, want prior %s approval history", state.Records, gate)
		}
		if _, ok := state.Records["approval:"+gate]; !ok {
			t.Fatalf("records = %+v, want current %s approval", state.Records, gate)
		}
	}
	requests, err := state.ApprovalRequests()
	if err != nil {
		t.Fatal(err)
	}
	for _, gate := range []string{"requirements", "design", "tasks"} {
		for _, id := range []string{core.ApprovalRequestID(gate, 1), core.ApprovalRequestID(gate, 2)} {
			latest, count := core.LatestApprovalRequest(requests, id)
			if latest.Transition != core.ApprovalApproved || count != 2 {
				t.Fatalf("request %s = %+v after %d transitions, want retained approved history", id, latest, count)
			}
		}
	}
}

// TestApprovalRequestIntegrationStaleDigest asserts approve refuses when an
// already-open request pinned inputs that have since drifted, and leaves state
// untouched (R5.3). Recovery is a new or superseding request; there is no
// bypass.
func TestApprovalRequestIntegrationStaleDigest(t *testing.T) {
	root := newDemoSpec(t)
	statePath := core.StatePath(root, "demo")
	before, err := core.LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	pending := core.StampApprovalRequest(core.ApprovalRequestRecord{
		ID:            "approve:requirements",
		Transition:    core.ApprovalRequested,
		EntityKind:    core.ApprovalEntitySpec,
		EntityID:      "demo",
		EntityVersion: "requirements",
		Pins:          core.ApprovalPins{ArtifactDigest: "stale", StateRevision: 0, PlanDigest: "stale", ConfigDigest: "stale"},
		Requester:     "human",
		ExpiresAt:     core.Clock().Add(time.Hour).Format(time.RFC3339),
	}, "head")
	raw, err := json.Marshal(pending)
	if err != nil {
		t.Fatal(err)
	}
	before.Records["approval_request:approve:requirements:0"] = raw
	if err := core.SaveStateCAS(statePath, before.Revision, before); err != nil {
		t.Fatal(err)
	}
	seeded, err := core.LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}

	approveErr := Run(root, "approve", []string{"demo"}, nil)
	if approveErr == nil || !strings.Contains(approveErr.Error(), "stale") {
		t.Fatalf("approve on drifted request = %v, want stale refusal", approveErr)
	}
	if !strings.Contains(approveErr.Error(), "superseding request") {
		t.Fatalf("stale refusal does not name the recovery: %v", approveErr)
	}
	after, err := core.LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != seeded.Revision || after.Status != seeded.Status || len(after.Records) != len(seeded.Records) {
		t.Fatalf("refused approval mutated state: %+v -> %+v", seeded, after)
	}
}

func TestReadinessApprovalParity(t *testing.T) {
	t.Run("green_same_revision_plan", func(t *testing.T) {
		root := newDemoSpec(t)
		out, err := captureStdout(t, func() error {
			return Run(root, "check", []string{"demo"}, map[string]string{"json": "true"})
		})
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		var envelope core.ReadinessEnvelope
		if err := json.Unmarshal([]byte(out), &envelope); err != nil {
			t.Fatalf("readiness envelope: %v\n%s", err, out)
		}
		if envelope.SchemaVersion != core.ReadinessSchemaVersion || !envelope.Plan.ReadinessChecked {
			t.Fatalf("unversioned or unchecked readiness: %+v", envelope)
		}
		if envelope.Plan.Target != core.StatusDesign || envelope.Plan.ConfigDigest == "" || len(envelope.Plan.ArmedGates) == 0 || len(envelope.Plan.ArtifactDigests) != 3 {
			t.Fatalf("incomplete readiness identity: %+v", envelope.Plan)
		}

		approved, err := captureStdout(t, func() error { return Run(root, "approve", []string{"demo"}, nil) })
		if err != nil {
			t.Fatalf("approve: %v", err)
		}
		for _, want := range []string{envelope.Plan.PlanDigest, "revision " + fmt.Sprint(envelope.Plan.StateRevision)} {
			if !strings.Contains(approved, want) {
				t.Fatalf("approval did not consume readiness identity %q: %s", want, approved)
			}
		}
	})

	t.Run("blockers_and_legacy_shape", func(t *testing.T) {
		root := newDemoSpec(t)
		writeTasks(t, root, "demo", "| T1 | scout | spec.md | T9 | true | ok |")
		out, err := captureStdout(t, func() error {
			return Run(root, "check", []string{"demo"}, map[string]string{"json": "true"})
		})
		if err == nil {
			t.Fatal("blocking readiness returned success")
		}
		var envelope core.ReadinessEnvelope
		if err := json.Unmarshal([]byte(out), &envelope); err != nil {
			t.Fatalf("readiness envelope: %v\n%s", err, out)
		}
		if len(envelope.Plan.Blockers) == 0 {
			t.Fatalf("blocking finding missing from plan: %+v", envelope.Plan)
		}
		before, _ := core.LoadState(core.StatePath(root, "demo"))
		approveErr := Run(root, "approve", []string{"demo"}, nil)
		if approveErr == nil || !strings.Contains(approveErr.Error(), envelope.Plan.PlanDigest) {
			t.Fatalf("approval did not consume blocking plan %s: %v", envelope.Plan.PlanDigest, approveErr)
		}
		after, _ := core.LoadState(core.StatePath(root, "demo"))
		if after.Revision != before.Revision || after.Status != before.Status {
			t.Fatalf("blocked approval mutated state: %+v -> %+v", before, after)
		}

		legacy, err := captureStdout(t, func() error {
			return Run(root, "check", []string{"demo"}, map[string]string{"json": "legacy"})
		})
		if err != nil {
			t.Fatalf("legacy check: %v", err)
		}
		var findings []gates.Finding
		if err := json.Unmarshal([]byte(legacy), &findings); err != nil || len(findings) != len(envelope.Findings) {
			t.Fatalf("legacy findings: err=%v findings=%+v envelope=%+v", err, findings, envelope.Findings)
		}
	})

	t.Run("warnings_do_not_block", func(t *testing.T) {
		root := newDemoSpec(t)
		requirements := filepath.Join(core.SpecdDir(root), "specs", "demo", "requirements.md")
		if err := os.WriteFile(requirements, []byte("# Requirements — demo\n\n- R1 describes validation.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := captureStdout(t, func() error {
			return Run(root, "check", []string{"demo"}, map[string]string{"json": "true"})
		})
		if err != nil {
			t.Fatalf("warning-only check: %v", err)
		}
		var envelope core.ReadinessEnvelope
		if err := json.Unmarshal([]byte(out), &envelope); err != nil {
			t.Fatal(err)
		}
		if len(envelope.Plan.Blockers) != 0 || len(envelope.Plan.Warnings) == 0 {
			t.Fatalf("warning classification = %+v", envelope.Plan)
		}
		if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
			t.Fatalf("warning blocked approval: %v", err)
		}
	})

	t.Run("revision_drift_replans", func(t *testing.T) {
		root := newDemoSpec(t)
		state, _ := core.LoadState(core.StatePath(root, "demo"))
		old, err := buildReadiness(root, "demo", state)
		if err != nil {
			t.Fatal(err)
		}
		if err := Run(root, "request-decision", []string{"demo"}, map[string]string{"text": "record drift"}); err != nil {
			t.Fatal(err)
		}
		approved, err := captureStdout(t, func() error { return Run(root, "approve", []string{"demo"}, nil) })
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(approved, old.Envelope.Plan.PlanDigest) || !strings.Contains(approved, "revision 1") {
			t.Fatalf("approval consumed stale revision-0 plan: %s", approved)
		}
	})
}
