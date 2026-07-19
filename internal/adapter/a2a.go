package adapter

import (
	"github.com/0xkhdr/specd/internal/orchestration"
)

// MissionFromRequest performs strict payload decoding and verifies duplicated
// common-envelope identity/authority pins before returning mission semantics.
func MissionFromRequest(req Request) (orchestration.MissionV1, error) {
	if err := req.Validate(); err != nil {
		return orchestration.MissionV1{}, err
	}
	if req.Kind != "mission.request" {
		return orchestration.MissionV1{}, newFinding(ErrUnknownKind, "kind", "expected mission.request")
	}
	var mission orchestration.MissionV1
	if err := decode(req.Payload, &mission); err != nil {
		return orchestration.MissionV1{}, err
	}
	if err := orchestration.ValidateMission(mission); err != nil {
		return orchestration.MissionV1{}, err
	}
	if req.Subject.SpecSlug != mission.SpecSlug || req.Subject.TaskID != mission.TaskID || req.Subject.MissionID != mission.MissionID || req.Subject.GitHead != mission.SubjectHead || req.AuthorityRef != mission.AuthorityRef || req.Actor != mission.Role {
		return orchestration.MissionV1{}, newFinding(ErrIdentityMismatch, "mission", "mission payload does not match envelope authority/scope identity")
	}
	orchestration.CanonicalizeMission(&mission)
	return mission, nil
}
