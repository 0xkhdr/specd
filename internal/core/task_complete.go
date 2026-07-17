package core

import (
	"fmt"
	"strings"
)

// UnknownHead is the sentinel gitHead writes when HEAD cannot be resolved
// (commitless repo, no git). Evidence carrying it is not pinned to a commit.
const UnknownHead = "unknown"

// HeadPinned reports whether an evidence record's git_head names a real commit.
// Empty (pre-W3 records) and the "unknown" sentinel both fail: an evidence
// record that cannot be pinned to a commit does not count toward completion.
func HeadPinned(gitHead string) bool {
	return gitHead != "" && gitHead != UnknownHead
}

// verifyEvidenceReady refuses completion unless the task's deterministic verify
// record passed and is pinned to a real commit. This is the no-bypass test gate
// (spec 04 R3.4): a failing deterministic test always blocks completion.
func verifyEvidenceReady(taskID string, records map[string]EvidenceRecord) error {
	record, ok := records[taskID]
	if !ok || record.ExitCode != 0 {
		return fmt.Errorf("task %s requires passing evidence", taskID)
	}
	if !HeadPinned(record.GitHead) {
		return fmt.Errorf("task %s evidence is not pinned to a commit (git_head %q); re-run `specd verify %s` in a repo with a resolvable HEAD", taskID, record.GitHead, taskID)
	}
	return nil
}

// CompleteTask advances a task to done. Completion authority is verify/eval
// evidence alone: this function never reads the run ledger (runs.jsonl), so the
// evidence gate passes or fails identically whether that additive ledger is
// present or absent (spec 07 R2.3).
func CompleteTask(rawTasks []byte, taskID string, records map[string]EvidenceRecord) ([]byte, error) {
	if err := verifyEvidenceReady(taskID, records); err != nil {
		return nil, err
	}
	return RewriteTaskStatusLine(rawTasks, taskID, "✅")
}

// CompleteTaskWithQuality is CompleteTask plus the quality contract: after the
// no-bypass verify gate, it requires every declared evidence class/check to
// have a fresh passing record for the current subject (spec 04 R3.3, R3.4). A
// missing required record refuses with EVIDENCE_MISSING; a passing-but-stale
// one with EVIDENCE_STALE. A contract with no requirements degrades to
// CompleteTask, so tasks without them stay unaffected.
func CompleteTaskWithQuality(rawTasks []byte, taskID string, records map[string]EvidenceRecord, c QualityContract, evals []EvidenceEnvelopeV1, subject FreshnessSubject) ([]byte, error) {
	if err := verifyEvidenceReady(taskID, records); err != nil {
		return nil, err
	}
	if c.TaskID != "" && c.TaskID != taskID {
		return nil, fmt.Errorf("QUALITY_TASK_MISMATCH: contract task %s cannot complete %s", c.TaskID, taskID)
	}
	st := EvaluateQuality(c, evals, subject)
	if len(st.Missing) > 0 {
		return nil, fmt.Errorf("EVIDENCE_MISSING: task %s lacks passing evidence for %s", taskID, formatRequirements(st.Missing))
	}
	if len(st.Stale) > 0 {
		return nil, fmt.Errorf("EVIDENCE_STALE: task %s evidence not current for %s", taskID, formatRequirements(st.Stale))
	}
	return RewriteTaskStatusLine(rawTasks, taskID, "✅")
}

func formatRequirements(reqs []EvidenceRequirement) string {
	parts := make([]string, len(reqs))
	for i, r := range reqs {
		parts[i] = string(r.EvidenceClass) + "/" + r.CheckID
	}
	return strings.Join(parts, ", ")
}
