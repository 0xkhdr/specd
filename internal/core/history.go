package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GovernanceStatus is shared by decisions and exceptions. Values are closed;
// unknown values fail validation instead of being silently reinterpreted.
type GovernanceStatus string

const (
	GovernanceProposed   GovernanceStatus = "proposed"
	GovernanceAccepted   GovernanceStatus = "accepted"
	GovernanceSuperseded GovernanceStatus = "superseded"
	GovernanceExpired    GovernanceStatus = "expired"
	GovernanceRevoked    GovernanceStatus = "revoked"
)

// DecisionV1 is one immutable governance assertion. Supersession is expressed
// by a later record's Supersedes link; EffectiveDecisionStatus projects the old
// record as superseded without rewriting history.
type DecisionV1 struct {
	ID                 string           `json:"id"`
	Status             GovernanceStatus `json:"status"`
	Owner              string           `json:"owner"`
	CreatedAt          string           `json:"created_at"`
	ReviewAt           string           `json:"review_at"`
	ExpiresAt          string           `json:"expires_at"`
	Supersedes         string           `json:"supersedes,omitempty"`
	AffectedInvariants []string         `json:"affected_invariants,omitempty"`
}

func DecisionPath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "decisions.json")
}

// LoadDecisions decodes immutable records in append order. Missing file means
// governance is unconfigured; malformed or invalid content fails closed.
func LoadDecisions(path string) ([]DecisionV1, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []DecisionV1
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("decode decisions: %w", err)
	}
	var checked []DecisionV1
	for _, record := range records {
		var appendErr error
		checked, appendErr = AppendDecision(checked, record)
		if appendErr != nil {
			return nil, appendErr
		}
	}
	return checked, nil
}

func validGovernanceStatus(s GovernanceStatus) bool {
	switch s {
	case GovernanceProposed, GovernanceAccepted, GovernanceSuperseded, GovernanceExpired, GovernanceRevoked:
		return true
	}
	return false
}

func ValidateGovernanceOwner(owner string) error {
	o := strings.ToLower(strings.TrimSpace(owner))
	if o == "" {
		return errors.New("governance owner must identify a human or team")
	}
	if IsKnownRole(o) || o == "agent" || strings.HasPrefix(o, "agent:") || strings.HasPrefix(o, "worker:") || strings.HasPrefix(o, "model/") {
		return fmt.Errorf("governance owner %q must identify a human or team, not an agent", owner)
	}
	return nil
}

func (d DecisionV1) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return errors.New("decision id is required")
	}
	if !validGovernanceStatus(d.Status) {
		return fmt.Errorf("decision %s has invalid status %q", d.ID, d.Status)
	}
	if err := ValidateGovernanceOwner(d.Owner); err != nil {
		return err
	}
	return validateGovernanceDates("decision "+d.ID, d.CreatedAt, d.ReviewAt, d.ExpiresAt)
}

func validateGovernanceDates(label, created, review, expires string) error {
	var parsed []time.Time
	for _, pair := range []struct{ name, value string }{{"created_at", created}, {"review_at", review}, {"expires_at", expires}} {
		v, err := time.Parse(time.RFC3339, pair.value)
		if err != nil {
			return fmt.Errorf("%s %s must be RFC3339: %w", label, pair.name, err)
		}
		parsed = append(parsed, v)
	}
	if parsed[1].Before(parsed[0]) || parsed[2].Before(parsed[0]) {
		return fmt.Errorf("%s review_at/expires_at precede created_at", label)
	}
	return nil
}

func AppendDecision(records []DecisionV1, next DecisionV1) ([]DecisionV1, error) {
	if err := next.Validate(); err != nil {
		return records, err
	}
	seen, superseded := false, false
	for _, record := range records {
		if record.ID == next.ID {
			return records, fmt.Errorf("decision id %q already exists", next.ID)
		}
		if record.ID == next.Supersedes {
			seen = true
		}
		if record.Supersedes == next.Supersedes && next.Supersedes != "" {
			superseded = true
		}
	}
	if next.Supersedes != "" && (!seen || superseded) {
		return records, fmt.Errorf("decision %s supersedes unknown or already superseded decision %q", next.ID, next.Supersedes)
	}
	return append(append([]DecisionV1(nil), records...), next), nil
}

func EffectiveDecisionStatus(records []DecisionV1, id string) GovernanceStatus {
	for _, record := range records {
		if record.Supersedes == id {
			return GovernanceSuperseded
		}
	}
	for _, record := range records {
		if record.ID == id {
			return record.Status
		}
	}
	return ""
}

func DecisionChain(records []DecisionV1, id string) []DecisionV1 {
	byID := make(map[string]DecisionV1, len(records))
	for _, r := range records {
		byID[r.ID] = r
	}
	var reverse []DecisionV1
	for id != "" {
		r, ok := byID[id]
		if !ok {
			break
		}
		reverse = append(reverse, r)
		id = r.Supersedes
	}
	for i, j := 0, len(reverse)-1; i < j; i, j = i+1, j-1 {
		reverse[i], reverse[j] = reverse[j], reverse[i]
	}
	return reverse
}

func (d DecisionV1) ActiveAt(now time.Time) bool {
	if d.Status != GovernanceAccepted {
		return false
	}
	expires, err := time.Parse(time.RFC3339, d.ExpiresAt)
	return err == nil && now.Before(expires)
}

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

	// TaskID is the in-process run-correlation key (spec 07 R6): the trace
	// exporter reuses it to attach a span to the W2 run chain and to link
	// activity spans to their task's dispatch. Not serialized — history JSON
	// stays byte-identical; the task already travels in Reference for readers.
	TaskID string `json:"-"`
}

// SpanKind maps a history event to its trace span kind and reports whether the
// event is a trace-worthy activity (spec 07 R6.1). This is the single place event
// names become span kinds, so the trace exporter and the audit replay never
// drift. Bookkeeping events without a code, evaluation, or dispatch effect —
// decisions, mid-requirement notes, submissions, ACP claim/report transport — are
// not spans and return false.
func (e HistoryEvent) SpanKind() (SpanKind, bool) {
	name := e.Event
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = name[:i]
	}
	switch name {
	case "approval":
		return SpanApproval, true
	case "verify":
		return SpanVerify, true
	case "completion", "criterion":
		return SpanEval, true
	case "acp":
		if e.Event == "acp:dispatch" {
			return SpanDispatch, true
		}
	}
	return "", false
}

// History source ranks. The values fix the tie-break order for events sharing a
// timestamp; they are an internal ordering key, not a public contract.
const (
	HistorySourceApproval = iota
	HistorySourceDecision
	HistorySourceMidReq
	HistorySourceProvenance
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
