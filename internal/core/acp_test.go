package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestACPIDGeneration(t *testing.T) {
	first, err := NewACPID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewACPID()
	if err != nil {
		t.Fatal(err)
	}
	if !acpIDRE.MatchString(first) || !acpIDRE.MatchString(second) {
		t.Fatalf("generated IDs are invalid: %q %q", first, second)
	}
	if first == second {
		t.Fatalf("generated duplicate IDs: %q", first)
	}
}

func TestACPMessageRoundTrips(t *testing.T) {
	tests := []struct {
		name        string
		messageType ACPMessageType
		task        string
		payload     any
	}{
		{"mission", ACPMessageMission, "T1", validACPMission()},
		{"accepted", ACPMessageAccepted, "T1", ACPAcceptedPayload{WorkerID: "worker-1"}},
		{"heartbeat", ACPMessageHeartbeat, "T1", ACPHeartbeatPayload{WorkerID: "worker-1", Status: "working"}},
		{"progress", ACPMessageProgress, "T1", ACPProgressPayload{Percent: 50, Message: "halfway"}},
		{"evidence", ACPMessageEvidence, "T1", ACPEvidencePayload{VerificationRef: "state:T1:revision:4", Summary: "verified", ChangedFiles: []string{"internal/core/acp.go"}}},
		{"blocker", ACPMessageBlocker, "T1", ACPBlockerPayload{Reason: "dependency unavailable"}},
		{"query", ACPMessageQuery, "T1", ACPQueryPayload{Text: "clarify acceptance"}},
		{"directive", ACPMessageDirective, "T1", ACPDirectivePayload{Action: "retry", Reason: "verification failed"}},
		{"cancelled", ACPMessageCancelled, "T1", ACPCancelledPayload{Reason: "cancel acknowledged"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := validACPEnvelope(t, tt.messageType, tt.task, tt.payload)
			if tt.messageType != ACPMessageMission && tt.messageType != ACPMessageDirective {
				envelope.From, envelope.To = envelope.To, envelope.From
			}
			raw, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseACPEnvelope(raw)
			if err != nil {
				t.Fatalf("ParseACPEnvelope() error = %v\n%s", err, raw)
			}
			again, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != string(again) {
				t.Fatalf("round-trip changed bytes:\n%s\n%s", raw, again)
			}
		})
	}
}

func TestACPValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ACPEnvelope)
		wantErr string
	}{
		{"version", func(e *ACPEnvelope) { e.Version = "2" }, "unsupported version"},
		{"message id", func(e *ACPEnvelope) { e.MessageID = "../escape" }, "invalid messageId"},
		{"session id", func(e *ACPEnvelope) { e.SessionID = "short" }, "invalid sessionId"},
		{"sequence", func(e *ACPEnvelope) { e.Sequence = 0 }, "sequence"},
		{"type", func(e *ACPEnvelope) { e.Type = "unknown" }, "unsupported type"},
		{"from", func(e *ACPEnvelope) { e.From = "../../brain" }, "invalid from"},
		{"to", func(e *ACPEnvelope) { e.To = "broadcast" }, "invalid to"},
		{"direction", func(e *ACPEnvelope) { e.From, e.To = e.To, e.From }, "must be sent from brain"},
		{"slug", func(e *ACPEnvelope) { e.Spec = "../spec" }, "invalid spec"},
		{"task", func(e *ACPEnvelope) { e.Task = "task-1" }, "invalid task"},
		{"attempt", func(e *ACPEnvelope) { e.Attempt = 0 }, "attempt"},
		{"expiry", func(e *ACPEnvelope) { e.ExpiresAt = e.CreatedAt }, "expiresAt"},
		{"role", func(e *ACPEnvelope) {
			payload := validACPMission()
			payload.Role = "wizard"
			e.Payload, _ = json.Marshal(payload)
		}, "invalid role"},
		{"payload unknown field", func(e *ACPEnvelope) {
			e.Payload = json.RawMessage(`{"percent":10,"message":"ok","extra":true}`)
			e.Type = ACPMessageProgress
			e.From, e.To = e.To, e.From
		}, "unknown field"},
		{"payload text size", func(e *ACPEnvelope) {
			e.Payload, _ = json.Marshal(ACPProgressPayload{Percent: 10, Message: strings.Repeat("x", ACPMaxTextBytes+1)})
			e.Type = ACPMessageProgress
			e.From, e.To = e.To, e.From
		}, "exceeds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := validACPEnvelope(t, ACPMessageMission, "T1", validACPMission())
			tt.mutate(&envelope)
			err := ValidateACPEnvelope(envelope)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateACPEnvelope() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestACPEnvelopeSizeLimit(t *testing.T) {
	raw := []byte(`{"padding":"` + strings.Repeat("x", ACPMaxEnvelopeBytes) + `"}`)
	_, err := ParseACPEnvelope(raw)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("ParseACPEnvelope() error = %v, want size rejection", err)
	}
}

func validACPMission() ACPMissionPayload {
	return ACPMissionPayload{
		DispatchDigest: strings.Repeat("a", 64),
		Role:           "builder",
		ContextCommand: "specd context example",
		ContextManifest: MissionContextManifest{
			Version:          missionContextManifestVersion,
			SoftTokenCeiling: missionContextSoftCeiling,
			Strategy:         "load required items first",
			Items: []MissionContextItem{{
				Order:     1,
				Kind:      "role",
				Path:      ".specd/roles/builder.md",
				Mode:      "read-full",
				Required:  true,
				TokenHint: 100,
				Rationale: "role authority and constraints",
			}},
		},
		Contract:      "implement one task",
		Files:         []string{"internal/core/acp.go"},
		Acceptance:    "tests pass",
		VerifyCommand: "go test ./internal/core/...",
		Dependencies:  []string{},
		Authority:     ACPAuthority{AllowedActions: []string{"read", "edit"}},
	}
}

func validACPEnvelope(t *testing.T, messageType ACPMessageType, task string, payload any) ACPEnvelope {
	t.Helper()
	envelope, err := NewACPEnvelope(messageType, payload)
	if err != nil {
		t.Fatal(err)
	}
	envelope.MessageID = strings.Repeat("1", 32)
	envelope.SessionID = strings.Repeat("2", 32)
	envelope.Sequence = 1
	envelope.CreatedAt = time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	envelope.ExpiresAt = time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	envelope.From = "brain"
	envelope.To = "pinky-worker-1"
	envelope.Spec = "example"
	envelope.Task = task
	envelope.Attempt = 1
	return envelope
}
