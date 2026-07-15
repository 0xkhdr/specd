package obs

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"":      slog.LevelWarn,
		"nope":  slog.LevelWarn,
	}
	for in, want := range tests {
		if got := ParseLevel(in); got != want {
			t.Fatalf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestNewSessionLoggerWritesBrainLog(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SPECD_LOG", "info")
	logger, closer := NewSessionLogger(root, "s-1")
	if closer != nil {
		defer closer.Close()
	}
	LogEvent(WithFields(context.Background(), "auth", "execute", "craftsman"), logger, "dispatch", "s-1", "worker-1", "T1", 0, 0)
	if closer != nil {
		closer.Close()
	}
	path := filepath.Join(root, ".specd", "sessions", "s-1", "brain.log")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read brain.log: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("brain.log empty")
	}
	events, err := ReadTimeline(root, "s-1")
	if err != nil {
		t.Fatalf("ReadTimeline: %v", err)
	}
	if len(events) != 1 || events[0].Event != "dispatch" || events[0].Worker != "worker-1" || events[0].Slug != "auth" || events[0].Phase != "execute" || events[0].Role != "craftsman" {
		t.Fatalf("events = %+v", events)
	}
}
