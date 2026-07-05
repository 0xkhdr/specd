package core

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SubmissionRecord is one terminal submission of a spec: the deterministic PR
// summary (identified by its content hash) that was streamed to the
// operator-configured command, pinned to the git HEAD it described and the
// command's exit code. Records are append-only — a resubmission never erases a
// prior one, so the ledger is a full audit trail (spec 08 R4, feeds spec 13).
type SubmissionRecord struct {
	Type        string `json:"type"`         // always "submission" — discriminates the store
	GitHead     string `json:"git_head"`     // commit the summary described
	SummaryHash string `json:"summary_hash"` // sha256 of the streamed summary
	Command     string `json:"command"`      // configured submit.command ("" = dry-run)
	Exit        int    `json:"exit"`         // command exit code (0 for dry-run)
	Timestamp   string `json:"timestamp"`    // RFC3339, from the injectable Clock
	Actor       string `json:"actor"`
}

const submissionType = "submission"

// SummaryHash is the content address of a submission summary: a hex sha256. The
// ledger stores it so `report --history` (spec 13) can show exactly what was
// submitted without re-deriving or storing the full summary text.
func SummaryHash(summary string) string {
	sum := sha256.Sum256([]byte(summary))
	return hex.EncodeToString(sum[:])
}

// SubmissionsPath is the per-spec append-only submission ledger. It is kept
// separate from evidence/criteria so the terminal-submit audit trail is
// physically distinct from execution evidence.
func SubmissionsPath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "submissions.jsonl")
}

// AppendSubmission stamps and appends a submission record. It fills the
// timestamp (Clock) and actor here; the caller resolves gitHead and holds the
// per-spec lock (see WithSpecLock).
func AppendSubmission(path string, rec SubmissionRecord) error {
	if rec.SummaryHash == "" {
		return errors.New("submission summary hash required")
	}
	rec.Type = submissionType
	rec.Timestamp = Clock().Format(time.RFC3339)
	rec.Actor = recordActor()
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return AppendFile(path, string(data)+"\n")
}

// LoadSubmissions reads the submission ledger in append order. A missing file
// is an empty ledger.
func LoadSubmissions(path string) ([]SubmissionRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var records []SubmissionRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var rec SubmissionRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, scanner.Err()
}

// AlreadySubmittedAt reports whether the ledger holds a successful submission
// (exit 0) pinned to head. It is the idempotence guard behind the --resubmit
// requirement (spec 08 R5): a double-fire from orchestration at the same HEAD is
// refused unless the operator explicitly opts back in.
func AlreadySubmittedAt(records []SubmissionRecord, head string) bool {
	if !HeadPinned(head) {
		return false
	}
	for _, rec := range records {
		if rec.Exit == 0 && rec.GitHead == head {
			return true
		}
	}
	return false
}

// SubmitBlockers enumerates why a spec is not submittable: every gate failure
// (rendered upstream, since the gate registry lives in a subpackage) plus every
// task that is not complete. An empty result means all gates are green and every
// task is done — the R1 precondition. It is a pure function of the model and the
// pre-rendered gate failures so it is unit-testable without running gates.
func SubmitBlockers(model ReportModel, gateFailures []string) []string {
	blockers := make([]string, 0, len(gateFailures)+len(model.Tasks))
	blockers = append(blockers, gateFailures...)
	for _, task := range model.Tasks {
		if task.Status != TaskComplete {
			blockers = append(blockers, fmt.Sprintf("task %s not complete (%s)", task.ID, task.Status))
		}
	}
	return blockers
}
