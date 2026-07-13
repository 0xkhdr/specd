package orchestration

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestA2ARoundTripRequiredPins(t *testing.T) {
	m := validMission()
	m.DeclaredFiles = []string{"z.go", "a.go"}
	m.Acceptance = []string{"R6.2", "R6.1"}

	raw, err := ExportA2A(A2AKindMission, m, A2ATransport{MessageID: "transport-1"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ImportA2A(raw)
	if err != nil {
		t.Fatal(err)
	}
	mission, ok := got.Payload.(MissionV1)
	if !ok {
		t.Fatalf("payload type = %T", got.Payload)
	}
	if MissionDigest(mission) != MissionDigest(m) || mission.ContextDigest != m.ContextDigest || mission.PolicyDigest != m.PolicyDigest || mission.SubjectHead != m.SubjectHead {
		t.Fatalf("mission pins changed: %+v", mission)
	}
	if got.Transport.MessageID != "transport-1" {
		t.Fatalf("transport metadata lost: %+v", got.Transport)
	}
}

func TestA2ACanonicalSerialization(t *testing.T) {
	left, right := validMission(), validMission()
	left.DeclaredFiles, right.DeclaredFiles = []string{"z.go", "a.go"}, []string{"a.go", "z.go"}
	left.Acceptance, right.Acceptance = []string{"R6.2", "R6.1"}, []string{"R6.1", "R6.2"}
	a, err := ExportA2A(A2AKindMission, left, A2ATransport{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := ExportA2A(A2AKindMission, right, A2ATransport{})
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("semantic mission order changed export:\n%s\n%s", a, b)
	}
}

func TestA2ALifecycleMappings(t *testing.T) {
	m := validMission()
	now := m.IssuedAt.Add(time.Second)
	cases := []struct {
		kind    string
		payload any
	}{
		{A2AKindClaim, A2AClaimV1{ProtocolVersion: A2AProtocolVersion, Worker: WorkerV1{WorkerID: "w1", Host: "fake", Roles: []string{m.Role}, Capabilities: []string{"sandbox"}}, Echo: ClaimEcho{MissionID: m.MissionID, TaskID: m.TaskID, Role: m.Role, ContextDigest: m.ContextDigest, ConfigDigest: m.ConfigDigest, PaletteDigest: m.PaletteDigest, AuthorityRef: m.AuthorityRef, SubjectHead: m.SubjectHead}, RequestedLeaseSeconds: 60}},
		{A2AKindHeartbeat, HeartbeatV1{LeaseID: "l1", MissionID: m.MissionID, WorkerID: "w1", Attempt: 1, At: now}},
		{A2AKindCancel, A2ACancelV1{ProtocolVersion: A2AProtocolVersion, MissionID: m.MissionID, LeaseID: "l1", WorkerID: "w1", Attempt: 1, Reason: "operator", At: now}},
		{A2AKindReport, WorkerReportV1{MissionID: m.MissionID, LeaseID: "l1", WorkerID: "w1", TaskID: m.TaskID, Attempt: 1, Role: m.Role, SubjectHead: m.SubjectHead, VerifyRef: "evidence#T1", Status: "complete"}},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			raw, err := ExportA2A(tc.kind, tc.payload, A2ATransport{Adapter: "fake"})
			if err != nil {
				t.Fatal(err)
			}
			got, err := ImportA2A(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind != tc.kind {
				t.Fatalf("kind = %q", got.Kind)
			}
		})
	}
}

func TestA2AUnknownVersionAndSensitiveDataFailClosed(t *testing.T) {
	m := validMission()
	raw, err := ExportA2A(A2AKindMission, m, A2ATransport{})
	if err != nil {
		t.Fatal(err)
	}
	badVersion := strings.Replace(string(raw), `"protocol_version":"1"`, `"protocol_version":"99"`, 1)
	if _, err := ImportA2A([]byte(badVersion)); err == nil || !strings.Contains(err.Error(), "A2A_VERSION_UNSUPPORTED") {
		t.Fatalf("unknown version error = %v", err)
	}
	if _, err := ImportA2A([]byte(`{"protocol_version":"1","kind":"report","payload":{"mission_id":"m","secret":"token"}}`)); err == nil {
		t.Fatal("secret/unknown payload field accepted")
	}
	if _, err := ExportA2A(A2AKindCancel, A2ACancelV1{ProtocolVersion: A2AProtocolVersion, MissionID: "m", LeaseID: "l", WorkerID: "w", Attempt: 1, Reason: "password=hunter2", At: time.Now()}, A2ATransport{}); err == nil {
		t.Fatal("sensitive cancellation reason exported")
	}
}

func TestA2ATransportMetadataDoesNotChangeSemanticACP(t *testing.T) {
	m := validMission()
	var events []ACPEvent
	for _, transport := range []A2ATransport{{Adapter: "mcp", MessageID: "one"}, {Adapter: "a2a", MessageID: "two"}} {
		raw, err := ExportA2A(A2AKindMission, m, transport)
		if err != nil {
			t.Fatal(err)
		}
		message, err := ImportA2A(raw)
		if err != nil {
			t.Fatal(err)
		}
		event, err := A2ASemanticACP(message)
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if !reflect.DeepEqual(events[0], events[1]) {
		t.Fatalf("transport changed ACP semantics:\n%+v\n%+v", events[0], events[1])
	}
}
