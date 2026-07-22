package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ClarificationTransition is the closed set of clarification states. A record is
// never edited: open is appended once per id, and exactly one of the three
// resolutions may follow it (spec 03 R4.2). A changed question is a new id.
type ClarificationTransition string

const (
	ClarificationOpen      ClarificationTransition = "open"
	ClarificationAnswered  ClarificationTransition = "answered"
	ClarificationWithdrawn ClarificationTransition = "withdrawn"
	ClarificationExpired   ClarificationTransition = "expired"
)

// Clarification entity kinds. Only a task-scoped clarification can affect task
// readiness; spec- and artifact-scoped ones are recorded and reported but never
// widen the blast radius to unrelated tasks (R4.1).
const (
	ClarificationEntitySpec     = "spec"
	ClarificationEntityTask     = "task"
	ClarificationEntityArtifact = "artifact"
)

// Wait reason codes owned by clarifications.
const (
	WaitClarificationOpen  = "clarification_open"
	WaitClarificationStale = "clarification_stale"
)

const clarificationRecordPrefix = "clarification:"

// ClarificationRecord is one immutable transition. Entity/EntityVersion pin what
// the question is about and the exact version it was asked (or answered)
// against, so a later artifact revision can be detected as staleness rather than
// silently inheriting an answer (R4.1, R4.3).
type ClarificationRecord struct {
	Kind          string                  `json:"kind"`
	ID            string                  `json:"id"`
	Transition    ClarificationTransition `json:"transition"`
	EntityKind    string                  `json:"entity_kind"`
	EntityID      string                  `json:"entity_id"`
	EntityVersion string                  `json:"entity_version,omitempty"`
	Blocking      bool                    `json:"blocking,omitempty"`
	Question      string                  `json:"question,omitempty"`
	Answer        string                  `json:"answer,omitempty"`
	Reason        string                  `json:"reason,omitempty"`
	Timestamp     string                  `json:"timestamp"`
	GitHead       string                  `json:"git_head"`
	Actor         string                  `json:"actor"`
}

// StampClarification fills the provenance triple, mirroring StampRecord.
func StampClarification(rec ClarificationRecord, gitHead string) ClarificationRecord {
	rec.Kind = "clarification"
	rec.Timestamp = Clock().Format(time.RFC3339)
	rec.GitHead = gitHead
	rec.Actor = recordActor()
	return rec
}

