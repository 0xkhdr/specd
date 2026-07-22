package core

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

func impactCandidateFixture() []ImpactCandidate {
	return []ImpactCandidate{
		{Kind: "task", ID: "T3", Spec: "alpha", Version: "1", State: "completed"},
		{Kind: "task", ID: "T4", Spec: "alpha", Version: "1", State: "completed", DependsOn: []string{ImpactRef("task", "alpha", "T3")}},
		{Kind: "criterion", ID: "C1", Spec: "alpha", Version: "1", State: "satisfied", DependsOn: []string{ImpactRef("task", "alpha", "T4")}},
		{Kind: "review", ID: "R1", Spec: "alpha", Version: "1", State: "approved", DependsOn: []string{ImpactRef("task", "alpha", "T4")}},
		{Kind: "artifact", ID: "design.md", Spec: "alpha", Version: "2", State: "approved", SnapshotRequired: true, DependsOn: []string{ImpactRef("task", "alpha", "T3")}},
		{Kind: "approval", ID: "AR-1", Spec: "alpha", Version: "1", State: "granted", DependsOn: []string{ImpactRef("artifact", "alpha", "design.md")}},
		{Kind: "mission", ID: "M1", Spec: "alpha", Version: "1", State: "active", LeaseHolder: "worker-9", DependsOn: []string{ImpactRef("task", "alpha", "T4")}},
		{Kind: "task", ID: "T9", Spec: "alpha", Version: "1", State: "completed"},
		{Kind: "submission", ID: "S1", Spec: "alpha", Version: "1", State: "accepted", DependsOn: []string{ImpactRef("task", "alpha", "T4")}},
		{Kind: "program_link", ID: "alpha->beta", Spec: "beta", Version: "1", State: "linked", DependsOn: []string{ImpactRef("task", "alpha", "T4")}},
		{Kind: "task", ID: "B2", Spec: "beta", Version: "1", State: "completed", DependsOn: []string{ImpactRef("program_link", "beta", "alpha->beta")}},
	}
}

func impactInputFixture() ImpactInput {
	return ImpactInput{
		Operation:             ImpactOperationReopen,
		RequestedKind:         "task",
		RequestedID:           "T3",
		RequestedSpec:         "alpha",
		RequestedVersion:      "1",
		ExpectedStateRevision: 42,
		Actor:                 ActorAgent,
		ActorID:               "actor-1",
		Candidates:            impactCandidateFixture(),
	}
}

func impactEntity(t *testing.T, plan ImpactPlan, kind, spec, id string) ImpactEntity {
	t.Helper()
	for _, entity := range plan.Entities {
		if entity.Kind == kind && entity.Spec == spec && entity.ID == id {
			return entity
		}
	}
	t.Fatalf("entity %s not previewed", ImpactRef(kind, spec, id))
	return ImpactEntity{}
}

func impactBlocked(plan ImpactPlan, code string) bool {
	for _, blocker := range plan.Blockers {
		if blocker.Code == code {
			return true
		}
	}
	return false
}

func TestImpactPlanDeterminism(t *testing.T) {
	base := impactInputFixture()

	t.Run("byte_stable_across_permuted_input", func(t *testing.T) {
		permuted := base
		permuted.Candidates = slices.Clone(base.Candidates)
		slices.Reverse(permuted.Candidates)

		first, err := json.Marshal(BuildImpactPlan(base))
		if err != nil {
			t.Fatal(err)
		}
		second, err := json.Marshal(BuildImpactPlan(permuted))
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(second) {
			t.Fatalf("plan is not byte stable:\n%s\n%s", first, second)
		}
	})

	t.Run("digest_pins_content_and_clears_itself", func(t *testing.T) {
		plan := BuildImpactPlan(base)
		if plan.ImpactDigest == "" {
			t.Fatal("impact digest is empty")
		}
		if plan.ImpactDigest != impactPlanDigest(plan) {
			t.Fatal("digest is not computed with the digest field cleared")
		}
		changed := base
		changed.ExpectedStateRevision = 43
		if BuildImpactPlan(changed).ImpactDigest == plan.ImpactDigest {
			t.Fatal("digest ignored a content change")
		}
	})

	t.Run("entities_are_totally_ordered", func(t *testing.T) {
		plan := BuildImpactPlan(base)
		if len(plan.Entities) != len(base.Candidates) {
			t.Fatalf("previewed %d entities, want %d", len(plan.Entities), len(base.Candidates))
		}
		keys := make([]string, 0, len(plan.Entities))
		for _, entity := range plan.Entities {
			keys = append(keys, ImpactRef(entity.Kind, entity.Spec, entity.ID))
		}
		if !slices.IsSorted(keys) {
			t.Fatalf("entities are not in canonical order: %v", keys)
		}
	})

	t.Run("every_entity_is_classified_exactly_once", func(t *testing.T) {
		legal := []string{ImpactCurrent, ImpactStale, ImpactReopened, ImpactRetained, ImpactSuperseded, ImpactCancelled, ImpactForbidden}
		for _, entity := range BuildImpactPlan(base).Entities {
			if !slices.Contains(legal, entity.Classification) {
				t.Fatalf("entity %s has illegal classification %q", entity.ID, entity.Classification)
			}
			if entity.Reason == "" {
				t.Fatalf("entity %s has no reason", entity.ID)
			}
		}
	})
}

