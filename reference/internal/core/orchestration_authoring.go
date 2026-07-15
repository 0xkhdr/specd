package core

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Authoring frontier (GAP-1).
//
// The execution DAG covers only the `executing` phase: while a spec is in
// `requirements`, `design`, or `tasks` there are no DAG tasks, so the
// orchestration loop has nothing to dispatch and stalls. The authoring frontier
// closes that gap. For a planning-phase spec whose current artifact is absent or
// fails its gate, SenseOrchestration surfaces a single synthetic authoring work
// item; DecideOrchestration dispatches an authoring mission (a PinkyMission with
// a reserved A<n> work ID and `specd check` as its verify command) under the
// `planning`/`session` policies, and advances the phase once the artifact passes.
//
// Authoring never mutates the DAG and never bypasses a gate: completion is
// detected by re-sensing the artifact against the live `specd check` gates, so
// the evidence invariant (a phase advances only when its gate truly passes) is
// preserved.

// authoringStep maps a planning status to its reserved work ID, target artifact,
// gate label, and worker role.
type authoringStep struct {
	WorkID   string
	Artifact string
	Gate     string
	Role     string
}

// authoringSteps enumerates the planning artifacts in phase order. The work IDs
// (A1/A2/A3) are reserved authoring identifiers — distinct from execution task
// IDs (T<n>) — and flow through the existing ACP/lease/mission machinery
// unchanged (see acpTaskIDRE).
var authoringSteps = map[SpecStatus]authoringStep{
	StatusRequirements: {WorkID: "A1", Artifact: "requirements.md", Gate: "ears", Role: "craftsman"},
	StatusDesign:       {WorkID: "A2", Artifact: "design.md", Gate: "design", Role: "craftsman"},
	StatusTasks:        {WorkID: "A3", Artifact: "tasks.md", Gate: "task-schema, dag, traceability", Role: "craftsman"},
}

// authoringArtifactID returns the reserved work ID for a planning artifact, or
// "" if the name is not a recognized planning artifact. Used by decision
// validation to reject authoring decisions that name an unknown artifact.
func authoringArtifactID(artifact string) string {
	for _, step := range authoringSteps {
		if step.Artifact == artifact {
			return step.WorkID
		}
	}
	return ""
}

// senseAuthoring inspects a planning-phase spec and returns its authoring
// frontier plus whether the current artifact already passes its gate
// (planningReady). For non-planning statuses it returns (nil, false): the
// execution DAG owns those phases.
func senseAuthoring(status SpecStatus, reqMd, designMd *string, doc ParsedTasks) (*OrchestrationAuthoring, bool) {
	step, planning := authoringSteps[status]
	if !planning {
		return nil, false
	}
	issues := PhaseReadiness(status, reqMd, designMd, doc)
	if len(issues) == 0 {
		// Artifact satisfies its gate — nothing to author; phase is ready to
		// advance.
		return nil, true
	}
	return &OrchestrationAuthoring{
		WorkID:   step.WorkID,
		Artifact: step.Artifact,
		Gate:     step.Gate,
		Role:     step.Role,
		Issues:   issues,
	}, false
}

// BuildAuthoringMission renders a PinkyMission for a planning-phase artifact. It
// is the artifact-mission counterpart of BuildPinkyMission: there is no DAG task,
// so the contract and acceptance are sourced from the live authoring brief
// (NewAuthoringBrief, itself derived from the gates) and the verify command is
// `specd check <spec>` — the same gate the brain re-senses to detect completion.
func BuildAuthoringMission(root, slug, sessionID, workerID, artifact string, cfg OrchestrationCfg) (PinkyMission, error) {
	step, ok := stepForArtifact(artifact)
	if !ok {
		return PinkyMission{}, fmt.Errorf("pinky: %q is not a planning artifact", artifact)
	}
	loaded, err := LoadSpec(root, slug)
	if err != nil {
		return PinkyMission{}, err
	}
	deadline := Clock().UTC().Add(time.Duration(cfg.Transport.MessageTTLSeconds) * time.Second)
	brief := NewAuthoringBrief("")

	mission := PinkyMission{
		Version:        OrchestrationModelVersion,
		SessionID:      sessionID,
		WorkerID:       workerID,
		Spec:           slug,
		TaskID:         step.WorkID,
		Attempt:        1,
		Deadline:       deadline.Format(time.RFC3339Nano),
		HeartbeatEvery: cfg.Transport.HeartbeatSeconds,
		Role:           step.Role,
		Title:          fmt.Sprintf("Author %s for %s", step.Artifact, slug),
		ContextCommand: fmt.Sprintf("specd context %s", slug),
		Contract:       authoringContract(step, brief),
		Files:          []string{filepath.ToSlash(filepath.Join(".specd", "specs", slug, step.Artifact))},
		Acceptance:     fmt.Sprintf("`specd check %s` passes — %s clears the '%s' gate", slug, step.Artifact, step.Gate),
		VerifyCommand:  fmt.Sprintf("specd check %s", slug),
		Dependencies:   []string{},
		Requirements:   []int{},
		Authority: ACPAuthority{
			ReadOnly:       false,
			AllowedActions: pinkyAllowedActions(step.Role),
		},
	}
	_ = loaded // LoadSpec validates the spec exists; its body is not needed here.
	mission.ContextManifest = BuildMissionContextManifest(mission, specArtifactReader(root, slug))
	mission.DispatchDigest = pinkyMissionDigest(mission)
	if err := validatePinkyMission(mission); err != nil {
		return PinkyMission{}, err
	}
	return mission, nil
}

func stepForArtifact(artifact string) (authoringStep, bool) {
	for _, step := range authoringSteps {
		if step.Artifact == artifact {
			return step, true
		}
	}
	return authoringStep{}, false
}

// authoringContract renders the mission contract for one artifact: the gate
// constraints the author must satisfy, sourced from the live brief so it can
// never drift from `specd check`.
func authoringContract(step authoringStep, brief AuthoringBrief) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Author %s so that `specd check` passes.", step.Artifact)
	for _, a := range brief.Artifacts {
		if a.Artifact != step.Artifact {
			continue
		}
		sb.WriteString(" Constraints: ")
		sb.WriteString(strings.Join(a.Constraints, "; "))
	}
	return sb.String()
}

// AdvancePlanningPhase ratchets a planning-phase spec to the next status when its
// current artifact passes its gate. It is the orchestration-loop counterpart of
// `specd approve`'s Case 3 planning ratchet: same readiness check, same
// PlanningAdvance table, so autonomy can never advance a phase the CLI gate would
// reject. It fails closed if readiness does not pass.
func AdvancePlanningPhase(root, slug string) (from, to SpecStatus, err error) {
	loaded, err := LoadSpec(root, slug)
	if err != nil {
		return "", "", err
	}
	state := loaded.State
	advance, ok := PlanningAdvance[state.Status]
	if !ok {
		return "", "", fmt.Errorf("orchestration: nothing to advance — spec %q is %q", slug, state.Status)
	}
	problems := PhaseReadiness(state.Status, ReadArtifact(root, slug, "requirements.md"), ReadArtifact(root, slug, "design.md"), loaded.Doc)
	if len(problems) > 0 {
		sort.Strings(problems)
		return "", "", fmt.Errorf("orchestration: cannot advance %q — gate not satisfied: %s", state.Status, strings.Join(problems, "; "))
	}
	from = state.Status
	state.Status = advance.Status
	state.Phase = advance.Phase
	if err := SaveState(root, slug, state); err != nil {
		return "", "", err
	}
	return from, advance.Status, nil
}
