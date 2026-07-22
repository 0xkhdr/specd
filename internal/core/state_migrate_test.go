package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLegacyState writes a schema-1 state.json with an explicit permission bit
// so migration can be asserted to preserve it.
func writeLegacyState(t *testing.T, dir, raw string, mode os.FileMode) (string, string) {
	t.Helper()
	statePath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(statePath, []byte(raw), mode); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}
	if err := os.Chmod(statePath, mode); err != nil {
		t.Fatalf("chmod legacy state: %v", err)
	}
	return statePath, filepath.Join(dir, "workflow-events.jsonl")
}

const legacyCompleteState = `{"schema_version":1,"slug":"demo","mode":"default","status":"complete","phase":"reflect","revision":4,` +
	`"records":{"approval:requirements":{"kind":"approval","gate":"requirements","timestamp":"t","git_head":"h","actor":"a"}}}`

// TestStageConditionMigrationIsLosslessAndBackedUp pins spec 03 R6.1/R6.3/R6.4:
// a v1 spec maps to cycle 1 with a permission-matched backup, a baseline ledger,
// a replayed candidate projection, and an unchanged effective meaning.
func TestStageConditionMigrationIsLosslessAndBackedUp(t *testing.T) {
	statePath, eventPath := writeLegacyState(t, t.TempDir(), legacyCompleteState, 0o640)
	plan, err := CommitStateMigration(statePath, eventPath)
	if err != nil {
		t.Fatalf("commit migration: %v", err)
	}
	info, err := os.Stat(plan.BackupPath)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("backup mode = %v, want 0640", info.Mode().Perm())
	}
	backup, err := os.ReadFile(plan.BackupPath)
	if err != nil || string(backup) != legacyCompleteState {
		t.Fatalf("backup bytes = %q, err = %v", backup, err)
	}
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil || len(events) != 1 || events[0].ID != plan.BaselineEventID {
		t.Fatalf("baseline ledger = %+v, err = %v", events, err)
	}
	migrated, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("load migrated state: %v", err)
	}
	if migrated.SchemaVersion != StateSchemaVersion || migrated.Cycle != 1 {
		t.Fatalf("migrated schema/cycle = %d/%d", migrated.SchemaVersion, migrated.Cycle)
	}
	if migrated.Stage != StageComplete || migrated.Condition != ConditionComplete {
		t.Fatalf("migrated pair = %q/%q", migrated.Stage, migrated.Condition)
	}
	if migrated.Status != StatusComplete || migrated.Phase != PhaseReflect {
		t.Fatalf("migration changed effective meaning: %+v", migrated)
	}
	if migrated.Revision != 5 || migrated.LastEventID != plan.BaselineEventID {
		t.Fatalf("migrated checkpoint = %d/%q", migrated.Revision, migrated.LastEventID)
	}
	if len(migrated.Records) != 1 {
		t.Fatalf("migration changed records: %+v", migrated.Records)
	}
	if _, err := CommitStateMigration(statePath, eventPath); err == nil {
		t.Fatal("second migration of an already-migrated state was accepted")
	}
}

// TestStageConditionMigrationRefusesLegacyBlocked pins spec 03 R6.2: a v1
// blocked state cannot reveal the stage it was blocked in, so migration refuses
// and names the repair instead of guessing a stage.
func TestStageConditionMigrationRefusesLegacyBlocked(t *testing.T) {
	raw := `{"schema_version":1,"slug":"demo","mode":"default","status":"blocked","phase":"reflect","revision":2}`
	statePath, eventPath := writeLegacyState(t, t.TempDir(), raw, 0o644)
	_, err := PlanStateMigration(statePath, eventPath)
	if err == nil || !strings.Contains(err.Error(), "prior stage") {
		t.Fatalf("plan error = %v, want a prior-stage repair diagnostic", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(statePath), "state.v1.json.bak")); !os.IsNotExist(err) {
		t.Fatalf("refused migration wrote a backup: %v", err)
	}
}

// TestStageConditionMigrationRefusesFutureSchema pins spec 03 R1.4: state newer
// than this binary fails before any file is written.
func TestStageConditionMigrationRefusesFutureSchema(t *testing.T) {
	raw := `{"schema_version":99,"slug":"demo","mode":"default","status":"tasks","phase":"plan","revision":1}`
	statePath, eventPath := writeLegacyState(t, t.TempDir(), raw, 0o644)
	if _, err := CommitStateMigration(statePath, eventPath); err == nil {
		t.Fatal("migration accepted a future state schema")
	}
	if events, err := ReadWorkflowEvents(eventPath); err != nil || len(events) != 0 {
		t.Fatalf("future schema mutated the ledger: %+v, err = %v", events, err)
	}
}

// TestStageConditionMigrationAbortsOnBackupFailure pins spec 03 R6.3: without a
// preserved backup nothing is appended and v1 stays active.
func TestStageConditionMigrationAbortsOnBackupFailure(t *testing.T) {
	dir := t.TempDir()
	statePath, eventPath := writeLegacyState(t, dir, legacyCompleteState, 0o644)
	// A directory in the backup slot makes the backup write fail while the
	// spec directory itself stays writable.
	if err := os.Mkdir(filepath.Join(dir, "state.v1.json.bak"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitStateMigration(statePath, eventPath); err == nil {
		t.Fatal("migration activated without a backup")
	}
	if events, err := ReadWorkflowEvents(eventPath); err != nil || len(events) != 0 {
		t.Fatalf("backup failure still wrote the ledger: %+v, err = %v", events, err)
	}
	raw, err := os.ReadFile(statePath)
	if err != nil || string(raw) != legacyCompleteState {
		t.Fatalf("backup failure activated v2: %q, err = %v", raw, err)
	}
}

// TestStageConditionMigrationRefusesProjectionDrift pins spec 03 failure
// behaviour: a projection that claims an event the ledger cannot show is
// corruption, never an invitation to invent the missing event.
func TestStageConditionMigrationRefusesProjectionDrift(t *testing.T) {
	raw := `{"schema_version":1,"slug":"demo","mode":"default","status":"tasks","phase":"plan","revision":3,"last_event_id":"deadbeef"}`
	statePath, eventPath := writeLegacyState(t, t.TempDir(), raw, 0o644)
	_, err := CommitStateMigration(statePath, eventPath)
	if err == nil || !strings.Contains(err.Error(), "ledger") {
		t.Fatalf("migration error = %v, want a ledger corruption refusal", err)
	}
}
