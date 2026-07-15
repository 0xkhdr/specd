package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// PinkyProgressReport is a worker's progress update for one in-flight task
// attempt: a percent-complete figure and free-form status message, recorded
// as an ACP progress event.
type PinkyProgressReport struct {
	SessionID string
	WorkerID  string
	Spec      string
	TaskID    string
	Attempt   int
	Percent   int
	Message   string
}

// PinkyBlockerReport is a worker's report that a task attempt is blocked,
// with a free-form reason, recorded as an ACP blocked event.
type PinkyBlockerReport struct {
	SessionID string
	WorkerID  string
	Spec      string
	TaskID    string
	Attempt   int
	Reason    string
}

// PinkyQueryReport is a worker's question to the Brain about a task attempt,
// recorded as an ACP query event awaiting a directive reply.
type PinkyQueryReport struct {
	SessionID string
	WorkerID  string
	Spec      string
	TaskID    string
	Attempt   int
	Text      string
}

// BrainDirective is the Brain's instruction back to a worker, recorded as an
// ACP directive event. InReplyTo links it to the message ID of the query or
// report it answers.
type BrainDirective struct {
	SessionID string
	WorkerID  string
	Spec      string
	TaskID    string
	Attempt   int
	Action    string
	Reason    string
	InReplyTo string
}

// PinkyInbox is the set of directives and other ACP events addressed to one
// worker, read back from the session's event log.
type PinkyInbox struct {
	SessionID  string        `json:"sessionId"`
	WorkerID   string        `json:"workerId"`
	Directives []ACPEnvelope `json:"directives"`
}

// PinkyTerminalReport is a worker's final report for a task attempt: the
// verification reference, summary, changed files, observed git head, and
// host-reported duration/token/cost figures used to reconcile evidence in
// ReconcilePinkyEvidence.
type PinkyTerminalReport struct {
	SessionID       string
	WorkerID        string
	Spec            string
	TaskID          string
	Attempt         int
	VerificationRef string
	Summary         string
	ChangedFiles    []string
	GitHead         string
	DurationMs      int64
	HostTokens      int
	HostCost        string
}

// RecordPinkyProgress appends a progress event for report to the session's
// ACP event log, stamping the report time from the host clock (rather than
// trusting the worker) so the driver can weight stall waits (R6).
func RecordPinkyProgress(root string, report PinkyProgressReport, cfg OrchestrationCfg) (ACPEnvelope, error) {
	// Stamp the report time server-side from the host clock so a worker cannot
	// spoof recency; the driver reads this to weight stall waits (R6).
	payload := ACPProgressPayload{
		Percent:    report.Percent,
		Message:    report.Message,
		LastReport: Clock().UTC().Format(time.RFC3339Nano),
	}
	return appendPinkyEvent(root, report.SessionID, report.WorkerID, report.Spec, report.TaskID, report.Attempt, ACPMessageProgress, payload, cfg)
}

// RecordPinkyBlocker appends a blocked event for report to the session's ACP
// event log.
func RecordPinkyBlocker(root string, report PinkyBlockerReport, cfg OrchestrationCfg) (ACPEnvelope, error) {
	payload := ACPBlockerPayload{Reason: report.Reason}
	return appendPinkyEvent(root, report.SessionID, report.WorkerID, report.Spec, report.TaskID, report.Attempt, ACPMessageBlocker, payload, cfg)
}

// RecordPinkyQuery appends a query event for report to the session's ACP
// event log.
func RecordPinkyQuery(root string, report PinkyQueryReport, cfg OrchestrationCfg) (ACPEnvelope, error) {
	payload := ACPQueryPayload{Text: report.Text}
	return appendPinkyEvent(root, report.SessionID, report.WorkerID, report.Spec, report.TaskID, report.Attempt, ACPMessageQuery, payload, cfg)
}

