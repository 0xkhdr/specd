package core

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	contextpkg "github.com/0xkhdr/specd/internal/context"
)

// ACP envelope protocol version and the size/length limits enforced when
// validating an envelope (see ValidateACPEnvelope).
const (
	ACPVersion             = "1"
	ACPMaxEnvelopeBytes    = 256 * 1024
	ACPMaxTextBytes        = 16 * 1024
	ACPMaxListItems        = 128
	ACPMaxPathBytes        = 1024
	ACPVerificationRefSize = 256
)

// ACPMessageType identifies the kind of message carried by an ACPEnvelope.
type ACPMessageType string

// ACP message type values, naming the lifecycle events that flow between a
// Brain and its Pinky workers.
const (
	ACPMessageMission   ACPMessageType = "mission"
	ACPMessageAccepted  ACPMessageType = "accepted"
	ACPMessageHeartbeat ACPMessageType = "heartbeat"
	ACPMessageProgress  ACPMessageType = "progress"
	ACPMessageEvidence  ACPMessageType = "evidence"
	ACPMessageBlocker   ACPMessageType = "blocker"
	ACPMessageQuery     ACPMessageType = "query"
	ACPMessageDirective ACPMessageType = "directive"
	ACPMessageCancelled ACPMessageType = "cancelled"
	// ACPMessageCheckpoint is a worker's durable mid-task progress marker (R1).
	// Like progress it flows pinky -> brain, but it also pairs with an on-disk
	// CheckpointRecord and releases the worker's lease so the Brain can resume.
	ACPMessageCheckpoint ACPMessageType = "checkpoint"
	// ACPMessageResume is a returning worker's signal that a suspended lease is
	// active again (R3). It flows pinky -> brain and records how long the worker
	// was suspended so the Brain (and observers) can attribute the pause.
	ACPMessageResume ACPMessageType = "resume"
)

var (
	acpIDRE    = regexp.MustCompile(`^[a-f0-9]{32}$`)
	acpPartyRE = regexp.MustCompile(`^(brain|pinky-[a-z0-9][a-z0-9-]{0,62})$`)
	// acpTaskIDRE accepts both execution task IDs (T<n>, parsed from tasks.md)
	// and reserved authoring work IDs (A<n>, synthesized for planning-phase
	// artifact missions — see authoringWorkID). Authoring IDs never originate
	// from tasks.md, so widening the shared matcher does not loosen any
	// tasks.md-sourced validation in practice.
	acpTaskIDRE  = regexp.MustCompile(`^[TA][0-9]+$`)
	acpDigestRE  = regexp.MustCompile(`^[a-f0-9]{64}$`)
	acpActionSet = sliceToSet([]string{"retry", "cancel", "reassign", "escalate", "continue"})
	// acpAuthorityActionSet enumerates worker capabilities carried in a mission's
	// authority grant. These are distinct from directive verbs in acpActionSet and
	// must stay in sync with pinkyAllowedActions.
	acpAuthorityActionSet = sliceToSet([]string{"read", "edit", "verify", "report"})
	acpMessageSet         = map[ACPMessageType]bool{
		ACPMessageMission: true, ACPMessageAccepted: true, ACPMessageHeartbeat: true,
		ACPMessageProgress: true, ACPMessageEvidence: true, ACPMessageBlocker: true,
		ACPMessageQuery: true, ACPMessageDirective: true, ACPMessageCancelled: true,
		ACPMessageCheckpoint: true, ACPMessageResume: true,
	}
)

// ACPEnvelope is the transport-level wrapper for every ACP message exchanged
// between a Brain and its Pinky workers. Payload carries the type-specific
// body (see ACPMessageType) as raw JSON, decoded separately by the caller.
type ACPEnvelope struct {
	Version   string                 `json:"version"`
	MessageID string                 `json:"messageId"`
	SessionID string                 `json:"sessionId"`
	Sequence  uint64                 `json:"sequence"`
	CreatedAt string                 `json:"createdAt"`
	ExpiresAt string                 `json:"expiresAt"`
	Type      ACPMessageType         `json:"type"`
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Spec      string                 `json:"spec"`
	Task      string                 `json:"task,omitempty"`
	Attempt   int                    `json:"attempt"`
	InReplyTo string                 `json:"inReplyTo,omitempty"`
	Payload   json.RawMessage        `json:"payload"`
	Decision  *OrchestrationDecision `json:"decision,omitempty"`
}

