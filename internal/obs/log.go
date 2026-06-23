// Package obs owns specd's structured operational logging.
//
// Loggers are hard-wired to stderr so Brain tracing never mutates stdout, whose
// bytes are part of specd's deterministic CLI contract.
package obs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var sessionIDRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// NewLogger returns a JSON slog logger that writes to stderr only.
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: ParseLevel(os.Getenv("SPECD_LOG"))}))
}

// ParseLevel maps SPECD_LOG to a slog level. Unknown values fail closed to warn.
func ParseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning", "":
		return slog.LevelWarn
	default:
		return slog.LevelWarn
	}
}

// NewSessionLogger mirrors JSON logs to .specd/sessions/<sessionID>/brain.log.
// File setup is best-effort: failures are warned on stderr and logging continues
// to stderr only.
func NewSessionLogger(root, sessionID string) (*slog.Logger, io.Closer) {
	level := ParseLevel(os.Getenv("SPECD_LOG"))
	stderr := os.Stderr
	file, err := openBrainLog(root, sessionID)
	if err != nil {
		logger := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: level}))
		logger.Warn("brain log file unavailable", "event", "log_open_failed", "session", sessionID, "error", err.Error())
		return logger, nil
	}
	stderrHandler := slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: level})
	fileHandler := slog.NewJSONHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(teeHandler{handlers: []slog.Handler{stderrHandler, fileHandler}}), file
}

func openBrainLog(root, sessionID string) (*os.File, error) {
	if !sessionIDRE.MatchString(sessionID) {
		return nil, fmt.Errorf("invalid session id %q", sessionID)
	}
	dir := filepath.Join(root, ".specd", "sessions", sessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // session dir holds non-secret brain logs; group/other-readable for shared CI checkouts (see SECURITY.md)
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, "brain.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644) //nolint:gosec // brain.log is a non-secret diagnostic log; world-readable by design (see SECURITY.md)
}

type teeHandler struct{ handlers []slog.Handler }

func (h teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h teeHandler) Handle(ctx context.Context, record slog.Record) error {
	var first error
	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		if err := handler.Handle(ctx, record.Clone()); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (h teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		out = append(out, handler.WithAttrs(attrs))
	}
	return teeHandler{handlers: out}
}

func (h teeHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		out = append(out, handler.WithGroup(name))
	}
	return teeHandler{handlers: out}
}

// BrainLogPath returns the structured timeline path for a Brain session.
func BrainLogPath(root, sessionID string) (string, error) {
	if !sessionIDRE.MatchString(sessionID) {
		return "", fmt.Errorf("invalid session id %q", sessionID)
	}
	return filepath.Join(root, ".specd", "sessions", sessionID, "brain.log"), nil
}

// TimelineEvent is one parsed Brain structured event.
type TimelineEvent struct {
	Time    string `json:"time,omitempty"`
	Level   string `json:"level,omitempty"`
	Message string `json:"msg,omitempty"`
	Event   string `json:"event"`
	Session string `json:"session,omitempty"`
	Worker  string `json:"worker,omitempty"`
	Task    string `json:"task,omitempty"`
	DurMS   int64  `json:"dur_ms,omitempty"`
	Exit    int    `json:"exit,omitempty"`
}

// LogEvent emits one stable Brain event.
func LogEvent(ctx context.Context, logger *slog.Logger, event, session, worker, task string, dur time.Duration, exit int) {
	if logger == nil {
		return
	}
	attrs := []any{"event", event, "session", session, "worker", worker, "task", task}
	if dur > 0 {
		attrs = append(attrs, "dur_ms", dur.Milliseconds())
	}
	if exit != 0 || event == "complete" {
		attrs = append(attrs, "exit", exit)
	}
	switch event {
	case "timeout", "escalate":
		logger.WarnContext(ctx, "brain event", attrs...)
	default:
		logger.InfoContext(ctx, "brain event", attrs...)
	}
}

// ReadTimeline parses NDJSON slog output and keeps records carrying an event.
func ReadTimeline(root, sessionID string) ([]TimelineEvent, error) {
	path, err := BrainLogPath(root, sessionID)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []TimelineEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var ev TimelineEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Event != "" {
			out = append(out, ev)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
