package core

import (
	"fmt"
	"sort"
	"time"
)

// DurationMsBetween returns the milliseconds between two RFC3339 timestamps, or
// 0 if either is unparseable or the interval is negative. It is the shared,
// clock-agnostic basis for telemetry durations (the timestamps themselves come
// from the injectable Clock, so the result is deterministic under the test clock).
func DurationMsBetween(startISO, endISO string) int64 {
	start, err1 := time.Parse(time.RFC3339Nano, startISO)
	end, err2 := time.Parse(time.RFC3339Nano, endISO)
	if err1 != nil || err2 != nil {
		return 0
	}
	d := end.Sub(start).Milliseconds()
	if d < 0 {
		return 0
	}
	return d
}

// TimelineEvent is one normalized, audit-derived moment in a spec's life. Events
// are collected from state.json's task history (start/finish/verify/block) and
// acceptance records, then stably ordered for a deterministic replay.
type TimelineEvent struct {
	At     string `json:"at"`             // RFC3339 timestamp; may be "" when the source lacks one
	Kind   string `json:"kind"`           // started|finished|verified|verify-failed|blocked|criterion-pass|criterion-fail
	Task   string `json:"task,omitempty"` // task id, when the event is task-scoped
	Detail string `json:"detail,omitempty"`
}

// ReplayTimeline normalizes a spec's on-disk audit records into a stably ordered
// event list. It is read-only and total: missing timestamps and partially
// populated records are tolerated (an absent timestamp simply sorts first), so a
// corrupt or half-written state never panics the replay.
func ReplayTimeline(state *State) []TimelineEvent {
	if state == nil {
		return nil
	}
	var events []TimelineEvent
	for _, t := range state.Tasks {
		if t.StartedAt != nil {
			events = append(events, TimelineEvent{At: *t.StartedAt, Kind: "started", Task: t.ID, Detail: t.Title})
		}
		if t.FinishedAt != nil {
			events = append(events, TimelineEvent{At: *t.FinishedAt, Kind: "finished", Task: t.ID, Detail: t.Title})
		}
		if v := t.Verification; v != nil {
			kind := "verified"
			if !v.Verified {
				kind = "verify-failed"
			}
			events = append(events, TimelineEvent{At: v.RanAt, Kind: kind, Task: t.ID, Detail: v.Command})
		}
		if t.Blocker != nil {
			events = append(events, TimelineEvent{Kind: "blocked", Task: t.ID, Detail: *t.Blocker})
		}
	}
	for key, c := range state.Acceptance {
		kind := "criterion-pass"
		if c.Status != "pass" {
			kind = "criterion-fail"
		}
		events = append(events, TimelineEvent{At: c.RanAt, Kind: kind, Detail: fmt.Sprintf("%s: %s", key, c.Evidence)})
	}

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].At != events[j].At {
			return events[i].At < events[j].At
		}
		if oi, oj := ordinal(events[i].Task), ordinal(events[j].Task); oi != oj {
			return oi < oj
		}
		if events[i].Task != events[j].Task {
			return events[i].Task < events[j].Task
		}
		return events[i].Kind < events[j].Kind
	})
	return events
}
