package core

import (
	"fmt"
	"time"
)

type PinkyProgressReport struct {
	SessionID string
	WorkerID  string
	Spec      string
	TaskID    string
	Attempt   int
	Percent   int
	Message   string
}

type PinkyBlockerReport struct {
	SessionID string
	WorkerID  string
	Spec      string
	TaskID    string
	Attempt   int
	Reason    string
}

type PinkyTerminalReport struct {
	SessionID       string
	WorkerID        string
	Spec            string
	TaskID          string
	Attempt         int
	VerificationRef string
	Summary         string
	ChangedFiles    []string
	GitHead         string
	DurationMs      int64
	HostTokens      int
	HostCost        string
}

func RecordPinkyProgress(root string, report PinkyProgressReport, cfg OrchestrationCfg) (ACPEnvelope, error) {
	payload := ACPProgressPayload{Percent: report.Percent, Message: report.Message}
	return appendPinkyEvent(root, report.SessionID, report.WorkerID, report.Spec, report.TaskID, report.Attempt, ACPMessageProgress, payload, cfg)
}

func RecordPinkyBlocker(root string, report PinkyBlockerReport, cfg OrchestrationCfg) (ACPEnvelope, error) {
	payload := ACPBlockerPayload{Reason: report.Reason}
	return appendPinkyEvent(root, report.SessionID, report.WorkerID, report.Spec, report.TaskID, report.Attempt, ACPMessageBlocker, payload, cfg)
}

func RecordPinkyTerminalReport(root string, report PinkyTerminalReport, cfg OrchestrationCfg) (ACPEnvelope, error) {
	payload := ACPEvidencePayload{
		VerificationRef: report.VerificationRef,
		Summary:         report.Summary,
		ChangedFiles:    append([]string{}, report.ChangedFiles...),
		GitHead:         report.GitHead,
		DurationMs:      report.DurationMs,
		HostTokens:      report.HostTokens,
		HostCost:        report.HostCost,
	}
	return appendPinkyEvent(root, report.SessionID, report.WorkerID, report.Spec, report.TaskID, report.Attempt, ACPMessageEvidence, payload, cfg)
}

func AcknowledgePinkyCancellation(root, sessionID, workerID, spec, taskID string, attempt int, reason string, cfg OrchestrationCfg) (ACPEnvelope, error) {
	payload := ACPCancelledPayload{Reason: reason}
	return appendPinkyEvent(root, sessionID, workerID, spec, taskID, attempt, ACPMessageCancelled, payload, cfg)
}

func appendPinkyEvent(root, sessionID, workerID, spec, taskID string, attempt int, messageType ACPMessageType, payload any, cfg OrchestrationCfg) (ACPEnvelope, error) {
	store, err := NewACPStore(root)
	if err != nil {
		return ACPEnvelope{}, err
	}
	if err := store.ValidateActiveLease(sessionID, workerID, spec, taskID, attempt); err != nil {
		return ACPEnvelope{}, err
	}
	if messageType == ACPMessageEvidence {
		events, err := store.readAllEvents(sessionID)
		if err != nil {
			return ACPEnvelope{}, err
		}
		for _, event := range events {
			if event.Type == ACPMessageEvidence && event.From == "pinky-"+workerID && event.Spec == spec && event.Task == taskID && event.Attempt == attempt {
				return event, nil
			}
		}
	}
	envelope, err := NewACPEnvelope(messageType, payload)
	if err != nil {
		return ACPEnvelope{}, err
	}
	messageID, err := NewACPID()
	if err != nil {
		return ACPEnvelope{}, err
	}
	now := Clock().UTC()
	envelope.MessageID = messageID
	envelope.SessionID = sessionID
	envelope.CreatedAt = now.Format(time.RFC3339Nano)
	envelope.ExpiresAt = now.Add(time.Duration(cfg.Transport.MessageTTLSeconds) * time.Second).Format(time.RFC3339Nano)
	envelope.From = "pinky-" + workerID
	envelope.To = "brain"
	envelope.Spec = spec
	envelope.Task = taskID
	envelope.Attempt = attempt
	written, err := store.WriteEvent(envelope)
	if err != nil {
		return ACPEnvelope{}, fmt.Errorf("pinky report: append event: %w", err)
	}
	return written, nil
}
