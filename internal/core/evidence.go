package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	verifyexec "github.com/0xkhdr/specd/internal/core/verify"
)

// validateEvidenceRef enforces the evidence_ref locator contract (spec 07 R5.3):
// a reference must be workspace-relative or content-addressed — never a URL, an
// absolute path, or a parent-directory traversal. An empty ref is valid (the
// field is optional). Core never dereferences a ref, but must refuse to store
// one that points outside the workspace or off to the network.
func validateEvidenceRef(ref string) error {
	if ref == "" {
		return nil
	}
	if strings.Contains(ref, "://") {
		return fmt.Errorf("evidence_ref %q must be workspace-relative or content-addressed, not a URL", ref)
	}
	if filepath.IsAbs(ref) || strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, `\`) || strings.HasPrefix(ref, "~") {
		return fmt.Errorf("evidence_ref %q must be workspace-relative, not absolute", ref)
	}
	for _, seg := range strings.Split(filepath.ToSlash(ref), "/") {
		if seg == ".." {
			return fmt.Errorf("evidence_ref %q must not traverse outside the workspace", ref)
		}
	}
	return nil
}

const EvidenceOutputLimit = 64 * 1024

type EvidenceRecord struct {
	TaskID      string `json:"task_id"`
	Command     string `json:"command"`
	ExitCode    int    `json:"exit_code"`
	GitHead     string `json:"git_head"`
	EvidenceRef string `json:"evidence_ref,omitempty"`
	// ContextReceiptDigest pins context identity used for this attempt. It is
	// optional and never substitutes for exit-code
	// evidence or a resolvable Git HEAD.
	ContextReceiptDigest string `json:"context_receipt_digest,omitempty"`
	// Timestamp and Actor stamp the attempt so `report --history` (spec 13) can
	// replay verify attempts in time order alongside approvals and submissions.
	// Both are omitempty: records written before spec 13 carry neither and still
	// decode as fully valid evidence — they simply sort by append order.
	Timestamp string `json:"timestamp,omitempty"`
	Actor     string `json:"actor,omitempty"`
	// Attempt, PlanRevision, and ScopeRevision bind the record to one attempt of
	// the task (spec 04 R3.2). All three are omitempty and their zero values are
	// exactly attempt 1 at plan/scope revision 0, so every record written before
	// attempt binding still completes a task that was never reopened. Once a
	// task is reopened, a record carrying the prior attempt is refused even when
	// command, files, and git HEAD are identical.
	Attempt       int   `json:"attempt,omitempty"`
	PlanRevision  int64 `json:"plan_revision,omitempty"`
	ScopeRevision int64 `json:"scope_revision,omitempty"`
	// Telemetry is optional worker-reported cost, stored verbatim (spec 10). A
	// nil pointer means the worker reported none — never imputed as zero. Old
	// records predating telemetry decode to nil, so they stay fully valid (R5).
	Telemetry *Annotations `json:"telemetry,omitempty"`
}

func EvidencePath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "evidence.jsonl")
}

// attemptFor resolves the current attempt of a task from the workflow ledger
// that sits beside the evidence log. Stamping on write and filtering on read
// both funnel through here, so every caller of AppendEvidence/LoadEvidence —
// verify, complete-task, status, report — is attempt-bound without threading an
// attempt through its call sites (spec 04 R3.2).
//
// ponytail: the ledger is re-read per call rather than cached. Spec ledgers are
// small and both callers already do file IO; add a cache only if it measures.
func attemptFor(evidencePath, taskID string) (TaskAttempt, error) {
	attempts, err := reopenedAttempts(evidencePath)
	if err != nil {
		return TaskAttempt{}, err
	}
	if attempt, ok := attempts[taskID]; ok {
		return attempt, nil
	}
	return TaskAttempt{TaskID: taskID, Attempt: 1}, nil
}

// EvidenceAttempt is the attempt number a record is bound to; records written
// before attempt binding carry 0, which means the first attempt.
func EvidenceAttempt(record EvidenceRecord) int {
	if record.Attempt == 0 {
		return 1
	}
	return record.Attempt
}

// EvidenceBoundTo reports whether a record was recorded under this attempt and
// its current plan and scope revision.
func EvidenceBoundTo(record EvidenceRecord, attempt TaskAttempt) bool {
	return EvidenceAttempt(record) == attempt.Attempt &&
		record.PlanRevision == attempt.PlanRevision &&
		record.ScopeRevision == attempt.ScopeRevision
}

func AppendEvidence(path string, record EvidenceRecord) error {
	if record.TaskID == "" {
		return errors.New("evidence task id is required")
	}
	if err := ValidateAnnotations(record.Telemetry); err != nil {
		return err
	}
	if err := validateEvidenceRef(record.EvidenceRef); err != nil {
		return err
	}
	if err := validateContextReceiptDigest(record.ContextReceiptDigest); err != nil {
		return err
	}
	// Stamp the current attempt centrally, for the same reason provenance is
	// stamped here: a writer must not be able to record evidence for an attempt
	// other than the task's current one. A caller that already stamped (tests,
	// replay fixtures) is left untouched.
	if record.Attempt == 0 {
		attempt, err := attemptFor(path, record.TaskID)
		if err != nil {
			return err
		}
		// Attempt 1 stays implicit so a never-reopened task's record is written
		// exactly as it was before attempt binding existed.
		if attempt.Attempt > 1 {
			record.Attempt = attempt.Attempt
			record.PlanRevision = attempt.PlanRevision
			record.ScopeRevision = attempt.ScopeRevision
		}
	}
	// Stamp provenance centrally so every writer (verify and complete-task) gets
	// an ordering-safe timestamp/actor without threading it through call sites.
	// A caller that already stamped (tests, replay fixtures) is left untouched.
	if record.Timestamp == "" {
		record.Timestamp = Clock().Format(time.RFC3339)
	}
	if record.Actor == "" {
		record.Actor = recordActor()
	}
	redactor := verifyexec.NewRedactor(nil)
	record.Command = redactor.String(record.Command)
	record.EvidenceRef = redactor.String(record.EvidenceRef)
	// attestation_ref is telemetry's one free-form field; run it through the same
	// central redactor so a secret or absolute home path never reaches the ledger
	// (spec 07 R5.2/R5.4). Copy before mutating so the caller's record is unchanged.
	if record.Telemetry != nil && record.Telemetry.AttestationRef != "" {
		tel := *record.Telemetry
		tel.AttestationRef = redactor.String(tel.AttestationRef)
		record.Telemetry = &tel
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return AppendFile(path, string(data)+"\n")
}

// LoadEvidence returns the latest attempt-current record per task. Records left
// behind by a superseded attempt are dropped here rather than at each call site,
// so verify, completion, status, and reporting all see the same truth: only
// evidence bound to a task's current attempt counts (spec 04 R3.2).
// LoadEvidenceRecords keeps the full history, superseded attempts included.
func LoadEvidence(path string) (map[string]EvidenceRecord, error) {
	records := map[string]EvidenceRecord{}
	attempts, err := reopenedAttempts(path)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return records, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}
		var record EvidenceRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, err
		}
		if err := ValidateAnnotations(record.Telemetry); err != nil {
			return nil, fmt.Errorf("evidence %s: %w", record.TaskID, err)
		}
		if err := validateEvidenceRef(record.EvidenceRef); err != nil {
			return nil, fmt.Errorf("evidence %s: %w", record.TaskID, err)
		}
		if err := validateContextReceiptDigest(record.ContextReceiptDigest); err != nil {
			return nil, fmt.Errorf("evidence %s: %w", record.TaskID, err)
		}
		if record.TaskID == "" {
			continue
		}
		if attempt, reopened := attempts[record.TaskID]; reopened && !EvidenceBoundTo(record, attempt) {
			continue
		}
		records[record.TaskID] = record
	}
	return records, scanner.Err()
}

// reopenedAttempts is the current attempt of every reopened task in the spec
// this evidence log belongs to. Empty for a spec that has never reopened a
// task, which is why the read path is a no-op until a reopen happens.
func reopenedAttempts(evidencePath string) (map[string]TaskAttempt, error) {
	events, err := ReadWorkflowEvents(filepath.Join(filepath.Dir(evidencePath), "workflow-events.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("resolve task attempts for evidence: %w", err)
	}
	return TaskAttempts(events), nil
}

// LoadEvidenceRecords reads the evidence log in append order, preserving every
// attempt (unlike LoadEvidence, which keeps only the latest record per task).
// Telemetry aggregation needs the full history for per-attempt breakdown.
func LoadEvidenceRecords(path string) ([]EvidenceRecord, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []EvidenceRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}
		var record EvidenceRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, err
		}
		if err := ValidateAnnotations(record.Telemetry); err != nil {
			return nil, fmt.Errorf("evidence %s: %w", record.TaskID, err)
		}
		if err := validateEvidenceRef(record.EvidenceRef); err != nil {
			return nil, fmt.Errorf("evidence %s: %w", record.TaskID, err)
		}
		if err := validateContextReceiptDigest(record.ContextReceiptDigest); err != nil {
			return nil, fmt.Errorf("evidence %s: %w", record.TaskID, err)
		}
		records = append(records, record)
	}
	return records, scanner.Err()
}

func validateContextReceiptDigest(digest string) error {
	if digest == "" {
		return nil
	}
	if len(digest) != 64 || strings.ToLower(digest) != digest {
		return errors.New("context_receipt_digest must be a lowercase SHA-256 digest")
	}
	for _, r := range digest {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return errors.New("context_receipt_digest must be a lowercase SHA-256 digest")
		}
	}
	return nil
}

func HasPassingEvidence(records map[string]EvidenceRecord, taskID string) bool {
	record, ok := records[taskID]
	return ok && record.ExitCode == 0 && HeadPinned(record.GitHead)
}

func TruncateEvidenceOutput(output string) string {
	if len(output) <= EvidenceOutputLimit {
		return output
	}
	return output[:EvidenceOutputLimit] + fmt.Sprintf("\n[specd: output truncated to %d of %d bytes]\n", EvidenceOutputLimit, len(output))
}
