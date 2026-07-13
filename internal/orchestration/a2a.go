package orchestration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

const A2AProtocolVersion = "1"

const (
	A2AKindMission   = "mission"
	A2AKindClaim     = "claim"
	A2AKindHeartbeat = "heartbeat"
	A2AKindCancel    = "cancel"
	A2AKindReport    = "report"
)

// A2ATransport carries non-semantic delivery facts. It is deliberately kept
// outside Payload so changing adapters cannot change mission or ACP identity.
type A2ATransport struct {
	Adapter   string `json:"adapter,omitempty"`
	MessageID string `json:"message_id,omitempty"`
}

type A2AClaimV1 struct {
	ProtocolVersion       string    `json:"protocol_version"`
	Worker                WorkerV1  `json:"worker"`
	Echo                  ClaimEcho `json:"echo"`
	RequestedLeaseSeconds int       `json:"requested_lease_seconds"`
}

type A2ACancelV1 struct {
	ProtocolVersion string    `json:"protocol_version"`
	MissionID       string    `json:"mission_id"`
	LeaseID         string    `json:"lease_id"`
	WorkerID        string    `json:"worker_id"`
	Attempt         int       `json:"attempt"`
	Reason          string    `json:"reason"`
	At              time.Time `json:"at"`
}

type A2AEnvelope struct {
	ProtocolVersion string          `json:"protocol_version"`
	Kind            string          `json:"kind"`
	Payload         json.RawMessage `json:"payload"`
	Transport       A2ATransport    `json:"transport,omitempty"`
}

type A2AMessage struct {
	Kind      string
	Payload   any
	Transport A2ATransport
}

func ExportA2A(kind string, payload any, transport A2ATransport) ([]byte, error) {
	if err := validateA2APayload(kind, payload); err != nil {
		return nil, err
	}
	payload = canonicalA2APayload(payload)
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("A2A_PAYLOAD_ENCODE: %w", err)
	}
	if err := rejectSensitiveA2A(raw); err != nil {
		return nil, err
	}
	return json.Marshal(A2AEnvelope{ProtocolVersion: A2AProtocolVersion, Kind: kind, Payload: raw, Transport: transport})
}

func canonicalA2APayload(payload any) any {
	switch value := payload.(type) {
	case MissionV1:
		CanonicalizeMission(&value)
		return value
	case A2AClaimV1:
		value.Worker.Roles = append([]string(nil), value.Worker.Roles...)
		value.Worker.Capabilities = append([]string(nil), value.Worker.Capabilities...)
		sort.Strings(value.Worker.Roles)
		sort.Strings(value.Worker.Capabilities)
		return value
	default:
		return payload
	}
}

func ImportA2A(raw []byte) (A2AMessage, error) {
	if err := rejectSensitiveA2A(raw); err != nil {
		return A2AMessage{}, err
	}
	var envelope A2AEnvelope
	if err := decodeStrict(raw, &envelope); err != nil {
		return A2AMessage{}, fmt.Errorf("A2A_ENVELOPE_INVALID: %w", err)
	}
	if envelope.ProtocolVersion != A2AProtocolVersion {
		return A2AMessage{}, fmt.Errorf("A2A_VERSION_UNSUPPORTED: %s", envelope.ProtocolVersion)
	}
	var payload any
	switch envelope.Kind {
	case A2AKindMission:
		payload = &MissionV1{}
	case A2AKindClaim:
		payload = &A2AClaimV1{}
	case A2AKindHeartbeat:
		payload = &HeartbeatV1{}
	case A2AKindCancel:
		payload = &A2ACancelV1{}
	case A2AKindReport:
		payload = &WorkerReportV1{}
	default:
		return A2AMessage{}, fmt.Errorf("A2A_KIND_UNSUPPORTED: %s", envelope.Kind)
	}
	if err := decodeStrict(envelope.Payload, payload); err != nil {
		return A2AMessage{}, fmt.Errorf("A2A_PAYLOAD_INVALID: %w", err)
	}
	payload = dereferenceA2APayload(payload)
	if err := validateA2APayload(envelope.Kind, payload); err != nil {
		return A2AMessage{}, err
	}
	return A2AMessage{Kind: envelope.Kind, Payload: payload, Transport: envelope.Transport}, nil
}

