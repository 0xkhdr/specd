package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type EvidenceRecord struct {
	TaskID      string `json:"task_id"`
	Command     string `json:"command"`
	ExitCode    int    `json:"exit_code"`
	GitHead     string `json:"git_head"`
	EvidenceRef string `json:"evidence_ref,omitempty"`
}

func EvidencePath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "evidence.jsonl")
}

func AppendEvidence(path string, record EvidenceRecord) error {
	if record.TaskID == "" {
		return errors.New("evidence task id is required")
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return AppendFile(path, string(data)+"\n")
}

func LoadEvidence(path string) (map[string]EvidenceRecord, error) {
	records := map[string]EvidenceRecord{}
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
		if record.TaskID != "" {
			records[record.TaskID] = record
		}
	}
	return records, scanner.Err()
}

func HasPassingEvidence(records map[string]EvidenceRecord, taskID string) bool {
	record, ok := records[taskID]
	return ok && record.ExitCode == 0 && record.GitHead != ""
}
