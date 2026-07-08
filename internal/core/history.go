package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// HistoryEvent is one replayed line of a spec's audit trail (spec 13 R1). It is
// projected from records that already exist on disk — approvals/decisions in
// state.json, criterion and submission ledgers, verify evidence, the ACP ledger
// — never from a new event store, and `report --history` writes nothing (R2).
//
// Timestamp/Actor are shown "where recorded": a source that does not stamp them
// leaves them empty rather than inventing a value.
type HistoryEvent struct {
	Timestamp string `json:"timestamp,omitempty"`
	Actor     string `json:"actor,omitempty"`
	Event     string `json:"event"`
	Reference string `json:"reference,omitempty"`
	GitHead   string `json:"git_head,omitempty"`

	// SourceRank and Seq are the deterministic tie-break keys (R3), never
	// serialized. When two events share a timestamp (or both lack one), order is
	// resolved first by SourceRank (a fixed per-source-type ordering) then by Seq
	// (the record's position within its source). The pair is unique across all
	// events for a spec, so the total order — and therefore the byte output — is
	// identical on every run.
	SourceRank int `json:"-"`
	Seq        int `json:"-"`
}

// History source ranks. The values fix the tie-break order for events sharing a
// timestamp; they are an internal ordering key, not a public contract.
const (
	HistorySourceApproval = iota
	HistorySourceDecision
	HistorySourceMidReq
	HistorySourceVerify
	HistorySourceCompletion
	HistorySourceCriterion
	HistorySourceEscalation
	HistorySourceSubmission
	HistorySourceACP
)

// SortHistory orders events by timestamp, then by the (SourceRank, Seq)
// tie-break, giving a total order that is stable across runs (spec 13 R3).
func SortHistory(events []HistoryEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i], events[j]
		if a.Timestamp != b.Timestamp {
			return a.Timestamp < b.Timestamp
		}
		if a.SourceRank != b.SourceRank {
			return a.SourceRank < b.SourceRank
		}
		return a.Seq < b.Seq
	})
}

// RenderHistory prints the replay as one aligned line per event: timestamp,
// actor, event, reference. Empty fields render as "-" so columns stay stable.
func RenderHistory(slug string, events []HistoryEvent) string {
	SortHistory(events)
	var b strings.Builder
	fmt.Fprintf(&b, "history: %s (%d events)\n", slug, len(events))
	for _, e := range events {
		fmt.Fprintf(&b, "%s | %s | %s | %s\n",
			dash(e.Timestamp), dash(e.Actor), e.Event, dash(reference(e)))
	}
	return b.String()
}

// RenderHistoryJSON emits one JSON object per line (JSON Lines), the same events
// as RenderHistory in the same order (spec 13 R6).
func RenderHistoryJSON(events []HistoryEvent) (string, error) {
	SortHistory(events)
	var b strings.Builder
	for _, e := range events {
		raw, err := json.Marshal(e)
		if err != nil {
			return "", err
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// reference folds the git HEAD into the reference column when present, so the
// short-hash provenance travels with every stamped event.
func reference(e HistoryEvent) string {
	if e.GitHead == "" {
		return e.Reference
	}
	if e.Reference == "" {
		return "head=" + shortHead(e.GitHead)
	}
	return e.Reference + " head=" + shortHead(e.GitHead)
}

func shortHead(head string) string {
	if len(head) > 12 {
		return head[:12]
	}
	return head
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