func TestImpactPlanClassification(t *testing.T) {
	plan := BuildImpactPlan(impactInputFixture())

	t.Run("requested_entity_is_reopened", func(t *testing.T) {
		if got := impactEntity(t, plan, "task", "alpha", "T3").Classification; got != ImpactReopened {
			t.Fatalf("requested entity classified %q", got)
		}
		if plan.RequestedID != "T3" || plan.ExpectedStateRevision != 42 {
			t.Fatal("plan does not echo the request")
		}
	})

	t.Run("transitive_descendants_are_stale", func(t *testing.T) {
		for _, id := range []string{"T4", "C1", "R1", "AR-1"} {
			var entity ImpactEntity
			switch id {
			case "C1":
				entity = impactEntity(t, plan, "criterion", "alpha", id)
			case "R1":
				entity = impactEntity(t, plan, "review", "alpha", id)
			case "AR-1":
				entity = impactEntity(t, plan, "approval", "alpha", id)
			default:
				entity = impactEntity(t, plan, "task", "alpha", id)
			}
			if entity.Classification != ImpactStale {
				t.Fatalf("%s classified %q, want stale", id, entity.Classification)
			}
			if len(entity.Choices) == 0 {
				t.Fatalf("%s offers no resolution choices", id)
			}
		}
	})

	t.Run("unreachable_entity_stays_current", func(t *testing.T) {
		if got := impactEntity(t, plan, "task", "alpha", "T9").Classification; got != ImpactCurrent {
			t.Fatalf("unreachable entity classified %q", got)
		}
	})

	t.Run("snapshot_lease_and_gates_are_named", func(t *testing.T) {
		if !slices.Contains(plan.Snapshots, "design.md") {
			t.Fatalf("snapshot not required for reopened artifact: %v", plan.Snapshots)
		}
		if len(plan.LeaseActions) != 1 || plan.LeaseActions[0].Action != ImpactLeaseRevoke || plan.LeaseActions[0].Holder != "worker-9" {
			t.Fatalf("lease action wrong: %#v", plan.LeaseActions)
		}
		if !plan.AuthorityRequired {
			t.Fatal("foreign lease holder must require authority")
		}
		for _, gate := range []string{"impact", "evidence", "snapshot", "lease"} {
			if !slices.Contains(plan.Gates, gate) {
				t.Fatalf("gate %q missing from %v", gate, plan.Gates)
			}
		}
	})

	t.Run("retain_supersede_cancel_are_distinct", func(t *testing.T) {
		input := impactInputFixture()
		input.Candidates[1].Retain = true
		input.Candidates[2].SupersededBy = "C2"
		input.Candidates[3].Cancel = true
		dispositioned := BuildImpactPlan(input)
		if got := impactEntity(t, dispositioned, "task", "alpha", "T4").Classification; got != ImpactRetained {
			t.Fatalf("retain classified %q", got)
		}
		if got := impactEntity(t, dispositioned, "criterion", "alpha", "C1").Classification; got != ImpactSuperseded {
			t.Fatalf("supersede classified %q", got)
		}
		if got := impactEntity(t, dispositioned, "review", "alpha", "R1").Classification; got != ImpactCancelled {
			t.Fatalf("cancel classified %q", got)
		}
		if !slices.Contains(dispositioned.Gates, "approval") {
			t.Fatalf("retention must arm the approval gate: %v", dispositioned.Gates)
		}
	})
}

