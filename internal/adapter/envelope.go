package adapter

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// SchemaVersion is the single frozen version of the common adapter envelope.
// The common core is co-designed from Domains 04/05/07/08 P0 field demands and
// frozen once here; payload-specific fields are additive extensions carried in
// Payload and owned by the consuming domain (design.md "Common envelope fields").
const SchemaVersion = "adapter/v1"

// MaxInlineBytes bounds opt-in inline content on a reference (R4.3). By default
// references carry only a digest; inline bytes are opt-in and size-bounded.
const MaxInlineBytes = 64 * 1024

// kindRE constrains kind to "<domain>.request" or "<domain>.result". The suffix
// distinguishes request from result; the domain segment is open so a consuming
// domain can name its payload (eval, mission, deploy, telemetry) without a code
// change here, while an unknown shape still fails closed (R2.2).
var kindRE = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*\.(request|result)$`)

// Ref is a content reference: a named digest with a data class, and optional
// size-bounded inline content (R4.3).
type Ref struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
	Class  Class  `json:"class"`
	Inline string `json:"inline,omitempty"`
}

// Subject is the pinned work identity shared by a request and its result.
type Subject struct {
	SpecSlug    string `json:"spec_slug,omitempty"`
	TaskID      string `json:"task_id,omitempty"`
	MissionID   string `json:"mission_id,omitempty"`
	GitHead     string `json:"git_head,omitempty"`
	ReleaseID   string `json:"release_id,omitempty"`
	Environment string `json:"environment,omitempty"`
}

// Limits bounds an adapter invocation deterministically.
type Limits struct {
	TimeoutMS   int64 `json:"timeout_ms,omitempty"`
	OutputBytes int64 `json:"output_bytes,omitempty"`
}

// Measurements are named numeric results (scores, tokens, durations). Map keys
// are emitted in sorted order by encoding/json, keeping output byte-stable.
type Measurements map[string]float64

// Status is the stable result class (R2.4).
type Status string

const (
	StatusSucceeded   Status = "succeeded"
	StatusRejected    Status = "rejected"
	StatusFailed      Status = "failed"
	StatusTimedOut    Status = "timed_out"
	StatusUnavailable Status = "unavailable"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusSucceeded, StatusRejected, StatusFailed, StatusTimedOut, StatusUnavailable:
		return true
	default:
		return false
	}
}

// Retryable deterministically bounds retry behaviour (R2.4): only a transient
// timeout or unavailability may be retried; a rejection or failure may not.
func (s Status) Retryable() bool {
	return s == StatusTimedOut || s == StatusUnavailable
}

// ExitClass distinguishes the ways a runner invocation can end (R2.4/R6.2).
type ExitClass string

const (
	ExitOK            ExitClass = "ok"
	ExitMissingBinary ExitClass = "missing_binary"
	ExitTimeout       ExitClass = "timeout"
	ExitOversized     ExitClass = "oversized_output"
	ExitMalformed     ExitClass = "malformed_output"
	ExitNonZero       ExitClass = "nonzero_exit"
)

// Valid reports whether e is a known exit class.
func (e ExitClass) Valid() bool {
	switch e {
	case ExitOK, ExitMissingBinary, ExitTimeout, ExitOversized, ExitMalformed, ExitNonZero:
		return true
	default:
		return false
	}
}

// ErrorClass is a stable classification for a rejected envelope (R2.2).
type ErrorClass string

const (
	ErrMalformed        ErrorClass = "malformed"
	ErrUnknownVersion   ErrorClass = "unknown_schema_version"
	ErrUnknownKind      ErrorClass = "unknown_kind"
	ErrUnknownField     ErrorClass = "unknown_field"
	ErrInvalidValue     ErrorClass = "invalid_field_value"
	ErrIdentityMismatch ErrorClass = "identity_mismatch"
)

// Finding is a typed, stable error explaining why an envelope failed closed.
type Finding struct {
	Class   ErrorClass `json:"class"`
	Field   string     `json:"field,omitempty"`
	Message string     `json:"message"`
}

func (f *Finding) Error() string {
	if f.Field != "" {
		return string(f.Class) + " [" + f.Field + "]: " + f.Message
	}
	return string(f.Class) + ": " + f.Message
}

func newFinding(class ErrorClass, field, msg string) *Finding {
	return &Finding{Class: class, Field: field, Message: msg}
}

// Request is the versioned envelope core generates for an adapter (R2.1).
type Request struct {
	SchemaVersion        string          `json:"schema_version"`
	Kind                 string          `json:"kind"`
	RequestID            string          `json:"request_id"`
	CorrelationID        string          `json:"correlation_id"`
	Subject              Subject         `json:"subject"`
	Actor                string          `json:"actor,omitempty"`
	AuthorityRef         string          `json:"authority_ref,omitempty"`
	InputRefs            []Ref           `json:"input_refs,omitempty"`
	CapabilitiesRequired []string        `json:"capabilities_required,omitempty"`
	Limits               Limits          `json:"limits"`
	StartedAt            string          `json:"started_at"`
	AdapterName          string          `json:"adapter_name"`
	Payload              json.RawMessage `json:"payload,omitempty"`
}

// Result is the versioned envelope an adapter returns (R2.1).
type Result struct {
	SchemaVersion       string            `json:"schema_version"`
	Kind                string            `json:"kind"`
	RequestID           string            `json:"request_id"`
	CorrelationID       string            `json:"correlation_id"`
	Subject             Subject           `json:"subject"`
	AdapterName         string            `json:"adapter_name"`
	AdapterVersion      string            `json:"adapter_version"`
	CapabilitiesOffered []string          `json:"capabilities_offered,omitempty"`
	Status              Status            `json:"status"`
	ExitClass           ExitClass         `json:"exit_class"`
	Retryable           bool              `json:"retryable"`
	OutputRefs          []Ref             `json:"output_refs,omitempty"`
	EvidenceRefs        []Ref             `json:"evidence_refs,omitempty"`
	Measurements        Measurements      `json:"measurements,omitempty"`
	Redactions          []Redaction       `json:"redactions,omitempty"`
	InputDigests        map[string]string `json:"input_digests,omitempty"`
	StartedAt           string            `json:"started_at"`
	FinishedAt          string            `json:"finished_at"`
	Payload             json.RawMessage   `json:"payload,omitempty"`
}

// canonical encodes v deterministically: struct fields keep declaration order,
// map keys are sorted by encoding/json, HTML escaping is disabled, and the
// trailing newline the encoder appends is trimmed. Identical values therefore
// produce identical bytes (R2.3).
func canonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Canonical returns the byte-stable encoding of the request.
func (r Request) Canonical() ([]byte, error) { return canonical(r) }

// Canonical returns the byte-stable encoding of the result.
func (r Result) Canonical() ([]byte, error) { return canonical(r) }

// Digest returns the content address (hex SHA-256) of the canonical encoding.
func (r Request) Digest() (string, error) {
	b, err := r.Canonical()
	if err != nil {
		return "", err
	}
	return core.Digest(b), nil
}

// Digest returns the content address (hex SHA-256) of the canonical encoding.
func (r Result) Digest() (string, error) {
	b, err := r.Canonical()
	if err != nil {
		return "", err
	}
	return core.Digest(b), nil
}

func validateKind(kind, suffix string) error {
	if !kindRE.MatchString(kind) {
		return newFinding(ErrUnknownKind, "kind", "kind must match <domain>."+suffix)
	}
	if !strings.HasSuffix(kind, "."+suffix) {
		return newFinding(ErrUnknownKind, "kind", "kind must end in ."+suffix)
	}
	return nil
}

func validateVersion(v string) error {
	if v != SchemaVersion {
		return newFinding(ErrUnknownVersion, "schema_version", "unsupported schema_version "+v)
	}
	return nil
}

func validateRefs(refs []Ref) error {
	for _, r := range refs {
		if !r.Class.Valid() {
			return newFinding(ErrInvalidValue, "class", "unknown data class "+string(r.Class))
		}
		if len(r.Inline) > MaxInlineBytes {
			return newFinding(ErrInvalidValue, "inline", "inline content exceeds size bound for "+r.Name)
		}
	}
	return nil
}

// Validate checks the request against the frozen schema (R2.1/R2.2/R4.3).
func (r Request) Validate() error {
	if err := validateVersion(r.SchemaVersion); err != nil {
		return err
	}
	if err := validateKind(r.Kind, "request"); err != nil {
		return err
	}
	if r.RequestID == "" || r.CorrelationID == "" {
		return newFinding(ErrInvalidValue, "request_id", "request_id and correlation_id are required")
	}
	if r.StartedAt == "" {
		return newFinding(ErrInvalidValue, "started_at", "started_at is required")
	}
	return validateRefs(r.InputRefs)
}

// Validate checks the result against the frozen schema (R2.1/R2.2/R2.4).
func (r Result) Validate() error {
	if err := validateVersion(r.SchemaVersion); err != nil {
		return err
	}
	if err := validateKind(r.Kind, "result"); err != nil {
		return err
	}
	if r.RequestID == "" || r.CorrelationID == "" {
		return newFinding(ErrInvalidValue, "request_id", "request_id and correlation_id are required")
	}
	if !r.Status.Valid() {
		return newFinding(ErrInvalidValue, "status", "unknown status "+string(r.Status))
	}
	if !r.ExitClass.Valid() {
		return newFinding(ErrInvalidValue, "exit_class", "unknown exit_class "+string(r.ExitClass))
	}
	// Retryable is deterministically bounded by status (R2.4): a result may not
	// claim retryable when its status is not itself retryable.
	if r.Retryable && !r.Status.Retryable() {
		return newFinding(ErrInvalidValue, "retryable", "status "+string(r.Status)+" is not retryable")
	}
	if r.StartedAt == "" || r.FinishedAt == "" {
		return newFinding(ErrInvalidValue, "finished_at", "started_at and finished_at are required")
	}
	if err := validateRefs(r.OutputRefs); err != nil {
		return err
	}
	return validateRefs(r.EvidenceRefs)
}

func decode(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return newFinding(ErrUnknownField, "", err.Error())
		}
		return newFinding(ErrMalformed, "", err.Error())
	}
	return nil
}

// DecodeRequest parses and validates a request envelope, failing closed with a
// typed Finding on any unknown version/kind/field or malformed input (R2.2).
func DecodeRequest(data []byte) (Request, error) {
	var r Request
	if err := decode(data, &r); err != nil {
		return Request{}, err
	}
	if err := r.Validate(); err != nil {
		return r, err
	}
	return r, nil
}

// DecodeResult parses and validates a result envelope, failing closed with a
// typed Finding on any unknown version/kind/field or malformed input (R2.2).
func DecodeResult(data []byte) (Result, error) {
	var r Result
	if err := decode(data, &r); err != nil {
		return Result{}, err
	}
	if err := r.Validate(); err != nil {
		return r, err
	}
	return r, nil
}

// DecodeResultValue validates an already-constructed result value, so callers
// building a result in-process share the same fail-closed schema check.
func DecodeResultValue(r Result) (Result, error) {
	if err := r.Validate(); err != nil {
		return r, err
	}
	return r, nil
}
