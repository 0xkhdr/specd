package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type OrchestrationStepResult struct {
	Snapshot OrchestrationSnapshot `json:"snapshot"`
	Decision OrchestrationDecision `json:"decision"`
	Event    *ACPEnvelope          `json:"event,omitempty"`
}

func StepOrchestration(root, slug, sessionID string, policy OrchestrationPolicy, cfg OrchestrationCfg) (OrchestrationStepResult, error) {
	var result OrchestrationStepResult
	_, err := WithSpecLock[struct{}](root, slug, func() (struct{}, error) {
		snapshot, err := SenseOrchestration(root, slug, sessionID, policy)
		if err != nil {
			return struct{}{}, err
		}
		decision, err := DecideOrchestration(snapshot, policy)
		if err != nil {
			return struct{}{}, err
		}
		result.Snapshot = snapshot
		result.Decision = decision
		event, err := recordOrchestrationDecision(root, sessionID, decision, cfg)
		if err != nil {
			return struct{}{}, err
		}
		result.Event = event
		return struct{}{}, nil
	})
	return result, err
}

func recordOrchestrationDecision(root, sessionID string, decision OrchestrationDecision, cfg OrchestrationCfg) (*ACPEnvelope, error) {
	if err := ValidateOrchestrationDecision(decision); err != nil {
		return nil, err
	}
	switch decision.Action {
	case OrchestrationDispatch, OrchestrationRetry, OrchestrationCancel:
	default:
		return nil, nil
	}
	store, err := NewACPStore(root)
	if err != nil {
		return nil, err
	}
	events, err := store.readAllEvents(sessionID)
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		if event.MessageID == orchestrationDecisionMessageID(decision) {
			return &event, nil
		}
	}
	var messageType ACPMessageType
	var payload any
	switch decision.Action {
	case OrchestrationDispatch, OrchestrationRetry:
		mission, err := BuildPinkyMission(root, decision.Spec, sessionID, orchestrationWorkerID(decision), decision.TaskID, decision.Attempt, cfg)
		if err != nil {
			return nil, err
		}
		messageType = ACPMessageMission
		payload = ACPMissionPayload{
			DispatchDigest: mission.DispatchDigest,
			Role:           mission.Role,
			ContextCommand: mission.ContextCommand,
			Contract:       mission.Contract,
			Files:          append([]string{}, mission.Files...),
			Acceptance:     mission.Acceptance,
			VerifyCommand:  mission.VerifyCommand,
			Dependencies:   append([]string{}, mission.Dependencies...),
			Authority:      mission.Authority,
		}
	case OrchestrationCancel:
		messageType = ACPMessageDirective
		payload = ACPDirectivePayload{Action: "cancel", Reason: decision.Reason}
	default:
		return nil, nil
	}
	envelope, err := NewACPEnvelope(messageType, payload)
	if err != nil {
		return nil, err
	}
	now := Clock().UTC()
	envelope.MessageID = orchestrationDecisionMessageID(decision)
	envelope.SessionID = sessionID
	envelope.CreatedAt = now.Format(time.RFC3339Nano)
	envelope.ExpiresAt = now.Add(time.Duration(cfg.Transport.MessageTTLSeconds) * time.Second).Format(time.RFC3339Nano)
	envelope.From = "brain"
	envelope.To = "pinky-" + orchestrationWorkerID(decision)
	envelope.Spec = decision.Spec
	envelope.Task = decision.TaskID
	envelope.Attempt = decision.Attempt
	written, err := store.WriteEvent(envelope)
	if err != nil {
		return nil, fmt.Errorf("orchestration engine: record decision: %w", err)
	}
	return &written, nil
}

func orchestrationDecisionMessageID(decision OrchestrationDecision) string {
	sum := sha256.Sum256([]byte(decision.IdempotencyKey))
	return hex.EncodeToString(sum[:16])
}

func orchestrationWorkerID(decision OrchestrationDecision) string {
	if decision.Attempt < 1 {
		return ""
	}
	return fmt.Sprintf("%s-a%d", strings.ToLower(decision.TaskID), decision.Attempt)
}