func TestImpactPlanCrossSpec(t *testing.T) {
	t.Run("cross_spec_descendants_are_previewed", func(t *testing.T) {
		plan := BuildImpactPlan(impactInputFixture())
		link := impactEntity(t, plan, "program_link", "beta", "alpha->beta")
		if !link.CrossSpec || link.Classification != ImpactStale {
			t.Fatalf("cross-spec link previewed as %#v", link)
		}
		descendant := impactEntity(t, plan, "task", "beta", "B2")
		if !descendant.CrossSpec || descendant.Classification != ImpactStale {
			t.Fatalf("cross-spec descendant previewed as %#v", descendant)
		}
	})

	t.Run("unavailable_candidate_is_never_current", func(t *testing.T) {
		input := impactInputFixture()
		for i := range input.Candidates {
			if input.Candidates[i].ID == "B2" {
				input.Candidates[i].Unavailable = true
			}
		}
		entity := impactEntity(t, BuildImpactPlan(input), "task", "beta", "B2")
		if entity.Classification != ImpactStale {
			t.Fatalf("unavailable cross-spec candidate classified %q", entity.Classification)
		}
	})

	t.Run("unavailable_unreachable_candidate_is_still_not_current", func(t *testing.T) {
		input := impactInputFixture()
		for i := range input.Candidates {
			if input.Candidates[i].ID == "T9" {
				input.Candidates[i].Unavailable = true
			}
		}
		entity := impactEntity(t, BuildImpactPlan(input), "task", "alpha", "T9")
		if entity.Classification == ImpactCurrent {
			t.Fatal("unavailable candidate must not be classified current")
		}
	})
}

func TestImpactPlanMalformedCycle(t *testing.T) {
	input := impactInputFixture()
	input.Candidates = append(input.Candidates,
		ImpactCandidate{Kind: "task", ID: "X1", Spec: "alpha", DependsOn: []string{ImpactRef("task", "alpha", "T3"), ImpactRef("task", "alpha", "X2")}},
		ImpactCandidate{Kind: "task", ID: "X2", Spec: "alpha", DependsOn: []string{ImpactRef("task", "alpha", "X1")}},
		ImpactCandidate{Kind: "task", ID: "X3", Spec: "alpha", DependsOn: []string{ImpactRef("task", "alpha", "T3"), ImpactRef("task", "alpha", "gone")}},
		ImpactCandidate{Kind: "task", ID: "X4", Spec: "alpha", DependsOn: []string{ImpactRef("task", "alpha", "T3"), ImpactRef("task", "alpha", "X4")}},
	)
	plan := BuildImpactPlan(input)

	t.Run("cycle_terminates_and_blocks", func(t *testing.T) {
		if !impactBlocked(plan, "IMPACT_GRAPH_CYCLIC") {
			t.Fatalf("cyclic graph recorded no blocker: %#v", plan.Blockers)
		}
		for _, id := range []string{"X1", "X2", "X4"} {
			if got := impactEntity(t, plan, "task", "alpha", id).Classification; got != ImpactStale {
				t.Fatalf("cycle member %s classified %q, want stale", id, got)
			}
		}
	})

	t.Run("dangling_dependency_blocks_conservatively", func(t *testing.T) {
		if !impactBlocked(plan, "IMPACT_DEPENDENCY_MISSING") {
			t.Fatalf("dangling dependency recorded no blocker: %#v", plan.Blockers)
		}
		if got := impactEntity(t, plan, "task", "alpha", "X3").Classification; got != ImpactStale {
			t.Fatalf("entity with dangling dependency classified %q", got)
		}
	})

	t.Run("invalid_request_stays_in_blockers", func(t *testing.T) {
		invalid := BuildImpactPlan(ImpactInput{Operation: "delete", ExpectedStateRevision: -1})
		for _, code := range []string{"IMPACT_OPERATION_INVALID", "IMPACT_REQUEST_INCOMPLETE", "IMPACT_REVISION_INVALID", "IMPACT_ACTOR_REQUIRED", "IMPACT_REQUEST_UNKNOWN"} {
			if !impactBlocked(invalid, code) {
				t.Fatalf("missing blocker %s in %#v", code, invalid.Blockers)
			}
		}
	})

	t.Run("duplicate_candidate_blocks", func(t *testing.T) {
		duplicated := impactInputFixture()
		duplicated.Candidates = append(duplicated.Candidates, duplicated.Candidates[0])
		if !impactBlocked(BuildImpactPlan(duplicated), "IMPACT_CANDIDATE_DUPLICATE") {
			t.Fatal("duplicate candidate recorded no blocker")
		}
	})
}

