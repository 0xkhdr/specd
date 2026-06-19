package core

import (
	"strings"
	"testing"
	"time"
)

func TestPinkyProgressBlockReportCancel(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("6", 32)
	cfg := DefaultConfig.Orchestration
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	mission, err := BuildPinkyMission(root, "demo", sessionID, "pinky-a", "T1", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimPinkyMission(root, mission, cfg); err != nil {
		t.Fatal(err)
	}
	progress, err := RecordPinkyProgress(root, PinkyProgressReport{
		SessionID: sessionID,
		WorkerID:  "pinky-a",
		Spec:      "demo",
		TaskID:    "T1",
		Attempt:   1,
		Percent:   50,
		Message:   "halfway",
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if progress.Type != ACPMessageProgress || progress.Sequence != 1 {
		t.Fatalf("progress event = %#v, want progress sequence 1", progress)
	}
	blocker, err := RecordPinkyBlocker(root, PinkyBlockerReport{
		SessionID: sessionID,
		WorkerID:  "pinky-a",
		Spec:      "demo",
		TaskID:    "T1",
		Attempt:   1,
		Reason:    "waiting on review",
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if blocker.Type != ACPMessageBlocker || blocker.Sequence != 2 {
		t.Fatalf("blocker event = %#v, want blocker sequence 2", blocker)
	}
	report, err := RecordPinkyTerminalReport(root, PinkyTerminalReport{
		SessionID:       sessionID,
		WorkerID:        "pinky-a",
		Spec:            "demo",
		TaskID:          "T1",
		Attempt:         1,
		VerificationRef: "state:T1:revision:4",
		Summary:         "verified",
		ChangedFiles:    []string{"internal/core/demo.go"},
		GitHead:         strings.Repeat("a", 40),
		DurationMs:      1200,
		HostTokens:      33,
		HostCost:        "0.01",
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := RecordPinkyTerminalReport(root, PinkyTerminalReport{
		SessionID:       sessionID,
		WorkerID:        "pinky-a",
		Spec:            "demo",
		TaskID:          "T1",
		Attempt:         1,
		VerificationRef: "state:T1:revision:4",
		Summary:         "verified",
		ChangedFiles:    []string{"internal/core/demo.go"},
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.MessageID != report.MessageID || duplicate.Sequence != report.Sequence {
		t.Fatalf("duplicate terminal report wrote new event: first=%#v duplicate=%#v", report, duplicate)
	}
	cancelled, err := AcknowledgePinkyCancellation(root, sessionID, "pinky-a", "demo", "T1", 1, "stopped", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Type != ACPMessageCancelled {
		t.Fatalf("cancel event = %#v, want cancelled", cancelled)
	}
}

func TestPinkyReportRejectsReleasedLease(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("7", 32)
	cfg := DefaultConfig.Orchestration
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	mission, err := BuildPinkyMission(root, "demo", sessionID, "pinky-a", "T1", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimPinkyMission(root, mission, cfg); err != nil {
		t.Fatal(err)
	}
	if err := ReleasePinkyClaim(root, sessionID, "pinky-a", 1); err != nil {
		t.Fatal(err)
	}
	_, err = RecordPinkyProgress(root, PinkyProgressReport{
		SessionID: sessionID,
		WorkerID:  "pinky-a",
		Spec:      "demo",
		TaskID:    "T1",
		Attempt:   1,
		Percent:   80,
		Message:   "late",
	}, cfg)
	if err == nil || !strings.Contains(err.Error(), "released") {
		t.Fatalf("late progress error = %v, want released lease rejection", err)
	}
}
