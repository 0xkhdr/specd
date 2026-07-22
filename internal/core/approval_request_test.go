package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestApprovalRequestLifecycle pins the spec 03 R5 contract: a request pins the
// identities that govern it, only the canonical transitions are legal, and an
// approval whose pinned inputs drifted (or whose expiry passed) refuses as stale
// instead of consuming a request that no longer describes reality.
func TestApprovalRequestLifecycle(t *testing.T) {
	pins := ApprovalPins{ArtifactDigest: "a1", StateRevision: 7, PlanDigest: "p1", ConfigDigest: "c1"}
	create := func(id string) ApprovalRequestRecord {
		return ApprovalRequestRecord{
			ID: id, Transition: ApprovalRequested, EntityKind: ApprovalEntityArtifact,
			EntityID: "design.md", EntityVersion: "v2", Pins: pins, Requester: "maintainer",
			ExpiresAt: "2999-01-01T00:00:00Z",
		}
	}
	plan := func(t *testing.T, existing []ApprovalRequestRecord, rec ApprovalRequestRecord) []ApprovalRequestRecord {
		t.Helper()
		key, planned, err := PlanApprovalRequest(existing, rec)
		if err != nil {
			t.Fatalf("PlanApprovalRequest(%s %s): %v", rec.ID, rec.Transition, err)
		}
		if key == "" {
			t.Fatalf("PlanApprovalRequest(%s %s) returned an empty key", rec.ID, rec.Transition)
		}
		return append(append([]ApprovalRequestRecord(nil), existing...), planned)
	}

	t.Run("PinnedIdentities", func(t *testing.T) {
		records := plan(t, nil, create("AR1"))
		records = plan(t, records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalApproved, Pins: pins})
		approved := records[1]
		if approved.Pins != pins || approved.Requester != "maintainer" || approved.EntityID != "design.md" || approved.EntityVersion != "v2" {
			t.Fatalf("approval = %#v, want the request's identities inherited verbatim", approved)
		}
		if records[0].Transition != ApprovalRequested {
			t.Fatalf("create record mutated to %q, want it untouched", records[0].Transition)
		}
	})

	t.Run("MissingPins", func(t *testing.T) {
		incomplete := create("AR1")
		incomplete.Pins.ConfigDigest = ""
		if _, _, err := PlanApprovalRequest(nil, incomplete); err == nil {
			t.Fatal("PlanApprovalRequest accepted a request without a config digest")
		}
	})

	t.Run("StaleArtifact", func(t *testing.T) {
		records := plan(t, nil, create("AR1"))
		drifted := pins
		drifted.ArtifactDigest = "a2"
		_, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalApproved, Pins: drifted})
		if err == nil {
			t.Fatal("PlanApprovalRequest approved against a drifted artifact digest")
		}
		if got := err.Error(); !strings.Contains(got, "stale") || !strings.Contains(got, "artifact digest") {
			t.Fatalf("error = %q, want a stale artifact-digest refusal", got)
		}
		drifted = pins
		drifted.StateRevision = 8
		if _, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalApproved, Pins: drifted}); err == nil {
			t.Fatal("PlanApprovalRequest approved against a drifted state revision")
		}
		drifted = pins
		drifted.PlanDigest = "p2"
		if _, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalApproved, Pins: drifted}); err == nil {
			t.Fatal("PlanApprovalRequest approved against a drifted transition-plan digest")
		}
		drifted = pins
		drifted.ConfigDigest = "c2"
		if _, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalApproved, Pins: drifted}); err == nil {
			t.Fatal("PlanApprovalRequest approved against a drifted config digest")
		}
	})

	t.Run("DuplicateTransition", func(t *testing.T) {
		records := plan(t, nil, create("AR1"))
		if _, _, err := PlanApprovalRequest(records, create("AR1")); err == nil {
			t.Fatal("PlanApprovalRequest accepted a duplicate requested transition")
		}
		records = plan(t, records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalApproved, Pins: pins})
		if _, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalApproved, Pins: pins}); err == nil {
			t.Fatal("PlanApprovalRequest accepted a duplicate approval")
		}
		if _, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalRejected}); err == nil {
			t.Fatal("PlanApprovalRequest rejected an already approved request")
		}
		if _, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: "acknowledged"}); err == nil {
			t.Fatal("PlanApprovalRequest accepted a transition outside the closed set")
		}
	})

	t.Run("Revocation", func(t *testing.T) {
		records := plan(t, nil, create("AR1"))
		records = plan(t, records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalApproved, Pins: pins})
		records = plan(t, records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalRevoked, Reason: "regression found"})
		if latest, count := LatestApprovalRequest(records, "AR1"); latest.Transition != ApprovalRevoked || count != 3 {
			t.Fatalf("chain = %q after %d transitions, want revoked after 3", latest.Transition, count)
		}
		if ApprovalRequestPending(records, "AR1") {
			t.Fatal("a revoked request still reports as pending")
		}
		if _, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalApproved, Pins: pins}); err == nil {
			t.Fatal("PlanApprovalRequest re-approved a revoked request")
		}
	})

	t.Run("Expiry", func(t *testing.T) {
		expiring := create("AR1")
		expiring.ExpiresAt = "2020-01-01T00:00:00Z"
		records := plan(t, nil, expiring)
		if _, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalApproved, Pins: pins}); err == nil {
			t.Fatal("PlanApprovalRequest approved an expired request")
		}
		records = plan(t, records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalExpired})
		if ApprovalRequestPending(records, "AR1") {
			t.Fatal("an expired request still reports as pending")
		}
	})

	t.Run("Supersession", func(t *testing.T) {
		records := plan(t, nil, create("AR1"))
		if _, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalSuperseded}); err == nil {
			t.Fatal("PlanApprovalRequest superseded a request without naming its replacement")
		}
		if _, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalSuperseded, SupersededBy: "AR9"}); err == nil {
			t.Fatal("PlanApprovalRequest superseded a request by an unknown replacement")
		}
		records = plan(t, records, create("AR2"))
		records = plan(t, records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalSuperseded, SupersededBy: "AR2"})
		if ApprovalRequestPending(records, "AR1") || !ApprovalRequestPending(records, "AR2") {
			t.Fatal("supersession did not move the pending request from AR1 to AR2")
		}
	})

	t.Run("StateProjection", func(t *testing.T) {
		records := plan(t, nil, create("AR1"))
		state := InitialState("demo")
		state.Stage, state.Condition, state.CurrentRequest = StageDesign, ConditionWaitingApproval, "AR1"
		state.Status = ProjectStatus(StageCondition{Stage: state.Stage, Condition: state.Condition, CurrentRequest: state.CurrentRequest})
		state.Phase = PhaseForStatus(state.Status)
		if err := state.Validate(); err == nil {
			t.Fatal("state validated a waiting_approval condition whose request does not exist")
		}
		state.Records["approval_request:AR1:0"] = mustJSON(t, StampApprovalRequest(records[0], "abc123"))
		if err := state.Validate(); err != nil {
			t.Fatalf("state.Validate with an open request: %v", err)
		}
		closed := plan(t, records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalWithdrawn})
		state.Records["approval_request:AR1:1"] = mustJSON(t, StampApprovalRequest(closed[1], "abc123"))
		if err := state.Validate(); err == nil {
			t.Fatal("state validated a waiting_approval condition on a withdrawn request")
		}
		events, err := ApprovalRequestHistory(state.Records)
		if err != nil {
			t.Fatalf("ApprovalRequestHistory: %v", err)
		}
		if len(events) != 2 || events[0].Event != "approval_request:requested" || events[1].Event != "approval_request:withdrawn" {
			t.Fatalf("history = %#v, want both transitions in append order", events)
		}
		if events[0].Actor == "" || events[0].GitHead != "abc123" || events[0].SourceRank != HistorySourceApprovalRequest {
			t.Fatalf("history event = %#v, want stamped provenance", events[0])
		}
	})

	t.Run("ClockIndependence", func(t *testing.T) {
		defer func(saved func() time.Time) { Clock = saved }(Clock)
		Clock = func() time.Time { return time.Date(2031, 6, 1, 0, 0, 0, 0, time.UTC) }
		records := plan(t, nil, create("AR1"))
		if _, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalApproved, Pins: pins}); err != nil {
			t.Fatalf("approval before expiry refused: %v", err)
		}
		Clock = func() time.Time { return time.Date(3000, 6, 1, 0, 0, 0, 0, time.UTC) }
		if _, _, err := PlanApprovalRequest(records, ApprovalRequestRecord{ID: "AR1", Transition: ApprovalApproved, Pins: pins}); err == nil {
			t.Fatal("approval after expiry accepted")
		}
	})
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}
