package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendTrajectoryEventDeterministicJSONL(t *testing.T) {
	root := t.TempDir()
	slug := "demo"
	if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 2, 12, 34, 56, 789, time.UTC)
	oldClock := Clock
	Clock = func() time.Time { return now }
	defer func() { Clock = oldClock }()

	argsDigest, err := TrajectoryDigestJSON(map[string]string{"secret": "swordfish"})
	if err != nil {
		t.Fatal(err)
	}
	exitCode := 0
	err = AppendTrajectoryEvent(root, slug, TrajectoryEvent{
		Actor:      "pinky",
		Kind:       "tool",
		Tool:       "verify",
		TaskIDs:    []string{"T1"},
		ArgsDigest: argsDigest,
		CwdDigest:  TrajectoryDigestBytes([]byte("/work")),
		ExitCode:   &exitCode,
		WallMs:     42,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = AppendTrajectoryEvent(root, slug, TrajectoryEvent{
		Actor: "mcp",
		Kind:  "result",
		Tool:  "verify",
	})
	if err != nil {
		t.Fatal(err)
	}

	path := TrajectoryPath(root, slug)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	want := `{"seq":1,"at":"2026-07-02T12:34:56.000000789Z","actor":"pinky","kind":"tool","tool":"verify","taskIds":["T1"],"argsDigest":"` + argsDigest + `","cwdDigest":"` + TrajectoryDigestBytes([]byte("/work")) + `","exitCode":0,"wallMs":42}` + "\n" +
		`{"seq":2,"at":"2026-07-02T12:34:56.000000789Z","actor":"mcp","kind":"result","tool":"verify"}` + "\n"
	if got != want {
		t.Fatalf("ledger bytes:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "swordfish") {
		t.Fatalf("raw arg value leaked into ledger: %s", got)
	}

	events, err := ReadTrajectory(root, slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Seq != 1 || events[1].Seq != 2 {
		t.Fatalf("events = %#v", events)
	}
}

func TestReadTrajectoryAbsentIsEmpty(t *testing.T) {
	root := t.TempDir()
	events, err := ReadTrajectory(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("events = %#v, want empty", events)
	}
}

func TestTrajectoryRejectsHostileInput(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(SpecDir(root, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, event := range map[string]TrajectoryEvent{
		"nul":        {Kind: "tool\x00call"},
		"rawDigest":  {Kind: "tool", ArgsDigest: "secret-value"},
		"negativeMs": {Kind: "tool", WallMs: -1},
	} {
		t.Run(name, func(t *testing.T) {
			err := AppendTrajectoryEvent(root, "demo", event)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
	if err := AppendTrajectoryEvent(root, "../escape", TrajectoryEvent{Kind: "tool"}); err == nil {
		t.Fatal("expected slug validation error")
	}
	if _, err := TrajectoryDigestJSON(map[string]string{"arg": "bad\x00value"}); err == nil {
		t.Fatal("expected digest NUL rejection")
	}
	if _, err := TrajectoryDigestJSON([]byte{'o', 'k', 0}); err == nil {
		t.Fatal("expected byte digest NUL rejection")
	}
}

func TestDecodeTrajectoryRejectsOversizeLineAndSeqGap(t *testing.T) {
	oversize := strings.NewReader(`{"seq":1,"at":"2026-07-02T00:00:00Z","kind":"` + strings.Repeat("x", MaxTrajectoryLineBytes) + `"}` + "\n")
	if _, err := DecodeTrajectory(oversize); err == nil {
		t.Fatal("expected oversize error")
	}

	gap := strings.NewReader(`{"seq":2,"at":"2026-07-02T00:00:00Z","kind":"tool"}` + "\n")
	if _, err := DecodeTrajectory(gap); err == nil || !strings.Contains(err.Error(), "seq") {
		t.Fatalf("gap err = %v, want seq error", err)
	}
}

func TestTrajectoryParseSerializeIsByteStable(t *testing.T) {
	line := `{"seq":1,"at":"2026-07-02T00:00:00Z","actor":"pinky","kind":"tool","tool":"next","argsDigest":"` + TrajectoryDigestBytes([]byte("args")) + `"}` + "\n"
	events, err := DecodeTrajectory(strings.NewReader(line))
	if err != nil {
		t.Fatal(err)
	}
	got, err := MarshalTrajectoryEvent(events[0])
	if err != nil {
		t.Fatal(err)
	}
	if got != line {
		t.Fatalf("marshal = %q, want %q", got, line)
	}
}

func TestMarshalTrajectoryEventRejectsOversizeLine(t *testing.T) {
	_, err := MarshalTrajectoryEvent(TrajectoryEvent{
		Seq:  1,
		At:   "2026-07-02T00:00:00Z",
		Kind: strings.Repeat("x", MaxTrajectoryLineBytes),
	})
	if err == nil {
		t.Fatal("expected oversize error")
	}
}

func TestTryAppendTrajectoryEventIsInertOnFailure(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, ".specd")
	if err := os.WriteFile(blocker, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	TryAppendTrajectoryEvent(root, "demo", TrajectoryEvent{Kind: "tool"})
}
