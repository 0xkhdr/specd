package cmd

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/obs"
)

func TestBrainWhyRendersStructuredTimeline(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SPECD_LOG", "info")
	logger, closer := obs.NewSessionLogger(root, "s-why")
	obs.LogEvent(context.Background(), logger, "dispatch", "s-why", "worker-1", "T1", 0, 0)
	obs.LogEvent(context.Background(), logger, "complete", "s-why", "worker-1", "T1", 0, 0)
	if closer != nil {
		closer.Close()
	}

	out := captureStdout(t, func() int {
		return brainWhy(root, cli.Args{Flags: map[string]string{"session": "s-why"}})
	})
	if !strings.Contains(out, "brain why — s-why") || !strings.Contains(out, "dispatch") || !strings.Contains(out, "complete") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestBrainWhyRendersStructuredTimelineJSON(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SPECD_LOG", "info")
	logger, closer := obs.NewSessionLogger(root, "s-json")
	obs.LogEvent(context.Background(), logger, "timeout", "s-json", "worker-1", "T1", 0, 124)
	if closer != nil {
		closer.Close()
	}

	out := captureStdout(t, func() int {
		return brainWhy(root, cli.Args{Flags: map[string]string{"session": "s-json", "json": "true"}})
	})
	if !strings.Contains(out, `"session": "s-json"`) || !strings.Contains(out, `"event": "timeout"`) {
		t.Fatalf("unexpected JSON:\n%s", out)
	}
}

func TestBrainWhyMissingTimeline(t *testing.T) {
	root := t.TempDir()
	code := brainWhy(root, cli.Args{Flags: map[string]string{"session": "missing"}})
	if code != core.ExitNotFound {
		t.Fatalf("code = %d, want %d", code, core.ExitNotFound)
	}
}

func TestBrainObserverDoesNotWriteStdout(t *testing.T) {
	var stderr bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	out := captureStdout(t, func() int {
		brainObserver(logger)(core.DriverEvent{Event: "dispatch", Session: "s", Worker: "w", Task: "T1"})
		return core.ExitOK
	})
	if out != "" {
		t.Fatalf("logger wrote stdout: %q", out)
	}
	if !strings.Contains(stderr.String(), `"event":"dispatch"`) {
		t.Fatalf("stderr missing event: %q", stderr.String())
	}
}

func captureStdout(t *testing.T, fn func() int) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	code := fn()
	_ = w.Close()
	os.Stdout = old
	if code != core.ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	return buf.String()
}