// SemanticACP projects any supported transport payload onto canonical ACP
// semantics. Transport metadata never enters returned event.
func SemanticACP(kind string, payload any) (ACPEvent, error) {
	if err := validateA2APayload(kind, payload); err != nil {
		return ACPEvent{}, err
	}
	payload = canonicalA2APayload(payload)
	raw, err := json.Marshal(payload)
	if err != nil {
		return ACPEvent{}, fmt.Errorf("A2A_PAYLOAD_ENCODE: %w", err)
	}
	event := ACPEvent{Payload: string(raw)}
	switch value := payload.(type) {
	case MissionV1:
		event.Kind, event.MissionID, event.TaskID, event.Attempt, event.GitHead = ACPKindDispatch, value.MissionID, value.TaskID, value.Attempt, value.SubjectHead
	case A2AClaimV1:
		event.Kind, event.MissionID, event.TaskID = ACPKindClaim, value.Echo.MissionID, value.Echo.TaskID
	case HeartbeatV1:
		event.Kind, event.MissionID, event.Attempt = A2AKindHeartbeat, value.MissionID, value.Attempt
	case A2ACancelV1:
		event.Kind, event.MissionID, event.Attempt = ACPKindCancel, value.MissionID, value.Attempt
	case WorkerReportV1:
		event.Kind, event.MissionID, event.TaskID, event.Attempt, event.GitHead, event.VerifyRef = ACPKindReport, value.MissionID, value.TaskID, value.Attempt, value.SubjectHead, value.VerifyRef
	default:
		return ACPEvent{}, fmt.Errorf("A2A_PAYLOAD_TYPE_INVALID: %s", kind)
	}
	return event, nil
}

func A2ASemanticACP(message A2AMessage) (ACPEvent, error) {
	return SemanticACP(message.Kind, message.Payload)
}

func validateA2APayload(kind string, payload any) error {
	switch kind {
	case A2AKindMission:
		m, ok := payload.(MissionV1)
		if !ok {
			return fmt.Errorf("A2A_PAYLOAD_TYPE_INVALID: mission")
		}
		return ValidateMission(m)
	case A2AKindClaim:
		c, ok := payload.(A2AClaimV1)
		if !ok || c.ProtocolVersion != A2AProtocolVersion || c.Worker.WorkerID == "" || c.Worker.Host == "" || c.Echo.MissionID == "" || c.Echo.TaskID == "" || c.Echo.Role == "" || c.Echo.ContextDigest == "" || c.Echo.ConfigDigest == "" || c.Echo.PaletteDigest == "" || c.Echo.AuthorityRef == "" || c.Echo.SubjectHead == "" || c.RequestedLeaseSeconds < 1 {
			return fmt.Errorf("A2A_CLAIM_INVALID")
		}
	case A2AKindHeartbeat:
		h, ok := payload.(HeartbeatV1)
		if !ok || h.LeaseID == "" || h.MissionID == "" || h.WorkerID == "" || h.Attempt < 1 || h.At.IsZero() {
			return fmt.Errorf("A2A_HEARTBEAT_INVALID")
		}
	case A2AKindCancel:
		c, ok := payload.(A2ACancelV1)
		if !ok || c.ProtocolVersion != A2AProtocolVersion || c.MissionID == "" || c.LeaseID == "" || c.WorkerID == "" || c.Attempt < 1 || c.Reason == "" || c.At.IsZero() {
			return fmt.Errorf("A2A_CANCEL_INVALID")
		}
	case A2AKindReport:
		r, ok := payload.(WorkerReportV1)
		if !ok || r.MissionID == "" || r.LeaseID == "" || r.WorkerID == "" || r.TaskID == "" || r.Attempt < 1 || r.Role == "" || r.SubjectHead == "" || r.VerifyRef == "" || (r.Status != "complete" && r.Status != "blocked" && r.Status != "failed") {
			return fmt.Errorf("A2A_REPORT_INVALID")
		}
	default:
		return fmt.Errorf("A2A_KIND_UNSUPPORTED: %s", kind)
	}
	return nil
}

func dereferenceA2APayload(payload any) any {
	switch p := payload.(type) {
	case *MissionV1:
		return *p
	case *A2AClaimV1:
		return *p
	case *HeartbeatV1:
		return *p
	case *A2ACancelV1:
		return *p
	case *WorkerReportV1:
		return *p
	default:
		return payload
	}
}

func decodeStrict(raw []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}

func rejectSensitiveA2A(raw []byte) error {
	lower := strings.ToLower(string(raw))
	for _, marker := range []string{"password=", "secret=", "--token=", "raw_prompt", "raw_source", "chain-of-thought", "hidden_reasoning", "tool_output"} {
		if strings.Contains(lower, marker) {
			return fmt.Errorf("A2A_SENSITIVE_DATA_REFUSED")
		}
	}
	return nil
}
