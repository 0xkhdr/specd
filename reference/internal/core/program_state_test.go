package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProgramStateRoundTripAndValidation(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 6, 27, 10, 11, 12, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	parentID := strings.Repeat("a", 32)
	state := ProgramState{
		Version:         OrchestrationModelVersion,
		ParentSessionID: parentID,
		ChildSessions:   map[string]string{"beta": strings.Repeat("b", 32), "alpha": strings.Repeat("c", 32)},
		InflightKeys:    []string{"z", "a"},
		ChildStatus:     map[string]SpecStatus{"alpha": StatusComplete, "beta": StatusExecuting},
	}
	if err := SaveProgramState(root, state); err != nil {
		t.Fatalf("SaveProgramState: %v", err)
	}
	loaded, err := LoadProgramState(root, parentID)
	if err != nil {
		t.Fatalf("LoadProgramState: %v", err)
	}
	if loaded.UpdatedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("updatedAt = %q", loaded.UpdatedAt)
	}
	if got := loaded.CompleteChildCount(); got != 1 {
		t.Fatalf("CompleteChildCount = %d, want 1", got)
	}
	if len(loaded.InflightKeys) != 2 || loaded.InflightKeys[0] != "a" || loaded.InflightKeys[1] != "z" {
		t.Fatalf("inflight keys not canonical: %#v", loaded.InflightKeys)
	}

	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	path, err := paths.ProgramStatePath(parentID)
	if err != nil {
		t.Fatal(err)
	}
	raw := ReadOrNull(path)
	if raw == nil || !strings.Contains(*raw, "\n  \"childSessions\":") || !strings.HasSuffix(*raw, "\n") {
		t.Fatalf("program-state not canonical JSON: %q", valueOrEmpty(raw))
	}

	var corrupt ProgramState
	if err := json.Unmarshal([]byte(*raw), &corrupt); err != nil {
		t.Fatal(err)
	}
	corrupt.ChildStatus["bad/slug"] = StatusComplete
	if err := ValidateProgramState(corrupt); err == nil {
		t.Fatal("ValidateProgramState accepted bad slug")
	}
}

func TestProgramStateMissingAndProgressWindow(t *testing.T) {
	root := t.TempDir()
	if _, err := LoadProgramState(root, strings.Repeat("d", 32)); err == nil {
		t.Fatal("LoadProgramState missing file: err = nil")
	}

	base := time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return base })
	defer restore()
	cfg := DefaultConfig.Orchestration
	cfg.Resilience = &ResilienceCfg{ProgressTimeoutSeconds: 300}
	fresh := OrchestrationSnapshot{MostRecentProgressAt: base.Add(-299 * time.Second).Format(time.RFC3339Nano)}
	if !progressWithinWindow(fresh, cfg) {
		t.Fatal("fresh progress should weight wait as zero")
	}
	stale := OrchestrationSnapshot{MostRecentProgressAt: base.Add(-300 * time.Second).Format(time.RFC3339Nano)}
	if progressWithinWindow(stale, cfg) {
		t.Fatal("stale progress should count as wait")
	}
	cfg.Resilience.ProgressTimeoutSeconds = 0
	if progressWithinWindow(fresh, cfg) {
		t.Fatal("disabled progress weighting should count as wait")
	}
	if progressWithinWindow(OrchestrationSnapshot{MostRecentProgressAt: "not-time"}, DefaultConfig.Orchestration) {
		t.Fatal("bad timestamp should not weight wait")
	}
}

func valueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