// ACPAuthority declares the access scope granted to a worker for a mission:
// whether it is read-only and which actions it is permitted to take.
type ACPAuthority struct {
	ReadOnly       bool     `json:"readOnly"`
	AllowedActions []string `json:"allowedActions"`
}

// ACPMissionPayload is the body of an ACPMessageMission message: the full
// dispatch a Brain sends a worker, including its contract, file scope,
// acceptance and verify commands, dependencies, and granted ACPAuthority.
type ACPMissionPayload struct {
	DispatchDigest  string                            `json:"dispatchDigest"`
	Role            string                            `json:"role"`
	ContextCommand  string                            `json:"contextCommand"`
	ContextManifest contextpkg.MissionContextManifest `json:"contextManifest,omitempty"`
	Contract        string                            `json:"contract"`
	Files           []string                          `json:"files"`
	Acceptance      string                            `json:"acceptance"`
	VerifyCommand   string                            `json:"verifyCommand"`
	Dependencies    []string                          `json:"dependencies"`
	Authority       ACPAuthority                      `json:"authority"`
}

// ACPAcceptedPayload is the body of an accepted-message reply, identifying
// the worker that accepted the mission.
type ACPAcceptedPayload struct {
	WorkerID string `json:"workerId"`
}

// ACPHeartbeatPayload is the body of a heartbeat message a worker sends to
// signal it is still alive, along with its current status.
type ACPHeartbeatPayload struct {
	WorkerID string `json:"workerId"`
	Status   string `json:"status"`
}

// ACPProgressPayload is the body of a progress message reporting how far a
// worker has gotten on its mission.
type ACPProgressPayload struct {
	Percent int    `json:"percent"`
	Message string `json:"message"`
	// LastReport is the server-side write time of this progress record (RFC3339).
	// It is stamped by RecordPinkyProgress from the host clock, never from a
	// worker-supplied value, so it cannot be spoofed into the future. It lets the
	// driver distinguish slow-but-progressing work from a true stall (R6).
	// omitempty keeps pre-resilience progress events byte-stable.
	LastReport string `json:"lastReport,omitempty"`
}

// ACPEvidencePayload is the body of an evidence message a worker sends on
// completion, summarizing the verification ref, changed files, git head,
// elapsed time, and any host token/cost usage reported for the mission.
type ACPEvidencePayload struct {
	VerificationRef string   `json:"verificationRef"`
	Summary         string   `json:"summary"`
	ChangedFiles    []string `json:"changedFiles"`
	GitHead         string   `json:"gitHead,omitempty"`
	DurationMs      int64    `json:"durationMs,omitempty"`
	HostTokens      int      `json:"hostTokens,omitempty"`
	HostCost        string   `json:"hostCost,omitempty"`
}

// ACPBlockerPayload is the body of a blocker message a worker sends when it
// cannot proceed, giving the reason it is stuck.
type ACPBlockerPayload struct {
	Reason string `json:"reason"`
}

// ACPQueryPayload is the body of a query message a worker sends to ask the
// Brain a free-text question.
type ACPQueryPayload struct {
	Text string `json:"text"`
}

// ACPDirectivePayload is the body of a directive message the Brain sends to a
// worker, naming the action to take and the reason for it.
type ACPDirectivePayload struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

// ACPCancelledPayload is the body of a cancelled message confirming a
// mission was cancelled, with the reason for the cancellation.
type ACPCancelledPayload struct {
	Reason string `json:"reason"`
}

// ACPCheckpointPayload is the event-side summary of a checkpoint. The full
// resume payload (context manifest, working notes) lives in the on-disk
// CheckpointRecord; the event carries only the lean, bounded fields the Brain
// and observers need: how far the work got, why it stopped, and the file/git
// frontier it reached.
type ACPCheckpointPayload struct {
	Percent      int      `json:"percent"`
	Reason       string   `json:"reason,omitempty"`
	ChangedFiles []string `json:"changedFiles,omitempty"`
	GitHead      string   `json:"gitHead,omitempty"`
}

// ACPResumePayload is the event-side record of a worker returning from a
// suspended lease (R3): how long it was away and why it had suspended.
type ACPResumePayload struct {
	SuspendedSeconds int    `json:"suspendedSeconds"`
	Reason           string `json:"reason,omitempty"`
}

