package core

import (
	"strings"
	"testing"
	"time"
)

// pinky_validate_cov_test.go covers validatePinkyMission's field error branches
// (by mutating a known-valid mission) and pinkyAllowedActions for both role
// classes.

func TestPinkyAllowedActions(t *testing.T) {
	rw := pinkyAllowedActions("craftsman")
	if strings.Join(rw, ",") != "read,edit,verify,report" {
		t.Errorf("craftsman actions = %v", rw)
	}
	ro := pinkyAllowedActions("auditor")
	if strings.Join(ro, ",") != "read,verify,report" {
		t.Errorf("readonly actions = %v", ro)
	}
}

func TestValidatePinkyMissionBranches(t *testing.T) {
	root := writePinkySpec(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	defer setCoreClock(func() time.Time { return now })()
	valid, err := BuildPinkyMission(root, "demo", strings.Repeat("4", 32), "pinky-a", "T1", 1, DefaultConfig.Orchestration)
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePinkyMission(valid); err != nil {
		t.Fatalf("valid mission rejected: %v", err)
	}

	cases := map[string]func(m *PinkyMission){
		"bad version":   func(m *PinkyMission) { m.Version = 0 },
		"bad session":   func(m *PinkyMission) { m.SessionID = "short" },
		"bad worker":    func(m *PinkyMission) { m.WorkerID = "../bad" },
		"bad spec":      func(m *PinkyMission) { m.Spec = "../x" },
		"bad task":      func(m *PinkyMission) { m.TaskID = "bad id" },
		"bad attempt":   func(m *PinkyMission) { m.Attempt = 0 },
		"bad role":      func(m *PinkyMission) { m.Role = "wizard" },
		"bad deadline":  func(m *PinkyMission) { m.Deadline = "nope" },
		"bad heartbeat": func(m *PinkyMission) { m.HeartbeatEvery = 0 },
		"bad contract":  func(m *PinkyMission) { m.Contract = "" },
	}
	for name, mutate := range cases {
		m := valid
		mutate(&m)
		if err := validatePinkyMission(m); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
