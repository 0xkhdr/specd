package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// SpanKind is the versioned closed enum of run-span activity types (spec 07
// R6.1). A trace span records only metadata about one activity in a run — never
// the prompt, response, chain-of-thought, file contents, or raw output that
// produced it. Unknown kinds fail ParseSpanKind unless namespaced as an
// extension ("x-<name>"), so a critical kind is never silently dropped.
type SpanKind string

const (
	SpanContext  SpanKind = "context"
	SpanModel    SpanKind = "model"
	SpanTool     SpanKind = "tool"
	SpanEdit     SpanKind = "edit"
	SpanVerify   SpanKind = "verify"
	SpanEval     SpanKind = "eval"
	SpanApproval SpanKind = "approval"
	SpanDispatch SpanKind = "dispatch"
)

// SpanExtensionPrefix namespaces a vendor or experimental span kind. A kind
// carrying it is tolerated by ParseSpanKind even though it sits outside the
// closed enum; any other unknown kind fails closed (spec 07 R6.1).
const SpanExtensionPrefix = "x-"

func knownSpanKinds() []SpanKind {
	return []SpanKind{SpanContext, SpanModel, SpanTool, SpanEdit, SpanVerify, SpanEval, SpanApproval, SpanDispatch}
}

// ParseSpanKind validates a span kind string. A member of the closed enum, or a
// non-empty extension kind ("x-…"), parses; anything else fails so an unknown
// critical kind cannot silently disappear from a trace (spec 07 R6.1).
func ParseSpanKind(s string) (SpanKind, error) {
	for _, k := range knownSpanKinds() {
		if SpanKind(s) == k {
			return k, nil
		}
	}
	if strings.HasPrefix(s, SpanExtensionPrefix) && len(s) > len(SpanExtensionPrefix) {
		return SpanKind(s), nil
	}
	return "", fmt.Errorf("unknown span kind %q", s)
}

// ClaimsCodeEffect reports whether a span asserts a change to, or verification/
// evaluation of, tracked code — edit, verify, and eval spans. Such a span must
// carry a git_head anchoring the claim to a real artifact (spec 07 R6.3).
func (k SpanKind) ClaimsCodeEffect() bool {
	switch k {
	case SpanEdit, SpanVerify, SpanEval:
		return true
	}
	return false
}

// RunSpan is one metadata-only node in a run's trajectory (spec 07 R6). It reuses
// the W2 run/attempt identity (RunID) and W3 accounting rather than inventing new
// keys, and carries no payload content. StartedAt is informational — shown "where
// recorded" — and no gate derives an outcome from wall-clock order (R6.3).
// Ordering across equal or missing timestamps is resolved by the (SourceRank,
// Seq) tie-break, exactly like HistoryEvent, so a trace export is byte-stable
// (R6.2).
type RunSpan struct {
	SpanID       string   `json:"span_id"`
	ParentSpanID string   `json:"parent_span_id,omitempty"`
	RunID        string   `json:"run_id,omitempty"`
	SpecID       string   `json:"spec_id"`
	TaskID       string   `json:"task_id,omitempty"`
	Attempt      int      `json:"attempt,omitempty"`
	Kind         SpanKind `json:"kind"`
	StartedAt    string   `json:"started_at,omitempty"`
	GitHead      string   `json:"git_head,omitempty"`
	Actor        string   `json:"actor,omitempty"`
	Status       string   `json:"status,omitempty"`
	Reference    string   `json:"reference,omitempty"`

	// SourceRank and Seq are the deterministic tie-break keys (R6.2), never
	// serialized — identical role and semantics as HistoryEvent's.
	SourceRank int `json:"-"`
	Seq        int `json:"-"`
}

// NewSpanID derives a span's deterministic identity from its stable coordinates.
// (SourceRank, Seq) is unique per source record within a spec and never depends
// on wall-clock time, so the id — and therefore the whole export — is
// byte-identical on every run over the same tree (spec 07 R6.2).
func NewSpanID(specID string, kind SpanKind, sourceRank, seq int, reference string) string {
	key := strings.Join([]string{specID, string(kind), strconv.Itoa(sourceRank), strconv.Itoa(seq), reference}, "\x00")
	return Digest([]byte(key))[:16]
}

// Validate enforces the span invariants: a resolvable kind, a span id, and a
// git_head on any span that claims code effects or completion (spec 07 R6.1,R6.3).
func (s RunSpan) Validate() error {
	if _, err := ParseSpanKind(string(s.Kind)); err != nil {
		return err
	}
	if s.SpanID == "" {
		return errors.New("span requires a span_id")
	}
	if s.Kind.ClaimsCodeEffect() && s.GitHead == "" {
		return fmt.Errorf("%s span %q claims code effects but carries no git_head", s.Kind, s.SpanID)
	}
	return nil
}

// SortSpans orders spans by timestamp, then by the (SourceRank, Seq) tie-break,
// giving a total order stable across runs even when timestamps are equal or
// absent (spec 07 R6.2). No ordering decision is load-bearing for any outcome —
// timestamps are informational (R6.3).
func SortSpans(spans []RunSpan) {
	sort.SliceStable(spans, func(i, j int) bool {
		a, b := spans[i], spans[j]
		if a.StartedAt != b.StartedAt {
			return a.StartedAt < b.StartedAt
		}
		if a.SourceRank != b.SourceRank {
			return a.SourceRank < b.SourceRank
		}
		return a.Seq < b.Seq
	})
}

// RenderTraceJSON emits the run trace as JSON Lines in stable order (spec 07
// R6.2). Every span is validated first, so a malformed kind or a code-effect span
// missing its git_head fails the export rather than emitting a silent gap. Span
// ids must be unique and every parent reference must resolve to a span in the
// same export.
func RenderTraceJSON(spans []RunSpan) (string, error) {
	SortSpans(spans)
	ids := make(map[string]bool, len(spans))
	for _, s := range spans {
		if err := s.Validate(); err != nil {
			return "", err
		}
		if ids[s.SpanID] {
			return "", fmt.Errorf("duplicate span_id %q", s.SpanID)
		}
		ids[s.SpanID] = true
	}
	for _, s := range spans {
		if s.ParentSpanID != "" && !ids[s.ParentSpanID] {
			return "", fmt.Errorf("span %q references unknown parent %q", s.SpanID, s.ParentSpanID)
		}
	}
	var b strings.Builder
	for _, s := range spans {
		raw, err := json.Marshal(s)
		if err != nil {
			return "", err
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	return b.String(), nil
}
