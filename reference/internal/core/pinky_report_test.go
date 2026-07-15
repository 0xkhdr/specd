package core

import (
	"encoding/json"
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

func TestPinkyQueryDirectiveInbox(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("8", 32)
	cfg := DefaultConfig.Orchestration
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	mission, err := BuildPinkyMission(root, "demo", sessionID, "worker-a", "T1", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimPinkyMission(root, mission, cfg); err != nil {
		t.Fatal(err)
	}
	query, err := RecordPinkyQuery(root, PinkyQueryReport{
		SessionID: sessionID,
		WorkerID:  "worker-a",
		Spec:      "demo",
		TaskID:    "T1",
		Attempt:   1,
		Text:      "Should this include docs?",
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if query.Type != ACPMessageQuery || query.Sequence != 1 || query.From != "pinky-worker-a" || query.To != "brain" {
		t.Fatalf("query event = %#v", query)
	}

	directive, err := RecordBrainDirective(root, BrainDirective{
		SessionID: sessionID,
		WorkerID:  "worker-a",
		Spec:      "demo",
		TaskID:    "T1",
		Attempt:   1,
		Action:    "continue",
		Reason:    "docs not required for this task",
		InReplyTo: query.MessageID,
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if directive.Type != ACPMessageDirective || directive.Sequence != 2 || directive.From != "brain" || directive.To != "pinky-worker-a" || directive.InReplyTo != query.MessageID {
		t.Fatalf("directive event = %#v", directive)
	}
	var payload ACPDirectivePayload
	if err := json.Unmarshal(directive.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Action != "continue" || payload.Reason == "" {
		t.Fatalf("directive payload = %#v", payload)
	}

	inbox, err := ReadPinkyInbox(root, sessionID, "worker-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox.Directives) != 1 || inbox.Directives[0].MessageID != directive.MessageID {
		t.Fatalf("inbox = %#v, want directive %s", inbox, directive.MessageID)
	}

	_, err = RecordBrainDirective(root, BrainDirective{
		SessionID: sessionID,
		WorkerID:  "worker-a",
		Spec:      "demo",
		TaskID:    "T1",
		Attempt:   1,
		Action:    "invalid",
		Reason:    "nope",
	}, cfg)
	if err == nil || !strings.Contains(err.Error(), "invalid action") {
		t.Fatalf("invalid directive error = %v, want invalid action", err)
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
