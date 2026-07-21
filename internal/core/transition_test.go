package core

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestTransitionPlan(t *testing.T) {
	authority := &AuthorityV1{
		ActorID:          "actor-1",
		WorkerID:         "worker-1",
		Role:             "craftsman",
		Mode:             "write",
		PolicyDigest:     "authority-policy",
		BaselineRevision: "42",
		Digest:           "authority-digest",
	}
	base := TransitionInput{
		Current:               StatusExecuting,
		Target:                StatusVerifying,
		StateRevision:         42,
		Actor:                 ActorAgent,
		ActorAssurance:        "host-enforced",
		AuthorityRequired:     true,
		Authority:             authority,
		ArmedGates:            []string{"review", "evidence", "review"},
		Inputs:                map[string]string{"tasks": "tasks-digest", "evidence": "evidence-digest"},
		ArtifactDigests:       map[string]string{"design": "design-digest", "requirements": "requirements-digest"},
		ConfigDigest:          "config-digest",
		PolicyDigest:          "policy-digest",
		TransportCapabilities: []string{"authority", "local", "authority"},
		RequiredTransport:     []string{"local", "authority"},
		Blockers: []TransitionBlocker{
			{Code: "Z", Gate: "review", Message: "last"},
			{Code: "A", Gate: "evidence", Message: "first"},
		},
		Warnings:         []TransitionBlocker{{Code: "WARN", Gate: "review", Message: "warning"}},
		Recoveries:       []TransitionRecovery{{BlockerCode: "Z", Operation: "review", Actor: ActorAgent}},
		ReadinessChecked: true,
	}

	t.Run("byte_stable_and_complete", func(t *testing.T) {
		permuted := base
		permuted.ArmedGates = []string{"evidence", "review"}
		permuted.Inputs = map[string]string{"evidence": "evidence-digest", "tasks": "tasks-digest"}
		permuted.ArtifactDigests = map[string]string{"requirements": "requirements-digest", "design": "design-digest"}
		permuted.TransportCapabilities = []string{"local", "authority"}
		permuted.RequiredTransport = []string{"authority", "local"}
		permuted.Blockers = slices.Clone(base.Blockers)
		slices.Reverse(permuted.Blockers)

		first, err := json.Marshal(BuildTransitionPlan(base))
		if err != nil {
			t.Fatal(err)
		}
		second, err := json.Marshal(BuildTransitionPlan(permuted))
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(second) {
			t.Fatalf("plan is not byte-stable:\n%s\n%s", first, second)
		}

		plan := BuildTransitionPlan(base)
		if plan.PlanDigest == "" || plan.Target != StatusVerifying || plan.StateRevision != 42 || plan.Actor != ActorAgent {
			t.Fatalf("plan identity incomplete: %+v", plan)
		}
		if !plan.Authority.Required || !plan.Authority.Available || plan.Authority.Digest != authority.Digest {
			t.Fatalf("authority projection incomplete: %+v", plan.Authority)
		}
		if len(plan.ArmedGates) != 2 || len(plan.Inputs) != 2 || len(plan.ArtifactDigests) != 2 || len(plan.Recoveries) != 1 {
			t.Fatalf("plan inputs incomplete: %+v", plan)
		}
		if plan.StateChanged || plan.MutationIntent != TransitionMutationAdvanceStatus || !plan.ReadinessChecked {
			t.Fatalf("mutation/readiness fields incorrect: %+v", plan)
		}
	})

	t.Run("terminal_has_no_nominal_transition", func(t *testing.T) {
		input := base
		input.Current = StatusComplete
		input.Target = StatusExecuting
		plan := BuildTransitionPlan(input)
		if !plan.Terminal || plan.Target != "" || plan.MutationIntent != TransitionMutationNone || plan.TerminalReason == "" {
			t.Fatalf("terminal plan advertises a transition: %+v", plan)
		}
		for _, blocker := range plan.Blockers {
			if blocker.Code == "TARGET_INVALID" || blocker.Code == "TARGET_NOT_SUCCESSOR" {
				t.Fatalf("terminal plan treated nominal target as actionable: %+v", plan)
			}
		}
	})

	t.Run("missing_authority_blocks_with_recovery", func(t *testing.T) {
		input := base
		input.Authority = nil
		plan := BuildTransitionPlan(input)
		if !slices.ContainsFunc(plan.Blockers, func(blocker TransitionBlocker) bool { return blocker.Code == "AUTHORITY_REQUIRED" }) {
			t.Fatalf("missing authority was not blocked: %+v", plan.Blockers)
		}
		if !slices.ContainsFunc(plan.Recoveries, func(recovery TransitionRecovery) bool {
			return recovery.BlockerCode == "AUTHORITY_REQUIRED" && recovery.Operation == "context" && recovery.Actor == ActorAgent
		}) {
			t.Fatalf("missing authority has no agent-legal recovery: %+v", plan.Recoveries)
		}
	})

	t.Run("planning_is_mutation_free", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/state.json"
		if err := os.WriteFile(path, []byte("unchanged"), 0o600); err != nil {
			t.Fatal(err)
		}
		beforeInput := base
		beforeInput.ArmedGates = slices.Clone(base.ArmedGates)
		beforeInput.Blockers = slices.Clone(base.Blockers)
		beforeInput.Warnings = slices.Clone(base.Warnings)
		beforeInput.Recoveries = slices.Clone(base.Recoveries)
		beforeInput.Inputs = map[string]string{"tasks": "tasks-digest", "evidence": "evidence-digest"}
		beforeInput.ArtifactDigests = map[string]string{"design": "design-digest", "requirements": "requirements-digest"}
		beforeAuthority := *authority

		_ = BuildTransitionPlan(base)

		if !reflect.DeepEqual(base, beforeInput) || !reflect.DeepEqual(*authority, beforeAuthority) {
			t.Fatalf("planner mutated its input:\nbefore=%+v\nafter=%+v", beforeInput, base)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != "unchanged" {
			t.Fatalf("planner mutated filesystem state: %q", raw)
		}
	})
}
