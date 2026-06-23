package obs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// log_cov_test.go covers the logger constructors and the teeHandler attr/group
// fan-out, plus LogEvent's warn/duration/exit branches and the timeline/path
// error paths the happy-path test leaves open.

func TestNewLogger(t *testing.T) {
	if NewLogger() == nil {
		t.Fatal("NewLogger returned nil")
	}
}

func TestSessionLoggerWithAttrsAndGroup(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SPECD_LOG", "debug")
	logger, closer := NewSessionLogger(root, "s-2")
	if closer != nil {
		defer closer.Close()
	}

	// .With / .WithGroup fan out across the teeHandler to both sinks.
	derived := logger.With("k", "v").WithGroup("grp")
	LogEvent(context.Background(), derived, "timeout", "s-2", "wkr", "T1", 250*time.Millisecond, 0)
	LogEvent(context.Background(), derived, "complete", "s-2", "wkr", "T1", 0, 0)
	LogEvent(context.Background(), derived, "escalate", "s-2", "wkr", "T2", time.Second, 2)
	// nil logger is a no-op, not a panic.
	LogEvent(context.Background(), nil, "dispatch", "s-2", "wkr", "T1", 0, 0)

	if closer != nil {
		closer.Close()
	}
	if _, err := os.Stat(filepath.Join(root, ".specd", "sessions", "s-2", "brain.log")); err != nil {
		t.Fatalf("brain.log not written: %v", err)
	}
}

func TestSessionLoggerInvalidSessionDegrades(t *testing.T) {
	// An invalid session id makes openBrainLog fail; logging degrades to
	// stderr-only with a nil closer rather than erroring.
	logger, closer := NewSessionLogger(t.TempDir(), "bad id/with slash")
	if logger == nil {
		t.Fatal("degraded logger should still be usable")
	}
	if closer != nil {
		closer.Close()
		t.Fatal("invalid session should yield a nil closer")
	}
}

func TestBrainLogPathAndTimelineErrors(t *testing.T) {
	if _, err := BrainLogPath("/root", "bad id"); err == nil {
		t.Fatal("invalid session id should error")
	}
	if _, err := ReadTimeline("/root", "bad id"); err == nil {
		t.Fatal("ReadTimeline with bad session id should error")
	}
	// Missing log file → open error.
	if _, err := ReadTimeline(t.TempDir(), "s-missing"); err == nil {
		t.Fatal("ReadTimeline for missing log should error")
	}
}
