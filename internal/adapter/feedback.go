package adapter

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// FeedbackSchemaV1 versions runtime-feedback payload semantics independently
// from adapter, CLI, and on-disk state schemas.
const FeedbackSchemaV1 = "release_feedback/v1"

const feedbackCapabilityV1 = "release-feedback/v1"

// ReleaseFeedbackV1 is bounded, untrusted runtime data. It can identify and
// link successor maintenance work; it carries no command or mutation field.
type ReleaseFeedbackV1 struct {
	SchemaVersion string   `json:"schema_version"`
	SourceSpec    string   `json:"source_spec"`
	SuccessorSpec string   `json:"successor_spec"`
	ReleaseID     string   `json:"release_id"`
	Environment   string   `json:"environment"`
	GitHead       string   `json:"git_head"`
	ObservedAt    string   `json:"observed_at"`
	EvidenceRefs  []string `json:"evidence_refs"`
}

func validateFeedback(f ReleaseFeedbackV1) error {
	if f.SchemaVersion != FeedbackSchemaV1 {
		return newFinding(ErrUnknownVersion, "payload.schema_version", "unsupported feedback schema_version "+f.SchemaVersion)
	}
	if err := core.ValidateSlug(f.SourceSpec); err != nil {
		return fmt.Errorf("source spec: %w", err)
	}
	if err := core.ValidateSlug(f.SuccessorSpec); err != nil {
		return fmt.Errorf("successor spec: %w", err)
	}
	if f.SourceSpec == f.SuccessorSpec {
		return newFinding(ErrInvalidValue, "successor_spec", "successor must differ from completed source")
	}
	if f.ReleaseID == "" || f.Environment == "" || f.GitHead == "" {
		return newFinding(ErrInvalidValue, "release_id", "release_id, environment, and git_head are required")
	}
	if _, err := time.Parse(time.RFC3339, f.ObservedAt); err != nil {
		return newFinding(ErrInvalidValue, "observed_at", "observed_at must be RFC3339")
	}
	if len(f.EvidenceRefs) == 0 || len(f.EvidenceRefs) > 32 {
		return newFinding(ErrInvalidValue, "evidence_refs", "feedback requires 1-32 evidence references")
	}
	seen := map[string]bool{}
	for _, ref := range f.EvidenceRefs {
		u, err := url.Parse(ref)
		if err != nil || u.Scheme == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(ref) > 256 || strings.ContainsAny(ref, "\x00\r\n") || seen[ref] {
			return newFinding(ErrInvalidValue, "evidence_refs", "feedback evidence reference is unsafe or duplicate")
		}
		seen[ref] = true
	}
	return nil
}

// FeedbackRequest wraps runtime feedback in the common boundary envelope.
func FeedbackRequest(f ReleaseFeedbackV1, requestID, correlationID, adapterName string) (Request, error) {
	if err := validateFeedback(f); err != nil {
		return Request{}, err
	}
	sort.Strings(f.EvidenceRefs)
	payload, err := json.Marshal(f)
	if err != nil {
		return Request{}, err
	}
	req := Request{
		SchemaVersion: SchemaVersion, Kind: "release_feedback.request", RequestID: requestID, CorrelationID: correlationID,
		Subject:              Subject{SpecSlug: f.SourceSpec, GitHead: f.GitHead, ReleaseID: f.ReleaseID, Environment: f.Environment},
		CapabilitiesRequired: []string{feedbackCapabilityV1}, StartedAt: f.ObservedAt, AdapterName: adapterName, Payload: payload,
	}
	if err := req.Validate(); err != nil {
		return Request{}, err
	}
	return req, nil
}

// FeedbackFromRequest decodes strictly and verifies duplicated release pins.
// Returned value remains data; core owns whether a maintenance edge is valid.
func FeedbackFromRequest(req Request) (ReleaseFeedbackV1, error) {
	if err := req.Validate(); err != nil {
		return ReleaseFeedbackV1{}, err
	}
	if req.Kind != "release_feedback.request" {
		return ReleaseFeedbackV1{}, newFinding(ErrUnknownKind, "kind", "expected release_feedback.request")
	}
	if len(req.CapabilitiesRequired) != 1 || req.CapabilitiesRequired[0] != feedbackCapabilityV1 {
		return ReleaseFeedbackV1{}, newFinding(ErrInvalidValue, "capabilities_required", "release-feedback/v1 capability is required")
	}
	var f ReleaseFeedbackV1
	if err := decode(req.Payload, &f); err != nil {
		return ReleaseFeedbackV1{}, err
	}
	if err := validateFeedback(f); err != nil {
		return ReleaseFeedbackV1{}, err
	}
	if req.Subject.SpecSlug != f.SourceSpec || req.Subject.ReleaseID != f.ReleaseID || req.Subject.Environment != f.Environment || req.Subject.GitHead != f.GitHead || req.StartedAt != f.ObservedAt {
		return ReleaseFeedbackV1{}, newFinding(ErrIdentityMismatch, "feedback", "feedback payload does not match envelope release identity")
	}
	sort.Strings(f.EvidenceRefs)
	return f, nil
}