// RecordBrainDirective validates that the targeted worker holds an active
// lease for the directive's task/attempt, then builds and appends a
// directive event addressed to that worker, stamping its ID, timestamps, and
// expiry from the host clock and configured message TTL.
func RecordBrainDirective(root string, directive BrainDirective, cfg OrchestrationCfg) (ACPEnvelope, error) {
	store, err := NewACPStore(root)
	if err != nil {
		return ACPEnvelope{}, err
	}
	if err := store.ValidateActiveLease(directive.SessionID, directive.WorkerID, directive.Spec, directive.TaskID, directive.Attempt); err != nil {
		return ACPEnvelope{}, err
	}
	payload := ACPDirectivePayload{Action: directive.Action, Reason: directive.Reason}
	envelope, err := NewACPEnvelope(ACPMessageDirective, payload)
	if err != nil {
		return ACPEnvelope{}, err
	}
	messageID, err := NewACPID()
	if err != nil {
		return ACPEnvelope{}, err
	}
	now := Clock().UTC()
	envelope.MessageID = messageID
	envelope.SessionID = directive.SessionID
	envelope.CreatedAt = now.Format(time.RFC3339Nano)
	envelope.ExpiresAt = now.Add(time.Duration(cfg.Transport.MessageTTLSeconds) * time.Second).Format(time.RFC3339Nano)
	envelope.From = "brain"
	envelope.To = "pinky-" + directive.WorkerID
	envelope.Spec = directive.Spec
	envelope.Task = directive.TaskID
	envelope.Attempt = directive.Attempt
	envelope.InReplyTo = directive.InReplyTo
	written, err := store.WriteEvent(envelope)
	if err != nil {
		return ACPEnvelope{}, fmt.Errorf("brain directive: append event: %w", err)
	}
	return written, nil
}

// ReadPinkyInbox validates sessionID and workerID, then reads the session's
// ACP event log and returns the directive events addressed to that worker.
func ReadPinkyInbox(root, sessionID, workerID string) (PinkyInbox, error) {
	if err := validateACPOpaqueID("session ID", sessionID); err != nil {
		return PinkyInbox{}, err
	}
	if err := validateACPRuntimeSegment("worker ID", workerID); err != nil {
		return PinkyInbox{}, err
	}
	store, err := NewACPStore(root)
	if err != nil {
		return PinkyInbox{}, err
	}
	events, err := store.readAllEvents(sessionID)
	if err != nil {
		return PinkyInbox{}, err
	}
	to := "pinky-" + workerID
	out := PinkyInbox{SessionID: sessionID, WorkerID: workerID, Directives: []ACPEnvelope{}}
	for _, event := range events {
		if event.Type == ACPMessageDirective && event.To == to {
			out.Directives = append(out.Directives, event)
		}
	}
	return out, nil
}

// RecordPinkyTerminalReport appends an evidence event for report to the
// session's ACP event log, the worker's final word on a task attempt before
// the Brain reconciles it via ReconcilePinkyEvidence.
func RecordPinkyTerminalReport(root string, report PinkyTerminalReport, cfg OrchestrationCfg) (ACPEnvelope, error) {
	payload := ACPEvidencePayload{
		VerificationRef: report.VerificationRef,
		Summary:         report.Summary,
		ChangedFiles:    append([]string{}, report.ChangedFiles...),
		GitHead:         report.GitHead,
		DurationMs:      report.DurationMs,
		HostTokens:      report.HostTokens,
		HostCost:        report.HostCost,
	}
	return appendPinkyEvent(root, report.SessionID, report.WorkerID, report.Spec, report.TaskID, report.Attempt, ACPMessageEvidence, payload, cfg)
}

// AcknowledgePinkyCancellation appends a cancelled event for the given task
// attempt to the session's ACP event log, the worker's acknowledgment that it
// has stopped after a cancel directive.
func AcknowledgePinkyCancellation(root, sessionID, workerID, spec, taskID string, attempt int, reason string, cfg OrchestrationCfg) (ACPEnvelope, error) {
	payload := ACPCancelledPayload{Reason: reason}
	return appendPinkyEvent(root, sessionID, workerID, spec, taskID, attempt, ACPMessageCancelled, payload, cfg)
}

