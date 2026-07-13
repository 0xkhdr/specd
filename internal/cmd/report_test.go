package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

func TestReportEventAndEfficiencyOffline(t *testing.T) {
	root := newHistoryDemo(t)
	events, err := captureStdout(t, func() error { return Run(root, "report", []string{"demo"}, map[string]string{"format": "event"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(events), "\n") {
		var event core.EventV1
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("invalid event: %v", err)
		}
		if err := event.Validate(); err != nil {
			t.Fatalf("invalid event contract: %v", err)
		}
	}
	efficiency, err := captureStdout(t, func() error { return Run(root, "report", []string{"demo"}, map[string]string{"efficiency": ""}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"context_efficiency schema=context-efficiency/v1", "task=T1", "retries=1", "first_pass=fail", "actual_tokens=unknown", "duration_ms=unknown", "cost=unknown"} {
		if !strings.Contains(efficiency, want) {
			t.Fatalf("missing %q in %s", want, efficiency)
		}
	}
}

func TestReportMissionAuditReferenceIsPolicyKeyedAndSanitized(t *testing.T) {
	e := orchestration.ACPEvent{AuditID: 3, AuditKind: "diff", RunID: "run-1", MissionID: "mission-1", TaskID: "T1", PolicyDigest: "sha256:abc", Payload: "internal detail"}
	got := acpReference(e)
	for _, want := range []string{"run=run-1", "mission=mission-1", "task=T1", "policy=sha256:abc", "stage=diff", "audit_id=3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("reference %q missing %q", got, want)
		}
	}
	if strings.Contains(got, e.Payload) {
		t.Fatal("raw payload leaked")
	}
}
