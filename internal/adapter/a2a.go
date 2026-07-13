package adapter

import (
	"encoding/json"
	"fmt"

	"github.com/0xkhdr/specd/internal/orchestration"
)

// MissionRequest maps specd's canonical mission onto the common adapter
// envelope. Mission payload remains domain-owned while identity and authority
// are duplicated in the common pins so generic adapters can reject mismatch.
func MissionRequest(m orchestration.MissionV1, requestID, correlationID, adapterName string) (Request, error) {
	if err := orchestration.ValidateMission(m); err != nil {
		return Request{}, err
	}
	orchestration.CanonicalizeMission(&m)
	payload, err := json.Marshal(m)
	if err != nil {
		return Request{}, fmt.Errorf("encode mission payload: %w", err)
	}
	req := Request{
		SchemaVersion: SchemaVersion,
		Kind:          "mission.request",
		RequestID:     requestID,
		CorrelationID: correlationID,
		Subject: Subject{
			SpecSlug:  m.SpecSlug,
			TaskID:    m.TaskID,
			MissionID: m.MissionID,
			GitHead:   m.SubjectHead,
		},
		Actor:        m.Role,
		AuthorityRef: m.AuthorityRef,
		Limits: Limits{
			TimeoutMS: int64(m.Limits.TimeoutSeconds) * 1000,
		},
		StartedAt:   m.IssuedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		AdapterName: adapterName,
		Payload:     payload,
	}
	if err := req.Validate(); err != nil {
		return Request{}, err
	}
	return req, nil
}

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