func TestImpactPlanImmutableConsumption(t *testing.T) {
	input := impactInputFixture()
	for i := range input.Candidates {
		switch input.Candidates[i].ID {
		case "T4":
			input.Candidates[i].Consumptions = []ImpactConsumption{
				{Record: "rel-7", Kind: "release", External: true},
				{Record: "att-1", Kind: "attempt"},
			}
		case "S1":
			input.Candidates[i].Consumptions = []ImpactConsumption{{Record: "dep-3", Kind: "deployment", External: true}}
			input.Candidates[i].SnapshotRequired = true
		}
	}
	plan := BuildImpactPlan(input)

	t.Run("external_consumption_forbids_in_place_repair", func(t *testing.T) {
		entity := impactEntity(t, plan, "task", "alpha", "T4")
		if entity.Classification != ImpactForbidden {
			t.Fatalf("externally consumed entity classified %q", entity.Classification)
		}
		if entity.SuccessorRoute == "" {
			t.Fatal("forbidden entity offers no linked-successor route")
		}
		if entity.Consumptions[1].Record != "rel-7" {
			t.Fatalf("consuming record not named: %#v", entity.Consumptions)
		}
		if entity.Snapshot {
			t.Fatal("forbidden entity must not request a snapshot")
		}
	})

	t.Run("internal_consumption_is_not_forbidden", func(t *testing.T) {
		clean := BuildImpactPlan(impactInputFixture())
		if got := impactEntity(t, clean, "task", "alpha", "T4").Classification; got != ImpactStale {
			t.Fatalf("unconsumed descendant classified %q", got)
		}
	})

	t.Run("plan_rolls_up_consuming_records", func(t *testing.T) {
		records := make([]string, 0, len(plan.Consumptions))
		for _, consumption := range plan.Consumptions {
			records = append(records, consumption.Record)
		}
		for _, want := range []string{"att-1", "dep-3", "rel-7"} {
			if !slices.Contains(records, want) {
				t.Fatalf("consumption %q missing from %v", want, records)
			}
		}
		if !plan.AuthorityRequired {
			t.Fatal("forbidden impact must require authority")
		}
	})
}

func TestImpactPlanRevisionRace(t *testing.T) {
	preview := BuildImpactPlan(impactInputFixture())

	t.Run("unchanged_preview_commits", func(t *testing.T) {
		if err := GuardImpactCommit(preview, 42, BuildImpactPlan(impactInputFixture())); err != nil {
			t.Fatalf("stable preview refused: %v", err)
		}
	})

	t.Run("moved_revision_refuses_with_recovery", func(t *testing.T) {
		err := GuardImpactCommit(preview, 43, BuildImpactPlan(impactInputFixture()))
		if !errors.Is(err, ErrImpactStale) {
			t.Fatalf("revision race returned %v", err)
		}
		if !strings.Contains(err.Error(), "re-run the impact preview") || !strings.Contains(err.Error(), "--expect-revision 43") {
			t.Fatalf("refusal does not name the fresh-preview recovery: %v", err)
		}
	})

	t.Run("moved_impact_refuses_with_recovery", func(t *testing.T) {
		fresh := impactInputFixture()
		fresh.Candidates[1].Cancel = true
		err := GuardImpactCommit(preview, 42, BuildImpactPlan(fresh))
		if !errors.Is(err, ErrImpactStale) {
			t.Fatalf("digest race returned %v", err)
		}
		if !strings.Contains(err.Error(), "impact digest moved") || !strings.Contains(err.Error(), "re-run the impact preview") {
			t.Fatalf("refusal does not name the fresh-preview recovery: %v", err)
		}
	})
}