// ReadClarifications projects the clarification transitions out of state records
// in append order (id, then sequence). Malformed clarification records fail
// closed: a readiness projection built from a record it cannot read would under-
// report a block.
func ReadClarifications(records map[string]json.RawMessage) ([]ClarificationRecord, error) {
	keys := make([]string, 0, len(records))
	for key := range records {
		if strings.HasPrefix(key, clarificationRecordPrefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	out := make([]ClarificationRecord, 0, len(keys))
	for _, key := range keys {
		var rec ClarificationRecord
		if err := json.Unmarshal(records[key], &rec); err != nil {
			return nil, fmt.Errorf("decode clarification record %s: %w", key, err)
		}
		out = append(out, rec)
	}
	return out, nil
}

// PlanClarification validates rec against the existing chain and returns the
// record key it must be appended under, plus the record with the entity
// identity inherited from its open record. It never rewrites an existing key.
func PlanClarification(existing []ClarificationRecord, rec ClarificationRecord) (string, ClarificationRecord, error) {
	if strings.TrimSpace(rec.ID) == "" {
		return "", rec, errors.New("clarification id is required")
	}
	var open *ClarificationRecord
	for i, prior := range existing {
		if prior.ID != rec.ID {
			continue
		}
		if prior.Transition == ClarificationOpen {
			open = &existing[i]
			continue
		}
		return "", rec, fmt.Errorf("clarification %s is already %s", rec.ID, prior.Transition)
	}
	switch rec.Transition {
	case ClarificationOpen:
		if open != nil {
			return "", rec, fmt.Errorf("clarification %s is already open", rec.ID)
		}
		if strings.TrimSpace(rec.Question) == "" {
			return "", rec, errors.New("clarification question is required")
		}
		switch rec.EntityKind {
		case ClarificationEntitySpec, ClarificationEntityTask, ClarificationEntityArtifact:
		default:
			return "", rec, fmt.Errorf("unknown clarification entity kind %q", rec.EntityKind)
		}
		if strings.TrimSpace(rec.EntityID) == "" {
			return "", rec, errors.New("clarification entity id is required")
		}
		return clarificationRecordPrefix + rec.ID + ":0", rec, nil
	case ClarificationAnswered, ClarificationWithdrawn, ClarificationExpired:
		if open == nil {
			return "", rec, fmt.Errorf("clarification %s is not open", rec.ID)
		}
		if rec.Transition == ClarificationAnswered && strings.TrimSpace(rec.Answer) == "" {
			return "", rec, errors.New("clarification answer is required")
		}
		// The resolution inherits the question's identity verbatim; only the
		// version it was resolved against is new.
		rec.EntityKind, rec.EntityID, rec.Blocking = open.EntityKind, open.EntityID, open.Blocking
		rec.Question = open.Question
		return clarificationRecordPrefix + rec.ID + ":1", rec, nil
	}
	return "", rec, fmt.Errorf("unknown clarification transition %q", rec.Transition)
}

// NextClarificationID returns the next unused id. A changed question takes a new
// id rather than mutating an existing record (R4.3).
func NextClarificationID(existing []ClarificationRecord) string {
	next := 1
	for _, rec := range existing {
		if rec.Transition == ClarificationOpen {
			next++
		}
	}
	return fmt.Sprintf("C%d", next)
}

// ClarificationTaskFacts projects clarifications into per-task readiness facts.
// current maps a task id to its current entity version; a task absent from it is
// never reported stale. Only blocking, task-scoped clarifications produce a
// wait, so a non-blocking question and a question about another entity leave
// readiness untouched (R4.1). Withdrawn and expired resolutions restore
// eligibility; an answer pinned to a superseded version keeps the answer as
// history and names the review it now needs (R4.2, R4.3).
func ClarificationTaskFacts(records []ClarificationRecord, current map[string]string) map[string]TaskFacts {
	latest := map[string]ClarificationRecord{}
	order := make([]string, 0, len(records))
	for _, rec := range records {
		if _, seen := latest[rec.ID]; !seen {
			order = append(order, rec.ID)
		}
		latest[rec.ID] = rec
	}
	sort.Strings(order)
	facts := map[string]TaskFacts{}
	for _, id := range order {
		rec := latest[id]
		if !rec.Blocking || rec.EntityKind != ClarificationEntityTask {
			continue
		}
		wait, ok := clarificationWait(rec, current[rec.EntityID])
		if !ok {
			continue
		}
		fact := facts[rec.EntityID]
		fact.Waits = append(fact.Waits, wait)
		facts[rec.EntityID] = fact
	}
	return facts
}

func clarificationWait(rec ClarificationRecord, currentVersion string) (WaitReason, bool) {
	switch rec.Transition {
	case ClarificationOpen:
		return WaitReason{
			Code: WaitClarificationOpen, Readiness: ReadinessWaitingClarification, Refs: []string{rec.ID},
			Owner: "human", Recovery: "answer with `specd clarification answer <spec> " + rec.ID + " --answer <text>`",
			Review: rec.Question,
		}, true
	case ClarificationAnswered:
		if currentVersion == "" || rec.EntityVersion == "" || currentVersion == rec.EntityVersion {
			return WaitReason{}, false
		}
		return WaitReason{
			Code: WaitClarificationStale, Readiness: ReadinessWaitingClarification, Refs: []string{rec.ID},
			Owner:    "human",
			Recovery: "open a new clarification for the revised task with `specd clarification open <spec> --entity task:" + rec.EntityID + " --question <text> --blocking`",
			Review:   "answer was given against version " + rec.EntityVersion + "; the task is now " + currentVersion,
		}, true
	}
	return WaitReason{}, false
}

// TaskEntityVersion is the version a clarification pins for a task entity: the
// digest of the task's contract. The completion marker is excluded so recording
// progress never invalidates an answer — only a revised contract does (R4.3).
func TaskEntityVersion(task TaskRow) string {
	return Digest([]byte(strings.Join([]string{
		task.ID, task.Role, task.Files, strings.Join(task.DependsOn, ","), task.Verify, task.Acceptance,
	}, "\x00")))
}

// TaskEntityVersions is TaskEntityVersion for every row, keyed by task id.
func TaskEntityVersions(tasks []TaskRow) map[string]string {
	versions := make(map[string]string, len(tasks))
	for _, task := range tasks {
		versions[task.ID] = TaskEntityVersion(task)
	}
	return versions
}

// ApplyClarificationReadiness recomputes the report model's readiness with the
// clarification facts applied. Readiness itself stays owned by ProjectTaskStates
// (R3.5): this only supplies the persisted waiting_clarification facts.
func ApplyClarificationReadiness(model *ReportModel, tasks []TaskRow, status map[string]TaskRunStatus, records map[string]json.RawMessage) error {
	clarifications, err := ReadClarifications(records)
	if err != nil {
		return err
	}
	if len(clarifications) == 0 {
		return nil
	}
	states, err := ProjectTaskStates(tasks, status, ClarificationTaskFacts(clarifications, TaskEntityVersions(tasks)))
	if err != nil {
		// An unprojectable graph is the DAG gate's finding, not this one's.
		return nil
	}
	projected := make(map[string]TaskState, len(states))
	for _, state := range states {
		projected[state.ID] = state
	}
	for i, task := range model.Tasks {
		state, ok := projected[task.ID]
		if !ok {
			continue
		}
		model.Tasks[i].Readiness, model.Tasks[i].Waits = state.Readiness, state.Waits
	}
	model.PendingBlockers = PendingCompletionBlockers(states)
	return nil
}

// BlockingClarifications returns the ids of the open, blocking clarifications
// scoped to task, in stable order.
func BlockingClarifications(records map[string]json.RawMessage, task string) ([]string, error) {
	clarifications, err := ReadClarifications(records)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, wait := range ClarificationTaskFacts(clarifications, nil)[task].Waits {
		ids = append(ids, wait.Refs...)
	}
	return ids, nil
}
