package core

import (
	"testing"
	"time"
)

// Future/skewed progress-timestamp bound (spec A7, Req 2).
//
// progress-weighted waits trust a server-stamped lastReport to decide whether a
// quiet worker is "honestly slow" (extend the wait) or stalled (count toward
// MaxWaits). A worker that stamps lastReport into the future — clock skew or a
// malicious worker trying to look perpetually fresh — must NOT be able to extend
// its wait. A future timestamp is rejected, so the wait falls through to the
// stall counter, and MaxWaits/MaxSteps bound it.
func TestProgressWithinWindowRejectsFutureSkew(t *testing.T) {
	base := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return base })
	defer restore()

	cfg := DefaultConfig.Orchestration
	cfg.Resilience = &ResilienceCfg{ProgressTimeoutSeconds: 300}

	cases := []struct {
		name   string
		offset time.Duration // relative to base; positive = future
		want   bool
	}{
		{"fresh-past", -10 * time.Second, true},
		{"stale-past", -300 * time.Second, false},
		{"just-future", 1 * time.Second, false},
		{"far-future", 365 * 24 * time.Hour, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := OrchestrationSnapshot{
				MostRecentProgressAt: base.Add(tc.offset).Format(time.RFC3339Nano),
			}
			if got := progressWithinWindow(snap, cfg); got != tc.want {
				t.Fatalf("progressWithinWindow(offset=%s) = %v, want %v", tc.offset, got, tc.want)
			}
		})
	}
}

// TestDriveOrchestrationFutureSkewTerminates proves the bound end-to-end: a
// worker that has reported a far-future progress timestamp cannot keep the
// driver waiting forever — with no in-flight workers the wait counts toward
// MaxWaits and the driver stalls within the documented bound rather than
// spinning to MaxSteps on a forged "fresh" stamp.
func TestDriveOrchestrationFutureSkewBoundsWait(t *testing.T) {
	base := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return base })
	defer restore()

	cfg := DefaultConfig.Orchestration
	cfg.Resilience = &ResilienceCfg{ProgressTimeoutSeconds: 300}

	// Decision stays pure over (snapshot, policy): skew enters only through the
	// sensed MostRecentProgressAt, not the decision. A future stamp is not
	// within-window, so it does not suppress the stall counter.
	future := OrchestrationSnapshot{
		MostRecentProgressAt: base.Add(48 * time.Hour).Format(time.RFC3339Nano),
	}
	if progressWithinWindow(future, cfg) {
		t.Fatal("future-stamped progress must not be treated as within-window (would extend wait unbounded)")
	}
}
