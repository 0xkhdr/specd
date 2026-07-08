package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// OverrideRecord is an append-only human clearance of an escalated task (spec 06
// R3). It resets the verify-failure counter but does NOT complete the task or
// stand in for evidence: after an override the task still needs a passing verify
// record to complete (the no-bypass invariant). PriorFailCount pins how many
// consecutive fails were cleared, for the audit trail.
type OverrideRecord struct {
	TaskID         string `json:"task_id"`
	Reason         string `json:"reason"`
	Actor          string `json:"actor"`
	Timestamp      string `json:"timestamp"`
	PriorFailCount int    `json:"prior_fail_count"`
}

// OverridePath is the per-spec append-only override ledger.
func OverridePath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "overrides.jsonl")
}

// AppendOverride appends one override record. An empty reason is rejected: a
// clearance with no stated reason is not a reasoned override (R3).
func AppendOverride(path string, record OverrideRecord) error {
	if record.TaskID == "" {
		return errors.New("override task id is required")
	}
	if strings.TrimSpace(record.Reason) == "" {
		return errors.New("override reason is required")
	}
	if record.Timestamp == "" {
		record.Timestamp = Clock().Format(time.RFC3339)
	}
	if record.Actor == "" {
		record.Actor = recordActor()
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return AppendFile(path, string(data)+"\n")
}

// LoadOverrides reads the override ledger in append order. A missing file is not
// an error (no overrides yet).
func LoadOverrides(path string) ([]OverrideRecord, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []OverrideRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}
		var record OverrideRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, scanner.Err()
}

// ConsecutiveVerifyFails counts the trailing run of failing verify attempts for
// taskID over a merged timeline of evidence and overrides. A passing verify or
// an override resets the count to zero. Pure over its inputs; deterministic for
// identical logs (R6).
func ConsecutiveVerifyFails(evidence []EvidenceRecord, overrides []OverrideRecord, taskID string) int {
	type event struct {
		ts    string
		sub   int // resets sort after same-timestamp verifies
		reset bool
		pass  bool
	}
	var events []event
	for _, e := range evidence {
		if e.TaskID != taskID {
			continue
		}
		events = append(events, event{ts: e.Timestamp, sub: 0, pass: e.ExitCode == 0})
	}
	for _, o := range overrides {
		if o.TaskID != taskID {
			continue
		}
		events = append(events, event{ts: o.Timestamp, sub: 1, reset: true})
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].ts != events[j].ts {
			return events[i].ts < events[j].ts
		}
		return events[i].sub < events[j].sub
	})
	count := 0
	for _, e := range events {
		switch {
		case e.reset, e.pass:
			count = 0
		default:
			count++
		}
	}
	return count
}

// EscalatedCounts returns, for each task that is currently escalated, its
// consecutive verify-fail count. Tasks below the threshold (or all tasks when
// the ratchet is disabled) are absent from the map. Pure over its inputs.
func EscalatedCounts(evidence []EvidenceRecord, overrides []OverrideRecord, tasks []TaskRow, maxFails int) map[string]int {
	out := map[string]int{}
	for _, task := range tasks {
		count := ConsecutiveVerifyFails(evidence, overrides, task.ID)
		if IsEscalated(count, maxFails) {
			out[task.ID] = count
		}
	}
	return out
}

// IsEscalated reports whether a task with the given consecutive-fail count is
// escalated under the ratchet threshold. maxFails <= 0 disables the ratchet: no
// task is ever escalated (the escape hatch, R5).
func IsEscalated(consecutiveFails, maxFails int) bool {
	return maxFails > 0 && consecutiveFails >= maxFails
}
