package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ApprovalTransition is the closed set of approval-request states (spec 03
// R5.2). A request is never edited: creation is appended once per id and every
// later state is another appended transition. Anything outside the matrix in
// approvalTransitions refuses.
type ApprovalTransition string

const (
	ApprovalDraft      ApprovalTransition = "draft"
	ApprovalRequested  ApprovalTransition = "requested"
	ApprovalApproved   ApprovalTransition = "approved"
	ApprovalRejected   ApprovalTransition = "rejected"
	ApprovalWithdrawn  ApprovalTransition = "withdrawn"
	ApprovalExpired    ApprovalTransition = "expired"
	ApprovalRevoked    ApprovalTransition = "revoked"
	ApprovalSuperseded ApprovalTransition = "superseded"
)

// Approval-request entity kinds, the same three scopes a clarification names.
const (
	ApprovalEntitySpec     = ClarificationEntitySpec
	ApprovalEntityTask     = ClarificationEntityTask
	ApprovalEntityArtifact = ClarificationEntityArtifact
)

const approvalRequestRecordPrefix = "approval_request:"

// ApprovalRequestID keeps cycle 1 compatible with existing request identities
// and gives every later lifecycle cycle a fresh chain.
func ApprovalRequestID(gate string, cycle int) string {
	id := "approve:" + gate
	if cycle > 1 {
		id += fmt.Sprintf(":cycle:%d", cycle)
	}
	return id
}

// approvalTransitions is the canonical model: the legal successors of each
// state. The empty key is creation. States absent from the map (rejected,
// withdrawn, expired, revoked, superseded) are terminal, so a repeated
// transition on a closed chain — and any duplicate of the current state — has
// no entry and refuses.
var approvalTransitions = map[ApprovalTransition][]ApprovalTransition{
	"":                {ApprovalDraft, ApprovalRequested},
	ApprovalDraft:     {ApprovalRequested, ApprovalWithdrawn, ApprovalExpired, ApprovalSuperseded},
	ApprovalRequested: {ApprovalApproved, ApprovalRejected, ApprovalWithdrawn, ApprovalExpired, ApprovalSuperseded},
	ApprovalApproved:  {ApprovalRevoked, ApprovalSuperseded},
}

// ApprovalPins are the governing identities a request is pinned to (R5.1).
// Approval is refused when any of them has drifted from the request (R5.3).
type ApprovalPins struct {
	ArtifactDigest string `json:"artifact_digest,omitempty"`
	StateRevision  int64  `json:"state_revision,omitempty"`
	PlanDigest     string `json:"plan_digest,omitempty"`
	ConfigDigest   string `json:"config_digest,omitempty"`
}

// ApprovalRequestRecord is one immutable transition of one request. The create
// record carries the pinned identities, the requester, and the expiry; every
// later transition inherits them verbatim so the governing inputs of an
// approval can never be rewritten by a subsequent state change.
type ApprovalRequestRecord struct {
	Kind          string             `json:"kind"`
	ID            string             `json:"id"`
	Transition    ApprovalTransition `json:"transition"`
	EntityKind    string             `json:"entity_kind"`
	EntityID      string             `json:"entity_id"`
	EntityVersion string             `json:"entity_version,omitempty"`
	Pins          ApprovalPins       `json:"pins"`
	Requester     string             `json:"requester,omitempty"`
	ExpiresAt     string             `json:"expires_at,omitempty"`
	// SupersededBy names the replacement request on a superseded transition.
	SupersededBy string `json:"superseded_by,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Timestamp    string `json:"timestamp"`
	GitHead      string `json:"git_head"`
	Actor        string `json:"actor"`
}

// StampApprovalRequest fills the provenance triple, mirroring StampRecord.
func StampApprovalRequest(rec ApprovalRequestRecord, gitHead string) ApprovalRequestRecord {
	rec.Kind = "approval_request"
	rec.Timestamp = Clock().Format(time.RFC3339)
	rec.GitHead = gitHead
	rec.Actor = recordActor()
	return rec
}

// ReadApprovalRequests projects the approval transitions out of state records in
// append order (id, then sequence). Malformed records fail closed: a projection
// built from a record it cannot read would under-report a pending approval.
func ReadApprovalRequests(records map[string]json.RawMessage) ([]ApprovalRequestRecord, error) {
	keys := make([]string, 0, len(records))
	for key := range records {
		if strings.HasPrefix(key, approvalRequestRecordPrefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	out := make([]ApprovalRequestRecord, 0, len(keys))
	for _, key := range keys {
		var rec ApprovalRequestRecord
		if err := json.Unmarshal(records[key], &rec); err != nil {
			return nil, fmt.Errorf("decode approval request record %s: %w", key, err)
		}
		out = append(out, rec)
	}
	return out, nil
}

// ApprovalRequests is ReadApprovalRequests over this state's records.
func (s State) ApprovalRequests() ([]ApprovalRequestRecord, error) {
	return ReadApprovalRequests(s.Records)
}

// LatestApprovalRequest returns the newest transition of id and how many
// transitions it already has.
func LatestApprovalRequest(existing []ApprovalRequestRecord, id string) (ApprovalRequestRecord, int) {
	var latest ApprovalRequestRecord
	count := 0
	for _, rec := range existing {
		if rec.ID == id {
			latest, count = rec, count+1
		}
	}
	return latest, count
}

// ApprovalRequestPending reports whether id exists and is still open — draft or
// requested. It is the check behind the waiting_approval condition: a stage that
// waits on approval must name a request that can still be answered.
func ApprovalRequestPending(existing []ApprovalRequestRecord, id string) bool {
	latest, count := LatestApprovalRequest(existing, id)
	if count == 0 {
		return false
	}
	return latest.Transition == ApprovalDraft || latest.Transition == ApprovalRequested
}

// PlanApprovalRequest validates rec against the existing chain and returns the
// record key it must be appended under, plus the record with the pinned
// identities inherited from its create record. It never rewrites an existing
// key. An approval must carry the current identities in rec.Pins: any drift
// from what the request pinned refuses as stale (R5.3).
func PlanApprovalRequest(existing []ApprovalRequestRecord, rec ApprovalRequestRecord) (string, ApprovalRequestRecord, error) {
	if strings.TrimSpace(rec.ID) == "" {
		return "", rec, errors.New("approval request id is required")
	}
	latest, count := LatestApprovalRequest(existing, rec.ID)
	if !approvalTransitionAllowed(latest.Transition, rec.Transition) {
		if count == 0 {
			return "", rec, fmt.Errorf("approval request %s cannot be created as %q", rec.ID, rec.Transition)
		}
		return "", rec, fmt.Errorf("approval request %s is %s and cannot transition to %q", rec.ID, latest.Transition, rec.Transition)
	}
	// The chain is at most four transitions deep (draft, requested, approved,
	// revoked), so the unpadded sequence still sorts in append order.
	key := fmt.Sprintf("%s%s:%d", approvalRequestRecordPrefix, rec.ID, count)
	if count == 0 {
		return key, rec, validateApprovalCreate(rec)
	}
	if rec.Transition == ApprovalApproved {
		if err := approvalCurrent(latest, rec.Pins); err != nil {
			return "", rec, err
		}
	}
	if rec.Transition == ApprovalSuperseded {
		if strings.TrimSpace(rec.SupersededBy) == "" {
			return "", rec, fmt.Errorf("approval request %s must name the request that supersedes it", rec.ID)
		}
		if _, found := LatestApprovalRequest(existing, rec.SupersededBy); found == 0 {
			return "", rec, fmt.Errorf("approval request %s supersedes unknown request %q", rec.ID, rec.SupersededBy)
		}
	}
	// The transition inherits the request's governing identity verbatim; only
	// the reason and the supersession link are its own.
	rec.EntityKind, rec.EntityID, rec.EntityVersion = latest.EntityKind, latest.EntityID, latest.EntityVersion
	rec.Pins, rec.Requester, rec.ExpiresAt = latest.Pins, latest.Requester, latest.ExpiresAt
	return key, rec, nil
}

func approvalTransitionAllowed(from, to ApprovalTransition) bool {
	for _, allowed := range approvalTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

func validateApprovalCreate(rec ApprovalRequestRecord) error {
	switch rec.EntityKind {
	case ApprovalEntitySpec, ApprovalEntityTask, ApprovalEntityArtifact:
	default:
		return fmt.Errorf("unknown approval request entity kind %q", rec.EntityKind)
	}
	for _, field := range []struct{ name, value string }{
		{"entity id", rec.EntityID},
		{"artifact digest", rec.Pins.ArtifactDigest},
		{"transition-plan digest", rec.Pins.PlanDigest},
		{"config digest", rec.Pins.ConfigDigest},
		{"requester", rec.Requester},
		{"expiry", rec.ExpiresAt},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("approval request %s: %s is required", rec.ID, field.name)
		}
	}
	if _, err := time.Parse(time.RFC3339, rec.ExpiresAt); err != nil {
		return fmt.Errorf("approval request %s expiry must be RFC3339: %w", rec.ID, err)
	}
	if rec.Pins.StateRevision < 0 {
		return fmt.Errorf("approval request %s pins invalid state revision %d", rec.ID, rec.Pins.StateRevision)
	}
	return nil
}

// approvalCurrent refuses approval when the request has expired or when any
// pinned identity has drifted from current. Both refusals name the recovery,
// since the only legal continuation is a new or superseding request (R5.3).
func approvalCurrent(request ApprovalRequestRecord, current ApprovalPins) error {
	if expires, err := time.Parse(time.RFC3339, request.ExpiresAt); err == nil && Clock().After(expires) {
		return fmt.Errorf("approval request %s expired at %s: record the expired transition and open a new request", request.ID, request.ExpiresAt)
	}
	drift := ApprovalDrift(request.Pins, current)
	if len(drift) == 0 {
		return nil
	}
	return fmt.Errorf("approval request %s is stale (%s changed since it was pinned): open a new or superseding request", request.ID, strings.Join(drift, ", "))
}

// ApprovalDrift names the pinned identities that no longer match current, in
// stable order.
func ApprovalDrift(pinned, current ApprovalPins) []string {
	var drift []string
	if pinned.ArtifactDigest != current.ArtifactDigest {
		drift = append(drift, "artifact digest")
	}
	if pinned.StateRevision != current.StateRevision {
		drift = append(drift, "state revision")
	}
	if pinned.PlanDigest != current.PlanDigest {
		drift = append(drift, "transition-plan digest")
	}
	if pinned.ConfigDigest != current.ConfigDigest {
		drift = append(drift, "config digest")
	}
	return drift
}

// ApprovalRequestHistory projects the transitions into history events so the
// audit trail replays every approval state change, not just the final one
// (spec 13 R1). It reads records only; nothing is written.
func ApprovalRequestHistory(records map[string]json.RawMessage) ([]HistoryEvent, error) {
	requests, err := ReadApprovalRequests(records)
	if err != nil {
		return nil, err
	}
	events := make([]HistoryEvent, 0, len(requests))
	for seq, rec := range requests {
		reference := "request=" + rec.ID + " entity=" + rec.EntityKind + ":" + rec.EntityID
		if rec.SupersededBy != "" {
			reference += " superseded_by=" + rec.SupersededBy
		}
		events = append(events, HistoryEvent{
			Timestamp:  rec.Timestamp,
			Actor:      rec.Actor,
			Event:      "approval_request:" + string(rec.Transition),
			Reference:  reference,
			GitHead:    rec.GitHead,
			SourceRank: HistorySourceApprovalRequest,
			Seq:        seq,
		})
	}
	return events, nil
}