// NewACPID generates a new random 32-character hex identifier suitable for
// use as an ACP message ID.
func NewACPID() (string, error) {
	var raw [16]byte
	if _, err := io.ReadFull(rand.Reader, raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

// NewACPEnvelope builds an ACPEnvelope of the given message type by
// marshaling payload to JSON and setting Version to ACPVersion. Callers fill
// in any remaining envelope fields (ids, session, routing) before sending it.
func NewACPEnvelope(messageType ACPMessageType, payload any) (ACPEnvelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return ACPEnvelope{}, err
	}
	return ACPEnvelope{Version: ACPVersion, Type: messageType, Payload: raw}, nil
}

// ParseACPEnvelope decodes raw JSON into an ACPEnvelope, rejecting empty
// input, input over ACPMaxEnvelopeBytes, and any envelope that fails
// ValidateACPEnvelope.
func ParseACPEnvelope(raw []byte) (ACPEnvelope, error) {
	if len(raw) == 0 {
		return ACPEnvelope{}, fmt.Errorf("acp: empty envelope")
	}
	if len(raw) > ACPMaxEnvelopeBytes {
		return ACPEnvelope{}, fmt.Errorf("acp: envelope exceeds %d bytes", ACPMaxEnvelopeBytes)
	}
	var envelope ACPEnvelope
	if err := decodeACPStrict(raw, &envelope); err != nil {
		return ACPEnvelope{}, fmt.Errorf("acp: invalid envelope: %w", err)
	}
	if err := ValidateACPEnvelope(envelope); err != nil {
		return ACPEnvelope{}, err
	}
	return envelope, nil
}

// ValidateACPEnvelope checks an envelope's version, IDs, sizes, and
// type-specific payload constraints, returning an error describing the first
// violation found.
//
//nolint:gocyclo // pre-existing complexity debt, out of scope for spec S3 — tracked for a future cleanup pass
func ValidateACPEnvelope(envelope ACPEnvelope) error {
	if envelope.Version != ACPVersion {
		return fmt.Errorf("acp: unsupported version %q", envelope.Version)
	}
	if !acpIDRE.MatchString(envelope.MessageID) {
		return fmt.Errorf("acp: invalid messageId")
	}
	if !acpIDRE.MatchString(envelope.SessionID) {
		return fmt.Errorf("acp: invalid sessionId")
	}
	if envelope.InReplyTo != "" && !acpIDRE.MatchString(envelope.InReplyTo) {
		return fmt.Errorf("acp: invalid inReplyTo")
	}
	if envelope.Sequence == 0 {
		return fmt.Errorf("acp: sequence must be greater than zero")
	}
	if !acpMessageSet[envelope.Type] {
		return fmt.Errorf("acp: unsupported type %q", envelope.Type)
	}
	if !acpPartyRE.MatchString(envelope.From) {
		return fmt.Errorf("acp: invalid from party")
	}
	if !acpPartyRE.MatchString(envelope.To) {
		return fmt.Errorf("acp: invalid to party")
	}
	if err := validateACPDirection(envelope.Type, envelope.From, envelope.To); err != nil {
		return err
	}
	if err := ValidateSlug(envelope.Spec); err != nil {
		return fmt.Errorf("acp: invalid spec: %w", err)
	}
	if envelope.Task != "" && !acpTaskIDRE.MatchString(envelope.Task) {
		return fmt.Errorf("acp: invalid task")
	}
	if envelope.Attempt < 1 {
		return fmt.Errorf("acp: attempt must be greater than zero")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, envelope.CreatedAt)
	if err != nil {
		return fmt.Errorf("acp: invalid createdAt")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, envelope.ExpiresAt)
	if err != nil {
		return fmt.Errorf("acp: invalid expiresAt")
	}
	if !expiresAt.After(createdAt) {
		return fmt.Errorf("acp: expiresAt must be after createdAt")
	}
	if len(envelope.Payload) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Payload), []byte("null")) {
		return fmt.Errorf("acp: payload is required")
	}
	if err := validateACPPayload(envelope.Type, envelope.Task, envelope.Payload); err != nil {
		return fmt.Errorf("acp: %s payload: %w", envelope.Type, err)
	}
	return nil
}

