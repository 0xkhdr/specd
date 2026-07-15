package core

import (
	"strings"
	"testing"
	"time"
)

func TestListResumableSessionsIncludesProgramParent(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()
	parentID := strings.Repeat("e", 32)
	if _, err := ensureProgramSession(root, parentID); err != nil {
		t.Fatalf("ensureProgramSession: %v", err)
	}
	if err := SaveProgramState(root, ProgramState{
		Version:         OrchestrationModelVersion,
		ParentSessionID: parentID,
		ChildSessions:   map[string]string{"done": strings.Repeat("1", 32), "todo": strings.Repeat("2", 32)},
		InflightKeys:    []string{"todo:T1"},
		ChildStatus:     map[string]SpecStatus{"done": StatusComplete, "todo": StatusExecuting},
		UpdatedAt:       now.Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("SaveProgramState: %v", err)
	}
	items, err := ListResumableSessions(root, time.Hour)
	if err != nil {
		t.Fatalf("ListResumableSessions: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%#v", items)
	}
	item := items[0]
	if !item.Program || item.SessionID != parentID || item.ChildrenComplete != 1 || item.ChildrenTotal != 2 {
		t.Fatalf("program item=%#v", item)
	}

	if _, err := PauseProgramOrchestration(root, parentID); err != nil {
		t.Fatalf("PauseProgramOrchestration: %v", err)
	}
	paused, ok := programResumableSession(root, parentID, now, time.Hour)
	if !ok || paused.PausedSince == "" || !paused.Program {
		t.Fatalf("paused program discovery = %#v ok=%v", paused, ok)
	}
	if _, ok := programResumableSession(root, parentID, now.Add(2*time.Hour), time.Hour); ok {
		t.Fatal("max-age should filter old program session")
	}
	if _, err := markProgramSessionStatus(root, parentID, OrchestrationSessionComplete); err != nil {
		t.Fatalf("mark complete: %v", err)
	}
	if _, ok := programResumableSession(root, parentID, now, 0); ok {
		t.Fatal("complete program should not be resumable")
	}
}
