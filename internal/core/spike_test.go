package core

import (
	"testing"
	"time"
)

// TestSpikeValidateBound pins R7.3's bound: a spike needs a question, a scope,
// and an expiry that is present, RFC3339, and strictly after the record's own
// timestamp. A spike missing any of these — or one whose window is empty/inverted
// — is rejected, so an "unbounded spike" cannot exist on disk.
func TestSpikeValidateBound(t *testing.T) {
	base := Spike{Question: "q", Scope: "s", Expiry: "2026-07-19T00:00:00Z", Timestamp: "2026-07-12T00:00:00Z"}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid spike rejected: %v", err)
	}

	cases := map[string]Spike{
		"missing question":           {Scope: "s", Expiry: "2026-07-19T00:00:00Z"},
		"blank question":             {Question: "  ", Scope: "s", Expiry: "2026-07-19T00:00:00Z"},
		"missing scope":              {Question: "q", Expiry: "2026-07-19T00:00:00Z"},
		"missing expiry":             {Question: "q", Scope: "s"},
		"unparseable expiry":         {Question: "q", Scope: "s", Expiry: "next week"},
		"expiry not after timestamp": {Question: "q", Scope: "s", Expiry: "2026-07-12T00:00:00Z", Timestamp: "2026-07-12T00:00:00Z"},
		"expiry before timestamp":    {Question: "q", Scope: "s", Expiry: "2026-07-01T00:00:00Z", Timestamp: "2026-07-12T00:00:00Z"},
	}
	for name, spike := range cases {
		if err := spike.Validate(); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}
}

// TestSpikeExpired covers the staleness read: a spike is expired once now has
// reached its expiry, and an unreadable expiry is treated as expired (fail-closed).
func TestSpikeExpired(t *testing.T) {
	s := Spike{Expiry: "2026-07-19T00:00:00Z"}
	if s.Expired(mustTime(t, "2026-07-18T23:59:59Z")) {
		t.Error("spike marked expired before its expiry")
	}
	if !s.Expired(mustTime(t, "2026-07-19T00:00:00Z")) {
		t.Error("spike not expired at its expiry instant")
	}
	if !(Spike{Expiry: "garbage"}).Expired(mustTime(t, "2026-07-12T00:00:00Z")) {
		t.Error("unparseable expiry must fail closed as expired")
	}
}

// TestSpikeAppendRoundtrip pins storage: appended spikes decode back in stable
// key order, keys are sequential and never reused, and — crucially — recording
// a spike touches no lifecycle status, task status, or approval/evidence record.
func TestSpikeAppendRoundtrip(t *testing.T) {
	swapClock(t, "2026-07-12T00:00:00Z")
	state := InitialState("demo")

	for i, q := range []string{"first", "second"} {
		if err := state.AppendSpike(StampSpike(Spike{Question: q, Scope: "s", Expiry: "2026-08-01T00:00:00Z"}, "abc123")); err != nil {
			t.Fatalf("append spike %d: %v", i, err)
		}
	}
	spikes, err := state.Spikes()
	if err != nil {
		t.Fatalf("Spikes: %v", err)
	}
	if len(spikes) != 2 || spikes[0].Question != "first" || spikes[1].Question != "second" {
		t.Fatalf("spikes out of order: %+v", spikes)
	}
	if spikes[0].Actor == "" || spikes[0].GitHead != "abc123" || spikes[0].Timestamp == "" {
		t.Fatalf("spike not stamped: %+v", spikes[0])
	}

	// A spike is learning, not authorization: none of the lifecycle transition
	// state moves (R7.3 "shall not let spike complete task or approve architecture").
	if state.Status != StatusRequirements || state.Phase != PhaseForStatus(StatusRequirements) {
		t.Fatalf("spike moved lifecycle: status=%q phase=%q", state.Status, state.Phase)
	}
	if len(state.TaskStatus) != 0 {
		t.Fatalf("spike completed a task: %+v", state.TaskStatus)
	}
	for key := range state.Records {
		if len(key) < len(spikeRecordPrefix) || key[:len(spikeRecordPrefix)] != spikeRecordPrefix {
			t.Fatalf("spike wrote a non-spike record %q", key)
		}
	}

	// A rejected spike writes nothing.
	before := len(state.Records)
	if err := state.AppendSpike(Spike{Scope: "s", Expiry: "2026-08-01T00:00:00Z"}); err == nil {
		t.Fatal("append accepted a spike with no question")
	}
	if len(state.Records) != before {
		t.Fatal("rejected spike mutated records")
	}
}

func swapClock(t *testing.T, iso string) {
	t.Helper()
	prev := Clock
	Clock = func() time.Time { return mustTime(t, iso) }
	t.Cleanup(func() { Clock = prev })
}

func mustTime(t *testing.T, iso string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		t.Fatalf("parse %q: %v", iso, err)
	}
	return parsed
}
