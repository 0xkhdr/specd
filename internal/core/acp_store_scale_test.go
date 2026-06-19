package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// scaleEnvelope builds a sequence-zero mission envelope with a unique messageId
// derived from number, without a *testing.T so it is usable from benchmarks.
func scaleEnvelope(sessionID string, number int) (ACPEnvelope, error) {
	envelope, err := NewACPEnvelope(ACPMessageMission, validACPMission())
	if err != nil {
		return ACPEnvelope{}, err
	}
	envelope.MessageID = fmt.Sprintf("%032x", number)
	envelope.SessionID = sessionID
	envelope.Sequence = 0
	envelope.CreatedAt = time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	envelope.ExpiresAt = time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	envelope.From = "brain"
	envelope.To = "pinky-worker-1"
	envelope.Spec = "example"
	envelope.Task = "T1"
	envelope.Attempt = 1
	return envelope, nil
}

// BenchmarkACPStoreWriteEvent measures per-event write cost against a session
// that already holds many events. Before the GAP-13 fix WriteEvent read, parsed
// and re-validated every prior event on each write (O(n²) over a session); now
// sequence allocation and the dup check are filename-derived, so per-write cost
// no longer scales with the parsed payloads of the backlog.
func BenchmarkACPStoreWriteEvent(b *testing.B) {
	store, err := NewACPStore(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	sessionID := strings.Repeat("2", 32)
	const seed = 2000
	for i := 1; i <= seed; i++ {
		env, err := scaleEnvelope(sessionID, i)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := store.WriteEvent(env); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env, err := scaleEnvelope(sessionID, seed+1+i)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := store.WriteEvent(env); err != nil {
			b.Fatal(err)
		}
	}
}

// TestACPStoreScaleBehaviorUnchanged is the GAP-13 regression guard: over a
// large session the filename-derived write path must preserve sequence
// contiguity, ordering, duplicate rejection, and sequence-gap detection.
func TestACPStoreScaleBehaviorUnchanged(t *testing.T) {
	store := newTestACPStore(t)
	sessionID := strings.Repeat("2", 32)
	const n = 500

	for i := 1; i <= n; i++ {
		env, err := scaleEnvelope(sessionID, i)
		if err != nil {
			t.Fatal(err)
		}
		written, err := store.WriteEvent(env)
		if err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		if written.Sequence != uint64(i) {
			t.Fatalf("write %d: sequence = %d, want %d", i, written.Sequence, i)
		}
	}

	// Ordering + contiguity across the whole session.
	events, err := store.ReadEvents(sessionID, "reader")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != n {
		t.Fatalf("read %d events, want %d", len(events), n)
	}
	for i, e := range events {
		if e.Sequence != uint64(i+1) {
			t.Fatalf("event %d has sequence %d, want %d", i, e.Sequence, i+1)
		}
	}

	// Duplicate messageId is still rejected after the backlog grows.
	dup, err := scaleEnvelope(sessionID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.WriteEvent(dup); err == nil || !strings.Contains(err.Error(), "duplicate messageId") {
		t.Fatalf("duplicate not rejected: %v", err)
	}

	// A sequence gap (manually planted) is still detected on the next read.
	gapPath, err := store.paths.EventPath(sessionID, n+2, fmt.Sprintf("%032x", n+2))
	if err != nil {
		t.Fatal(err)
	}
	gapEnv, err := scaleEnvelope(sessionID, n+2)
	if err != nil {
		t.Fatal(err)
	}
	gapEnv.Sequence = uint64(n + 2)
	raw, err := json.MarshalIndent(gapEnv, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := writeImmutablePrivate(gapPath, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadEvents(sessionID, "reader"); err == nil || !strings.Contains(err.Error(), "gap") {
		t.Fatalf("sequence gap not detected: %v", err)
	}
}
