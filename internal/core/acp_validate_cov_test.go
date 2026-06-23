package core

import (
	"encoding/json"
	"strings"
	"testing"
)

// acp_validate_cov_test.go drives validateACPPayload and its leaf validators
// (text/paths/taskIDs/direction/decode) directly across every message type's
// error branches, which the envelope round-trip tests never exercise.

func mustJSONACP(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestValidateACPPayloadBranches(t *testing.T) {
	bigText := strings.Repeat("x", ACPMaxTextBytes+1)
	cases := []struct {
		name string
		typ  ACPMessageType
		task string
		raw  []byte
	}{
		{"mission decode err", ACPMessageMission, "T1", []byte("{not json")},
		{"mission no task", ACPMessageMission, "", mustJSONACP(t, validACPMission())},
		{"mission bad digest", ACPMessageMission, "T1", func() []byte {
			p := validACPMission()
			p.DispatchDigest = "short"
			return mustJSONACP(t, p)
		}()},
		{"mission bad role", ACPMessageMission, "T1", func() []byte {
			p := validACPMission()
			p.Role = "wizard"
			return mustJSONACP(t, p)
		}()},
		{"mission bad contextCommand", ACPMessageMission, "T1", func() []byte {
			p := validACPMission()
			p.ContextCommand = bigText
			return mustJSONACP(t, p)
		}()},
		{"mission bad contract", ACPMessageMission, "T1", func() []byte {
			p := validACPMission()
			p.Contract = ""
			return mustJSONACP(t, p)
		}()},
		{"mission bad files", ACPMessageMission, "T1", func() []byte {
			p := validACPMission()
			p.Files = []string{""}
			return mustJSONACP(t, p)
		}()},
		{"mission bad deps", ACPMessageMission, "T1", func() []byte {
			p := validACPMission()
			p.Dependencies = []string{"not a task id"}
			return mustJSONACP(t, p)
		}()},
		{"mission no authority", ACPMessageMission, "T1", func() []byte {
			p := validACPMission()
			p.Authority.AllowedActions = nil
			return mustJSONACP(t, p)
		}()},
		{"mission bad authority action", ACPMessageMission, "T1", func() []byte {
			p := validACPMission()
			p.Authority.AllowedActions = []string{"launch-missiles"}
			return mustJSONACP(t, p)
		}()},

		{"accepted decode err", ACPMessageAccepted, "", []byte("{")},
		{"accepted bad worker", ACPMessageAccepted, "", mustJSONACP(t, ACPAcceptedPayload{WorkerID: "bad id!"})},

		{"heartbeat bad worker", ACPMessageHeartbeat, "", mustJSONACP(t, ACPHeartbeatPayload{WorkerID: "bad id!", Status: "ok"})},
		{"heartbeat bad status", ACPMessageHeartbeat, "", mustJSONACP(t, ACPHeartbeatPayload{WorkerID: "worker-1", Status: ""})},

		{"progress out of range", ACPMessageProgress, "", mustJSONACP(t, ACPProgressPayload{Percent: 150, Message: "ok"})},
		{"progress bad message", ACPMessageProgress, "", mustJSONACP(t, ACPProgressPayload{Percent: 10, Message: ""})},

		{"evidence no task", ACPMessageEvidence, "", mustJSONACP(t, ACPEvidencePayload{VerificationRef: "ref", Summary: "s"})},
		{"evidence no ref", ACPMessageEvidence, "T1", mustJSONACP(t, ACPEvidencePayload{Summary: "s"})},
		{"evidence bad summary", ACPMessageEvidence, "T1", mustJSONACP(t, ACPEvidencePayload{VerificationRef: "ref", Summary: ""})},
		{"evidence negative", ACPMessageEvidence, "T1", mustJSONACP(t, ACPEvidencePayload{VerificationRef: "ref", Summary: "s", DurationMs: -1})},

		{"blocker bad reason", ACPMessageBlocker, "", mustJSONACP(t, ACPBlockerPayload{Reason: ""})},
		{"query bad text", ACPMessageQuery, "", mustJSONACP(t, ACPQueryPayload{Text: ""})},
		{"directive bad action", ACPMessageDirective, "", mustJSONACP(t, ACPDirectivePayload{Action: "explode", Reason: "r"})},
		{"directive bad reason", ACPMessageDirective, "", func() []byte {
			// Use a valid action with an empty reason to reach the reason branch.
			raw := mustJSONACP(t, ACPDirectivePayload{Reason: ""})
			return raw
		}()},
		{"cancelled bad reason", ACPMessageCancelled, "", mustJSONACP(t, ACPCancelledPayload{Reason: ""})},

		{"unsupported type", ACPMessageType("bogus"), "", []byte("{}")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := validateACPPayload(c.typ, c.task, c.raw); err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
	}
}

func TestValidateACPLeafValidators(t *testing.T) {
	// validateACPText
	if err := validateACPText("f", "", true); err == nil {
		t.Error("required empty should error")
	}
	if err := validateACPText("f", strings.Repeat("x", ACPMaxTextBytes+1), false); err == nil {
		t.Error("oversize should error")
	}
	if err := validateACPText("f", "a\x00b", false); err == nil {
		t.Error("NUL should error")
	}
	if err := validateACPText("f", "", false); err != nil {
		t.Errorf("optional empty ok: %v", err)
	}

	// validateACPPaths
	if err := validateACPPaths("f", make([]string, ACPMaxListItems+1)); err == nil {
		t.Error("too many paths should error")
	}
	if err := validateACPPaths("f", []string{"a\x00b"}); err == nil {
		t.Error("NUL path should error")
	}
	if err := validateACPPaths("f", []string{"ok/path"}); err != nil {
		t.Errorf("valid path: %v", err)
	}

	// validateACPTaskIDs
	if err := validateACPTaskIDs("f", make([]string, ACPMaxListItems+1)); err == nil {
		t.Error("too many task ids should error")
	}
	if err := validateACPTaskIDs("f", []string{"bad id"}); err == nil {
		t.Error("invalid task id should error")
	}
}

func TestDecodeACPStrictBranches(t *testing.T) {
	var dst map[string]any
	if err := decodeACPStrict([]byte("{bad"), &dst); err == nil {
		t.Error("malformed should error")
	}
	if err := decodeACPStrict([]byte(`{"a":1} {}`), &dst); err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Errorf("multiple values err = %v", err)
	}
	if err := decodeACPStrict([]byte(`{"a":1}`), &dst); err != nil {
		t.Errorf("single value ok: %v", err)
	}
}

func TestValidateACPDirectionBranches(t *testing.T) {
	// mission/directive: brain→pinky only
	if err := validateACPDirection(ACPMessageMission, "pinky-w", "brain"); err == nil {
		t.Error("mission wrong direction should error")
	}
	// pinky→brain types
	if err := validateACPDirection(ACPMessageEvidence, "brain", "pinky-w"); err == nil {
		t.Error("evidence wrong direction should error")
	}
	// heartbeat: must be between the two parties
	if err := validateACPDirection(ACPMessageHeartbeat, "brain", "brain"); err == nil {
		t.Error("heartbeat same party should error")
	}
	if err := validateACPDirection(ACPMessageHeartbeat, "brain", "pinky-w"); err != nil {
		t.Errorf("valid heartbeat: %v", err)
	}
}
