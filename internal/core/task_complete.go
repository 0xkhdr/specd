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
	// records is attempt-filtered by LoadEvidence, so a reopened task reaches
	// here with its prior attempt's records already dropped: the same refusal
	// covers "never verified" and "verified under a superseded attempt", and
	// the recovery is identical in both cases (spec 04 R3.2).
	record, ok := records[taskID]
	if !ok || record.ExitCode != 0 {
		return fmt.Errorf("task %s requires passing evidence recorded for its current attempt; run `specd verify` again", taskID)
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
		return nil, fmt.Errorf("%s", missingEvidenceMessage(taskID, st.Missing))
	}
	if len(st.Stale) > 0 {
		return nil, fmt.Errorf("EVIDENCE_STALE: task %s evidence not current for %s", taskID, FormatRequirements(st.Stale))
	}
	return RewriteTaskStatusLine(rawTasks, taskID, "✅")
}

// missingEvidenceMessage makes an EVIDENCE_MISSING refusal self-unblocking
// (spec R2.3): it names each declared class/check-id, states that a plain
// verify record carries no evidence class (so it can never satisfy a non-test
// declaration), and prints the exact `specd eval import` command — or the
// option to remove the declaration — per outstanding requirement. Test-class
// gaps name re-verify instead: a passing `specd verify` stamps them (R2.1).
func missingEvidenceMessage(taskID string, missing []EvidenceRequirement) string {
	var b strings.Builder
	fmt.Fprintf(&b, "EVIDENCE_MISSING: task %s lacks passing evidence for %s", taskID, FormatRequirements(missing))
	for _, r := range missing {
		if r.EvidenceClass == EvidenceTest {
			fmt.Fprintf(&b, "; %s/%s: re-run `specd verify` — a passing run stamps this test-class envelope", r.EvidenceClass, r.CheckID)
			continue
		}
		fmt.Fprintf(&b, "; %s/%s: a plain `specd verify` record carries no evidence class and cannot satisfy it — import external evidence with `specd eval import <slug> <file> --task %s --check %s`, or remove the declaration from the task's evidence cell", r.EvidenceClass, r.CheckID, taskID, r.CheckID)
	}
	return b.String()
}

// FormatRequirements renders class/check-id requirements for refusals and the
// verify-time outstanding-contract notice, so both surfaces name a contract
// identically.
func FormatRequirements(reqs []EvidenceRequirement) string {
	parts := make([]string, len(reqs))
	for i, r := range reqs {
		parts[i] = string(r.EvidenceClass) + "/" + r.CheckID
	}
	return strings.Join(parts, ", ")
}
