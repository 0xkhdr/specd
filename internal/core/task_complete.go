package core

import "fmt"

func CompleteTask(rawTasks []byte, taskID string, records map[string]EvidenceRecord) ([]byte, error) {
	if !HasPassingEvidence(records, taskID) {
		return nil, fmt.Errorf("task %s requires passing evidence", taskID)
	}
	return RewriteTaskStatusLine(rawTasks, taskID, "✅")
}
