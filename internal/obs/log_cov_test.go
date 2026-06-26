package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	t.Setenv("SPECD_LOG_FORMAT", "text")
	if NewLogger() == nil {
		t.Fatal("text logger returned nil")
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
	ctx := WithFields(context.Background(), "demo", "execute", "builder")
	LogEvent(ctx, derived, "timeout", "s-2", "wkr", "T1", 250*time.Millisecond, 0)
	LogEvent(ctx, derived, "complete", "s-2", "wkr", "T1", 0, 0)
	LogEvent(ctx, derived, "escalate", "s-2", "wkr", "T2", time.Second, 2)
	// nil logger is a no-op, not a panic.
	LogEvent(context.Background(), nil, "dispatch", "s-2", "wkr", "T1", 0, 0)

	if closer != nil {
		closer.Close()
	}
	path := filepath.Join(root, ".specd", "sessions", "s-2", "brain.log")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("brain.log not written: %v", err)
	}
	if !strings.Contains(string(raw), "\"slug\":\"demo\"") || !strings.Contains(string(raw), "\"session_id\":\"s-2\"") {
		t.Fatalf("structured fields missing: %s", raw)
	}
	first, _, _ := strings.Cut(string(raw), "\n")
	first = strings.TrimSpace(first)
	if !json.Valid([]byte(first)) {
		t.Fatalf("brain.log not json: %s", first)
	}
	var ev map[string]any
	if err := json.Unmarshal(bytes.TrimSpace([]byte(first)), &ev); err != nil {
		t.Fatalf("brain.log decode: %v", err)
	}
	grp, ok := ev["grp"].(map[string]any)
	if !ok || grp["slug"] != "demo" || grp["session_id"] != "s-2" {
		t.Fatalf("missing structured fields: %v", ev)
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