func validateACPDirection(messageType ACPMessageType, from, to string) error {
	fromBrain := from == "brain"
	toBrain := to == "brain"
	switch messageType {
	case ACPMessageMission, ACPMessageDirective:
		if !fromBrain || toBrain {
			return fmt.Errorf("acp: %s must be sent from brain to pinky", messageType)
		}
	case ACPMessageAccepted, ACPMessageProgress, ACPMessageEvidence, ACPMessageBlocker, ACPMessageQuery, ACPMessageCancelled, ACPMessageCheckpoint, ACPMessageResume:
		if fromBrain || !toBrain {
			return fmt.Errorf("acp: %s must be sent from pinky to brain", messageType)
		}
	case ACPMessageHeartbeat:
		if fromBrain == toBrain {
			return fmt.Errorf("acp: heartbeat must be sent between brain and pinky")
		}
	}
	return nil
}

func validateACPPayload(messageType ACPMessageType, task string, raw []byte) error {
	switch messageType {
	case ACPMessageMission:
		return validateACPMissionPayload(task, raw)
	case ACPMessageAccepted:
		return validateACPAcceptedPayload(raw)
	case ACPMessageHeartbeat:
		return validateACPHeartbeatPayload(raw)
	case ACPMessageProgress:
		return validateACPProgressPayload(raw)
	case ACPMessageEvidence:
		return validateACPEvidencePayload(task, raw)
	case ACPMessageBlocker:
		return validateACPBlockerPayload(raw)
	case ACPMessageQuery:
		return validateACPQueryPayload(raw)
	case ACPMessageDirective:
		return validateACPDirectivePayload(raw)
	case ACPMessageCancelled:
		return validateACPCancelledPayload(raw)
	case ACPMessageCheckpoint:
		return validateACPCheckpointPayload(task, raw)
	case ACPMessageResume:
		return validateACPResumePayload(task, raw)
	default:
		return fmt.Errorf("unsupported type %q", messageType)
	}
}

func validateACPMissionPayload(task string, raw []byte) error {
	var payload ACPMissionPayload
	if err := decodeACPStrict(raw, &payload); err != nil {
		return err
	}
	if task == "" {
		return fmt.Errorf("task is required")
	}
	if !acpDigestRE.MatchString(payload.DispatchDigest) {
		return fmt.Errorf("invalid dispatchDigest")
	}
	if !IsValidRole(payload.Role) {
		return fmt.Errorf("invalid role %q", payload.Role)
	}
	if err := validateACPText("contextCommand", payload.ContextCommand, true); err != nil {
		return err
	}
	if err := validateMissionContextManifest(payload.ContextManifest, false); err != nil {
		return err
	}
	if err := validateACPText("contract", payload.Contract, true); err != nil {
		return err
	}
	if err := validateACPText("acceptance", payload.Acceptance, true); err != nil {
		return err
	}
	if err := validateACPText("verifyCommand", payload.VerifyCommand, true); err != nil {
		return err
	}
	if err := validateACPPaths("files", payload.Files); err != nil {
		return err
	}
	if err := validateACPTaskIDs("dependencies", payload.Dependencies); err != nil {
		return err
	}
	return validateACPAuthority(payload.Authority)
}

func validateACPAuthority(authority ACPAuthority) error {
	if len(authority.AllowedActions) == 0 || len(authority.AllowedActions) > ACPMaxListItems {
		return fmt.Errorf("authority.allowedActions must contain 1..%d items", ACPMaxListItems)
	}
	for _, action := range authority.AllowedActions {
		if !acpAuthorityActionSet[action] {
			return fmt.Errorf("invalid authority action %q", action)
		}
	}
	return nil
}

func validateACPAcceptedPayload(raw []byte) error {
	var payload ACPAcceptedPayload
	if err := decodeACPStrict(raw, &payload); err != nil {
		return err
	}
	return validateACPWorkerID(payload.WorkerID)
}

func validateACPHeartbeatPayload(raw []byte) error {
	var payload ACPHeartbeatPayload
	if err := decodeACPStrict(raw, &payload); err != nil {
		return err
	}
	if err := validateACPWorkerID(payload.WorkerID); err != nil {
		return err
	}
	return validateACPText("status", payload.Status, true)
}

func validateACPProgressPayload(raw []byte) error {
	var payload ACPProgressPayload
	if err := decodeACPStrict(raw, &payload); err != nil {
		return err
	}
	if payload.Percent < 0 || payload.Percent > 100 {
		return fmt.Errorf("percent must be between 0 and 100")
	}
	return validateACPText("message", payload.Message, true)
}

