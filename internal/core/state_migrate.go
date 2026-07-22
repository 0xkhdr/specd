package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StateMigrationPlan is the dry-run result of mapping one schema-1 spec onto
// schema 2 (spec 03 R6.1). It maps to cycle 1 and a single synthetic baseline
// event; nothing on disk changes until CommitStateMigration runs.
type StateMigrationPlan struct {
	Slug            string      `json:"slug"`
	Cycle           int         `json:"cycle"`
	Stage           Stage       `json:"stage"`
	Condition       Condition   `json:"condition"`
	Status          Status      `json:"status"`
	Revision        int64       `json:"revision"`
	BackupPath      string      `json:"backup_path"`
	BackupMode      os.FileMode `json:"backup_mode"`
	BaselineEventID string      `json:"baseline_event_id"`
}

// PlanStateMigration reads the v1 state and returns what migration would do.
func PlanStateMigration(statePath, eventPath string) (StateMigrationPlan, error) {
	plan, _, _, err := planStateMigration(statePath, eventPath)
	return plan, err
}

func planStateMigration(statePath, eventPath string) (StateMigrationPlan, State, WorkflowEventV1, error) {
	raw, err := os.ReadFile(statePath)
	if err != nil {
		return StateMigrationPlan{}, State{}, WorkflowEventV1{}, err
	}
	// Refuse a future schema before anything is read as meaning (spec 03 R1.4).
	if err := PreflightStateSchema(raw); err != nil {
		return StateMigrationPlan{}, State{}, WorkflowEventV1{}, err
	}
	legacy, err := DecodeState(raw)
	if err != nil {
		return StateMigrationPlan{}, State{}, WorkflowEventV1{}, fmt.Errorf("decode %s: %w", statePath, err)
	}
	if legacy.SchemaVersion != LegacyStateSchemaVersion {
		return StateMigrationPlan{}, State{}, WorkflowEventV1{}, fmt.Errorf("state %s is already schema %d", statePath, legacy.SchemaVersion)
	}
	if legacy.Status == StatusBlocked && legacy.Stage == "" {
		return StateMigrationPlan{}, State{}, WorkflowEventV1{}, fmt.Errorf("cannot migrate %s: legacy blocked state does not reveal its prior stage — record the stage the spec was blocked in (requirements|design|tasks|executing|verifying) and re-run migration", statePath)
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		return StateMigrationPlan{}, State{}, WorkflowEventV1{}, err
	}
	// A projection that already claims events the ledger cannot show is
	// corruption: restore from backup, never invent the missing event.
	if len(events) > 0 || legacy.LastEventID != "" {
		return StateMigrationPlan{}, State{}, WorkflowEventV1{}, fmt.Errorf("cannot migrate %s: projection is ahead of or diverges from the workflow ledger", statePath)
	}

	baseline := legacy
	baseline.SchemaVersion = StateSchemaVersion
	baseline.projectCanonical()
	if err := baseline.Validate(); err != nil {
		return StateMigrationPlan{}, State{}, WorkflowEventV1{}, err
	}
	event, err := NewWorkflowEvent(WorkflowEventV1{
		EntityKind: "spec", EntityID: baseline.Slug,
		BeforeEntityVersion: 0, AfterEntityVersion: 1,
		ExpectedRevision: baseline.Revision,
		Transition:       "state.migrate.v1",
		Actor:            recordActor(),
		AuthorityDigest:  Digest(raw),
		Reason:           "schema 1 state migrated to schema 2",
		InputDigests:     map[string]string{"state.v1.json": Digest(raw)},
		ImpactedEntities: []string{"spec:" + baseline.Slug},
		Timestamp:        Clock().Format(time.RFC3339),
		Projection:       baseline,
	})
	if err != nil {
		return StateMigrationPlan{}, State{}, WorkflowEventV1{}, err
	}
	mode := os.FileMode(0o600)
	if info, err := os.Stat(statePath); err == nil {
		mode = info.Mode().Perm()
	}
	plan := StateMigrationPlan{
		Slug: baseline.Slug, Cycle: baseline.Cycle, Stage: baseline.Stage,
		Condition: baseline.Condition, Status: baseline.Status, Revision: legacy.Revision,
		BackupPath: filepath.Join(filepath.Dir(statePath), "state.v1.json.bak"),
		BackupMode: mode, BaselineEventID: event.ID,
	}
	return plan, legacy, event, nil
}

// CommitStateMigration preserves a permission-matched backup, writes the
// baseline ledger, replays it into the candidate v2 projection, proves the
// effective meaning is unchanged, and only then activates v2 by CAS
// (spec 03 R6.3). Any failure returns before activation, leaving v1 readable.
func CommitStateMigration(statePath, eventPath string) (StateMigrationPlan, error) {
	plan, legacy, event, err := planStateMigration(statePath, eventPath)
	if err != nil {
		return StateMigrationPlan{}, err
	}
	raw, err := os.ReadFile(statePath)
	if err != nil {
		return StateMigrationPlan{}, err
	}
	// os.WriteFile applies umask to a new file, so restate the mode explicitly.
	if err := os.WriteFile(plan.BackupPath, raw, plan.BackupMode); err != nil {
		return StateMigrationPlan{}, fmt.Errorf("preserve v1 state backup: %w", err)
	}
	if err := os.Chmod(plan.BackupPath, plan.BackupMode); err != nil {
		return StateMigrationPlan{}, fmt.Errorf("preserve v1 state backup permissions: %w", err)
	}
	if err := AppendWorkflowEvent(eventPath, event); err != nil {
		return StateMigrationPlan{}, err
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		return StateMigrationPlan{}, err
	}
	replayBase := event.Projection
	replayBase.Revision = event.ExpectedRevision
	replayBase.LastEventID = ""
	candidate, err := ReplayWorkflowEvents(replayBase, events)
	if err != nil {
		return StateMigrationPlan{}, err
	}
	if err := sameEffectiveMeaning(legacy, candidate); err != nil {
		return StateMigrationPlan{}, err
	}
	if err := SaveStateCAS(statePath, legacy.Revision, candidate); err != nil {
		return StateMigrationPlan{}, err
	}
	return plan, nil
}

// sameEffectiveMeaning is the conformance check that the migrated projection
// says exactly what the v1 state said (spec 03 R6.1, R6.4).
func sameEffectiveMeaning(legacy, candidate State) error {
	if candidate.Status != legacy.Status || candidate.Phase != legacy.Phase || candidate.Mode != legacy.Mode || candidate.Slug != legacy.Slug {
		return errors.New("migration changed the effective spec status, phase, mode, or slug")
	}
	if len(candidate.TaskStatus) != len(legacy.TaskStatus) {
		return errors.New("migration changed task status")
	}
	for id, status := range legacy.TaskStatus {
		if candidate.TaskStatus[id] != status {
			return fmt.Errorf("migration changed task %s status", id)
		}
	}
	if len(candidate.Records) != len(legacy.Records) {
		return errors.New("migration changed the record ledger")
	}
	for key, record := range legacy.Records {
		if !json.Valid(record) || string(candidate.Records[key]) != string(record) {
			return fmt.Errorf("migration changed record %s", key)
		}
	}
	return nil
}
