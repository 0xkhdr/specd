package orchestration

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func TestACPRigor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acp.jsonl")
	now := time.Now().UTC()

	// First claim on T1 is attempt 1.
	if err := AppendClaim(path, ACPEvent{Time: now, TaskID: "T1"}); err != nil {
		t.Fatalf("claim 1: %v", err)
	}
	// A claim on a different task is independent.
	if err := AppendClaim(path, ACPEvent{Time: now, TaskID: "T2"}); err != nil {
		t.Fatalf("claim T2: %v", err)
	}
	// Second claim on T1 is attempt 2 — monotonic per task, derived from the
	// ledger, not a stored counter (R3).
	claim := ACPEvent{
		Time:         now.Add(time.Second),
		TaskID:       "T1",
		GitHead:      "abc123",
		ChangedFiles: []string{"a.go", "b.go"},
		VerifyRef:    "evidence.jsonl#T1",
		Telemetry:    &core.Annotations{EnvelopeVersion: core.TelemetryEnvelopeV1, Source: core.TelemetrySourceWorker, Tokens: 500, Cost: "0.01", Currency: "USD", PricingRef: "pricing/v1"},
	}
	if err := AppendClaim(path, claim); err != nil {
		t.Fatalf("claim 2: %v", err)
	}

	events, err := ReadACP(path)
	if err != nil {
		t.Fatal(err)
	}

	var t1Attempts []int
	var second ACPEvent
	for _, e := range events {
		if e.Kind == ACPKindClaim && e.TaskID == "T1" {
			t1Attempts = append(t1Attempts, e.Attempt)
			if e.Attempt == 2 {
				second = e
			}
		}
	}
	if len(t1Attempts) != 2 || t1Attempts[0] != 1 || t1Attempts[1] != 2 {
		t.Fatalf("attempt numbers wrong: %v", t1Attempts)
	}
	if second.GitHead != "abc123" || len(second.ChangedFiles) != 2 || second.VerifyRef == "" {
		t.Fatalf("rigor fields not carried: %+v", second)
	}
	if second.Telemetry == nil || second.Telemetry.Tokens != 500 || second.Telemetry.Cost != "0.01" {
		t.Fatalf("telemetry not carried on claim: %+v", second.Telemetry)
	}
}