func appendPinkyEvent(root, sessionID, workerID, spec, taskID string, attempt int, messageType ACPMessageType, payload any, cfg OrchestrationCfg) (ACPEnvelope, error) {
	store, err := NewACPStore(root)
	if err != nil {
		return ACPEnvelope{}, err
	}
	if err := store.ValidateActiveLease(sessionID, workerID, spec, taskID, attempt); err != nil {
		return ACPEnvelope{}, err
	}
	if messageType == ACPMessageEvidence {
		events, err := store.readAllEvents(sessionID)
		if err != nil {
			return ACPEnvelope{}, err
		}
		for _, event := range events {
			if event.Type == ACPMessageEvidence && event.From == "pinky-"+workerID && event.Spec == spec && event.Task == taskID && event.Attempt == attempt {
				return event, nil
			}
		}
	}
	envelope, err := NewACPEnvelope(messageType, payload)
	if err != nil {
		return ACPEnvelope{}, err
	}
	messageID, err := NewACPID()
	if err != nil {
		return ACPEnvelope{}, err
	}
	now := Clock().UTC()
	envelope.MessageID = messageID
	envelope.SessionID = sessionID
	envelope.CreatedAt = now.Format(time.RFC3339Nano)
	envelope.ExpiresAt = now.Add(time.Duration(cfg.Transport.MessageTTLSeconds) * time.Second).Format(time.RFC3339Nano)
	envelope.From = "pinky-" + workerID
	envelope.To = "brain"
	envelope.Spec = spec
	envelope.Task = taskID
	envelope.Attempt = attempt
	written, err := store.WriteEvent(envelope)
	if err != nil {
		return ACPEnvelope{}, fmt.Errorf("pinky report: append event: %w", err)
	}
	return written, nil
}

// PinkyEvidenceResult reports the outcome of reconciling a worker's terminal
// report: the immutable ACP evidence event that was recorded and the completion
// result from the existing task-integrity path.
type PinkyEvidenceResult struct {
	Event      ACPEnvelope        `json:"event"`
	Completion CompleteTaskResult `json:"completion"`
}

// ReconcilePinkyEvidence turns an untrusted worker terminal report into a real
// task completion — but only through specd's own integrity paths. It records the
// report as an immutable ACP event (lease-gated, idempotent), then accepts it
// only when it references the matching specd-generated verification record, the
// git head and declared file scope agree, and the role is permitted. Completion
// itself runs through core.CompleteTask, the same path `specd task --status
// complete` uses, so Pinky never becomes a second verification or completion
// mechanism (R4.6, R4.7, R4.8, R4.14). It is idempotent: a duplicate report
// re-records nothing and re-completes nothing.
//
//nolint:gocyclo // pre-existing complexity debt, out of scope for spec S3 — tracked for a future cleanup pass
func ReconcilePinkyEvidence(root string, report PinkyTerminalReport, cfg OrchestrationCfg) (PinkyEvidenceResult, error) {
	// 1. Record the worker's claim. RecordPinkyTerminalReport validates that the
	//    reporter still owns an active lease (V2) and is idempotent, so a forged
	//    or expired worker is rejected here before any state is touched.
	event, err := RecordPinkyTerminalReport(root, report, cfg)
	if err != nil {
		return PinkyEvidenceResult{}, err
	}

	loaded, err := LoadSpec(root, report.Spec)
	if err != nil {
		return PinkyEvidenceResult{}, err
	}
	task, ok := loaded.State.Tasks[report.TaskID]
	if !ok {
		return PinkyEvidenceResult{}, fmt.Errorf("pinky evidence: unknown task %s", report.TaskID)
	}
	docTask := FindTask(loaded.Doc, report.TaskID)
	if docTask == nil {
		return PinkyEvidenceResult{}, fmt.Errorf("pinky evidence: unknown task %s", report.TaskID)
	}

	// 2. Read-only roles have no edit authority and no runnable verify command;
	//    they cannot submit verified completion evidence (R4.6, V5).
	if IsReadonlyRole(task.Role) {
		return PinkyEvidenceResult{}, GateError(fmt.Sprintf("pinky evidence: task %s role %q is read-only — it cannot submit verified completion evidence", report.TaskID, task.Role))
	}

	// 3. The verification record must be specd-generated and passing. A missing or
	//    failed record fails closed.
	rec := task.Verification
	if rec == nil || !rec.Verified {
		return PinkyEvidenceResult{}, GateError(fmt.Sprintf("pinky evidence: task %s has no passing specd verification record — run `specd verify %s %s`", report.TaskID, report.Spec, report.TaskID))
	}

	// 4. The verify command on record must still match the task's `verify:` line;
	//    a changed command means the record is stale.
	if rec.Command != docTask.Meta["verify"] {
		return PinkyEvidenceResult{}, GateError(fmt.Sprintf("pinky evidence: task %s verification is stale — recorded command (%s) ≠ current verify line (%s)", report.TaskID, rec.Command, docTask.Meta["verify"]))
	}

	// 5. The report must reference the exact specd record. A forged ACP evidence
	//    payload carries a ref that will not match.
	if report.VerificationRef != VerificationRef(rec) {
		return PinkyEvidenceResult{}, GateError(fmt.Sprintf("pinky evidence: task %s verificationRef does not match the specd record — refusing forged evidence", report.TaskID))
	}

	// 6. The git head the worker reports must equal the record's head; otherwise
	//    the verification ran against a different tree (stale head).
	if report.GitHead != verificationGitHead(rec) {
		return PinkyEvidenceResult{}, GateError(fmt.Sprintf("pinky evidence: task %s git head %q does not match the verified head %q", report.TaskID, report.GitHead, verificationGitHead(rec)))
	}

	// 7. The worker's claimed changed files must equal the record's changed files;
	//    a divergent claim is rejected.
	if !sameStringSet(report.ChangedFiles, rec.ChangedFiles) {
		return PinkyEvidenceResult{}, GateError(fmt.Sprintf("pinky evidence: task %s reported changed files do not match the verification record", report.TaskID))
	}

	// 8. Scope gate: when configured as `error`, any recorded change outside the
	//    declared `files:` contract fails closed (R4.14).
	if err := enforcePinkyScope(root, docTask, rec); err != nil {
		return PinkyEvidenceResult{}, err
	}

	// 9. Complete through the single integrity path (dependencies, gate, verified
	//    record). Idempotent: a duplicate report finds the task already complete.
	req := CompleteTaskRequest{Idempotent: true}
	if report.HostTokens > 0 {
		tokens := report.HostTokens
		req.Tokens = &tokens
	}
	if strings.TrimSpace(report.HostCost) != "" {
		cost := report.HostCost
		req.Cost = &cost
	}
	completion, err := CompleteTask(root, report.Spec, report.TaskID, req)
	if err != nil {
		return PinkyEvidenceResult{}, err
	}
	// A verified-complete task must never be resurrected by a leftover checkpoint
	// (Req 6). Delete every checkpoint for this task now that it is done. Gated on
	// the resilience flag so the disabled path touches no new directories.
	if cfg.Resilience != nil && cfg.Resilience.CheckpointEnabled {
		if err := CleanupCheckpoint(root, report.SessionID, report.TaskID); err != nil {
			return PinkyEvidenceResult{}, err
		}
	}
	// Record host-reported token actuals into the context ledger (R7). It is a
	// no-op when the spec is not running under an orchestration session.
	if report.HostTokens > 0 {
		if err := recordSessionLedgerEntry(root, report.SessionID, ContextLedgerEntry{
			StepSequence:       uint64(loaded.State.Revision),
			Phase:              loaded.State.Phase,
			Action:             "step",
			HostReportedTokens: report.HostTokens,
			Reason:             "host-actuals",
		}); err != nil {
			return PinkyEvidenceResult{}, err
		}
	}
	return PinkyEvidenceResult{Event: event, Completion: completion}, nil
}

