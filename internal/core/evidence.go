package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	verifyexec "github.com/0xkhdr/specd/internal/core/verify"
)

const EvidenceOutputLimit = 64 * 1024

type EvidenceRecord struct {
	TaskID      string `json:"task_id"`
	Command     string `json:"command"`
	ExitCode    int    `json:"exit_code"`
	GitHead     string `json:"git_head"`
	EvidenceRef string `json:"evidence_ref,omitempty"`
	// Timestamp and Actor stamp the attempt so `report --history` (spec 13) can
	// replay verify attempts in time order alongside approvals and submissions.
	// Both are omitempty: records written before spec 13 carry neither and still
	// decode as fully valid evidence — they simply sort by append order.
	Timestamp string `json:"timestamp,omitempty"`
	Actor     string `json:"actor,omitempty"`
	// Telemetry is optional worker-reported cost, stored verbatim (spec 10). A
	// nil pointer means the worker reported none — never imputed as zero. Old
	// records predating telemetry decode to nil, so they stay fully valid (R5).
	Telemetry *Annotations `json:"telemetry,omitempty"`
}

func EvidencePath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "evidence.jsonl")
}

func AppendEvidence(path string, record EvidenceRecord) error {
	if record.TaskID == "" {
		return errors.New("evidence task id is required")
	}
	// Stamp provenance centrally so every writer (verify and task complete) gets
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
		records = append(records, record)
	}
	return records, scanner.Err()
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
