package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// CriterionRecord attests a single acceptance criterion of an approved
// requirement. It is a distinct evidence type from a task verify record
// (EvidenceRecord): a criterion record carries operator-supplied evidence and
// is never produced by running a command, so it can never substitute for a
// task's passing verify record (spec 04 R7). Records are append-only — a later
// pass never erases a prior fail (R4).
type CriterionRecord struct {
	Type      string `json:"type"`      // always "criterion" — discriminates the store
	Criterion string `json:"criterion"` // "<req>.<sub>", e.g. "1.2"
	Status    string `json:"status"`    // "pass" | "fail"
	Evidence  string `json:"evidence"`  // operator-supplied text or path
	GitHead   string `json:"git_head"`  // pinned commit, same discipline as verify (R3)
	Timestamp string `json:"timestamp"` // RFC3339, from the injectable Clock
	Actor     string `json:"actor"`
}

// CriterionStatusPass / Fail are the only valid statuses; the verify command's
// declared flag enum rejects anything else before this layer is reached.
const (
	CriterionStatusPass = "pass"
	CriterionStatusFail = "fail"
)

// CriteriaPath is the per-spec append-only criterion evidence ledger. It is kept
// separate from evidence.jsonl so the task-verify loader (last-write-wins per
// task) is untouched and the two evidence types stay physically distinct.
func CriteriaPath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "criteria.jsonl")
}

// AppendCriterion stamps and appends a criterion record. It fills the timestamp
// (Clock) and actor here so callers only supply the domain fields; the caller is
// responsible for holding the per-spec lock (see WithSpecLock) and for resolving
// gitHead.
func AppendCriterion(path string, rec CriterionRecord) error {
	if rec.Criterion == "" {
		return errors.New("criterion id required")
	}
	if rec.Status != CriterionStatusPass && rec.Status != CriterionStatusFail {
		return errors.New("criterion status must be pass or fail")
	}
	if rec.Evidence == "" {
		return errors.New("criterion evidence required")
	}
	rec.Type = "criterion"
	rec.Timestamp = Clock().Format(time.RFC3339)
	rec.Actor = recordActor()
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return AppendFile(path, string(data)+"\n")
}

// LoadCriteria reads the criterion ledger in append order, preserving history
// (fails retained after later passes). A missing file is an empty ledger.
func LoadCriteria(path string) ([]CriterionRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var records []CriterionRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var rec CriterionRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, scanner.Err()
}

// CurrentPassing returns the set of criterion ids whose latest record is a pass
// recorded strictly after `since` (the last requirements approval). Re-approving
// requirements moves `since` forward and invalidates stale attestations by
// construction — no mutation of records needed (spec 04 R5/R6, "current").
//
// A zero `since` means requirements were never approved; nothing counts.
func CurrentPassing(records []CriterionRecord, since time.Time) map[string]bool {
	// latest[id] = most recent record for that criterion after `since`.
	type stamped struct {
		status string
		at     time.Time
	}
	latest := map[string]stamped{}
	for _, rec := range records {
		at, err := time.Parse(time.RFC3339, rec.Timestamp)
		// Count records at-or-after the last requirements approval. Timestamps
		// are RFC3339 second-granularity, so a criterion recorded in the same
		// second as the approval is treated as current (>= not strictly >);
		// re-approving in a later second still invalidates earlier records.
		if err != nil || at.Before(since) {
			continue
		}
		if prev, ok := latest[rec.Criterion]; ok && at.Before(prev.at) {
			continue
		}
		latest[rec.Criterion] = stamped{status: rec.Status, at: at}
	}
	passing := map[string]bool{}
	for id, s := range latest {
		if s.status == CriterionStatusPass {
			passing[id] = true
		}
	}
	return passing
}