// VerificationRef is the canonical reference a worker must echo back to claim a
// specd verification record. It binds the command, git head, and run timestamp,
// so any change to the verified work changes the ref.
func VerificationRef(rec *VerificationRecord) string {
	if rec == nil {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{
		rec.Command,
		verificationGitHead(rec),
		rec.RanAt,
	}, "\x1f")))
	return hex.EncodeToString(sum[:16])
}

func verificationGitHead(rec *VerificationRecord) string {
	if rec == nil || rec.GitHead == nil {
		return ""
	}
	return *rec.GitHead
}

// enforcePinkyScope reuses the existing scope-gate semantics: when scope is
// `error`, a recorded changed file outside the task's declared glob contract is a
// hard failure. Other modes (off/warn/`*`) and `*` contracts opt out.
func enforcePinkyScope(root string, docTask *ParsedTask, rec *VerificationRecord) error {
	mode := LoadConfig(root).Gates.Scope
	if mode != "error" {
		return nil
	}
	patterns := parseFilesContract(docTask.Meta["files"])
	if len(patterns) == 0 || containsStr(patterns, "*") {
		return nil
	}
	for _, f := range rec.ChangedFiles {
		if !matchesAnyGlob(f, patterns) {
			return GateError(fmt.Sprintf("pinky evidence: task %s changed '%s' outside its declared files contract (%s)", docTask.ID, f, strings.Join(patterns, ", ")))
		}
	}
	return nil
}

func sameStringSet(a, b []string) bool {
	left := append([]string{}, a...)
	right := append([]string{}, b...)
	sort.Strings(left)
	sort.Strings(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
