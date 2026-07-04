package core

import "fmt"

// UnknownHead is the sentinel gitHead writes when HEAD cannot be resolved
// (commitless repo, no git). Evidence carrying it is not pinned to a commit.
const UnknownHead = "unknown"

// HeadPinned reports whether an evidence record's git_head names a real commit.
// Empty (pre-W3 records) and the "unknown" sentinel both fail: an evidence
// record that cannot be pinned to a commit does not count toward completion.
func HeadPinned(gitHead string) bool {
	return gitHead != "" && gitHead != UnknownHead
}

func CompleteTask(rawTasks []byte, taskID string, records map[string]EvidenceRecord) ([]byte, error) {
	record, ok := records[taskID]
	if !ok || record.ExitCode != 0 {
		return nil, fmt.Errorf("task %s requires passing evidence", taskID)
	}
	if !HeadPinned(record.GitHead) {
		return nil, fmt.Errorf("task %s evidence is not pinned to a commit (git_head %q); re-run `specd verify %s` in a repo with a resolvable HEAD", taskID, record.GitHead, taskID)
	}
	return RewriteTaskStatusLine(rawTasks, taskID, "✅")
}