func validateACPEvidencePayload(task string, raw []byte) error {
	var payload ACPEvidencePayload
	if err := decodeACPStrict(raw, &payload); err != nil {
		return err
	}
	if task == "" {
		return fmt.Errorf("task is required")
	}
	if len(payload.VerificationRef) == 0 || len(payload.VerificationRef) > ACPVerificationRefSize {
		return fmt.Errorf("verificationRef must contain 1..%d bytes", ACPVerificationRefSize)
	}
	if err := validateACPText("summary", payload.Summary, true); err != nil {
		return err
	}
	if err := validateACPPaths("changedFiles", payload.ChangedFiles); err != nil {
		return err
	}
	if payload.DurationMs < 0 || payload.HostTokens < 0 {
		return fmt.Errorf("durationMs and hostTokens must be non-negative")
	}
	return validateACPText("hostCost", payload.HostCost, false)
}

func validateACPBlockerPayload(raw []byte) error {
	var payload ACPBlockerPayload
	if err := decodeACPStrict(raw, &payload); err != nil {
		return err
	}
	return validateACPText("reason", payload.Reason, true)
}

func validateACPQueryPayload(raw []byte) error {
	var payload ACPQueryPayload
	if err := decodeACPStrict(raw, &payload); err != nil {
		return err
	}
	return validateACPText("text", payload.Text, true)
}

func validateACPDirectivePayload(raw []byte) error {
	var payload ACPDirectivePayload
	if err := decodeACPStrict(raw, &payload); err != nil {
		return err
	}
	if !acpActionSet[payload.Action] {
		return fmt.Errorf("invalid action %q", payload.Action)
	}
	return validateACPText("reason", payload.Reason, true)
}

func validateACPCancelledPayload(raw []byte) error {
	var payload ACPCancelledPayload
	if err := decodeACPStrict(raw, &payload); err != nil {
		return err
	}
	return validateACPText("reason", payload.Reason, true)
}

func validateACPCheckpointPayload(task string, raw []byte) error {
	var payload ACPCheckpointPayload
	if err := decodeACPStrict(raw, &payload); err != nil {
		return err
	}
	if task == "" {
		return fmt.Errorf("task is required")
	}
	if payload.Percent < 0 || payload.Percent > 100 {
		return fmt.Errorf("percent must be between 0 and 100")
	}
	if err := validateACPText("reason", payload.Reason, false); err != nil {
		return err
	}
	if err := validateACPPaths("changedFiles", payload.ChangedFiles); err != nil {
		return err
	}
	return validateACPText("gitHead", payload.GitHead, false)
}

func validateACPResumePayload(task string, raw []byte) error {
	var payload ACPResumePayload
	if err := decodeACPStrict(raw, &payload); err != nil {
		return err
	}
	if task == "" {
		return fmt.Errorf("task is required")
	}
	if payload.SuspendedSeconds < 0 {
		return fmt.Errorf("suspendedSeconds must be non-negative")
	}
	return validateACPText("reason", payload.Reason, false)
}

func validateACPWorkerID(workerID string) error {
	if !acpPartyRE.MatchString("pinky-" + workerID) {
		return fmt.Errorf("invalid workerId")
	}
	return nil
}

func decodeACPStrict(raw []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func validateACPText(name, value string, required bool) error {
	if required && strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	if len(value) > ACPMaxTextBytes {
		return fmt.Errorf("%s exceeds %d bytes", name, ACPMaxTextBytes)
	}
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("%s contains NUL", name)
	}
	return nil
}

func validateACPPaths(name string, values []string) error {
	if len(values) > ACPMaxListItems {
		return fmt.Errorf("%s exceeds %d items", name, ACPMaxListItems)
	}
	for _, value := range values {
		if value == "" || len(value) > ACPMaxPathBytes || strings.IndexByte(value, 0) >= 0 {
			return fmt.Errorf("%s contains an invalid path", name)
		}
	}
	return nil
}

func validateACPTaskIDs(name string, values []string) error {
	if len(values) > ACPMaxListItems {
		return fmt.Errorf("%s exceeds %d items", name, ACPMaxListItems)
	}
	for _, value := range values {
		if !acpTaskIDRE.MatchString(value) {
			return fmt.Errorf("%s contains invalid task %q", name, value)
		}
	}
	return nil
}
