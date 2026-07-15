package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTelemetryCompat(t *testing.T) {
	// A task without telemetry serializes without the field (byte-compat).
	ts := TaskState{ID: "T1", Status: TaskComplete}
	out, err := json.Marshal(ts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "telemetry") {
		t.Errorf("absent telemetry must be omitted: %s", out)
	}

	// Legacy state.json (no telemetry key) still parses.
	var legacy TaskState
	if err := json.Unmarshal([]byte(`{"id":"T1","status":"complete"}`), &legacy); err != nil {
		t.Fatalf("legacy task parse: %v", err)
	}
	if legacy.Telemetry != nil {
		t.Errorf("legacy task should have nil telemetry, got %+v", legacy.Telemetry)
	}

	// Populated telemetry round-trips, and empty sub-fields stay omitted.
	ts.Telemetry = &Telemetry{DurationMs: 1500, Retries: 2, Tokens: 9000, Cost: "0.42"}
	out, _ = json.Marshal(ts)
	if strings.Contains(string(out), "verifyDurationMs") {
		t.Errorf("empty verifyDurationMs should be omitted: %s", out)
	}
	var back TaskState
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if back.Telemetry == nil || back.Telemetry.DurationMs != 1500 || back.Telemetry.Retries != 2 ||
		back.Telemetry.Tokens != 9000 || back.Telemetry.Cost != "0.42" {
		t.Errorf("telemetry round-trip lost data: %+v", back.Telemetry)
	}
}

func TestDurationMsBetween(t *testing.T) {
	if got := DurationMsBetween("2026-01-01T00:00:00Z", "2026-01-01T00:00:01.5Z"); got != 1500 {
		t.Errorf("DurationMsBetween = %d, want 1500", got)
	}
	if got := DurationMsBetween("bad", "2026-01-01T00:00:01Z"); got != 0 {
		t.Errorf("unparseable start should yield 0, got %d", got)
	}
	if got := DurationMsBetween("2026-01-01T00:00:05Z", "2026-01-01T00:00:00Z"); got != 0 {
		t.Errorf("negative interval should clamp to 0, got %d", got)
	}
}
