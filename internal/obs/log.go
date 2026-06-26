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

const (
	logFormatJSON = "json"
	logFormatText = "text"
)

type logFieldKey string

const (
	logFieldSlug  logFieldKey = "slug"
	logFieldPhase logFieldKey = "phase"
	logFieldRole  logFieldKey = "role"
)

// NewLogger returns a slog logger that writes to stderr only.
// SPECD_LOG_FORMAT selects JSON or text output; JSON is the default.
func NewLogger() *slog.Logger {
	return newLogger(os.Stderr, ParseLevel(os.Getenv("SPECD_LOG")), ParseFormat(os.Getenv("SPECD_LOG_FORMAT")))
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

// NewSessionLogger mirrors logs to .specd/sessions/<sessionID>/brain.log.
// File setup is best-effort: failures are warned on stderr and logging continues
// to stderr only.
func NewSessionLogger(root, sessionID string) (*slog.Logger, io.Closer) {
	level := ParseLevel(os.Getenv("SPECD_LOG"))
	format := ParseFormat(os.Getenv("SPECD_LOG_FORMAT"))
	stderr := os.Stderr
	file, err := openBrainLog(root, sessionID)
	if err != nil {
		logger := newLogger(stderr, level, format)
		logger.Warn("brain log file unavailable", "event", "log_open_failed", "session_id", sessionID, "session", sessionID, "error", err.Error())
		return logger, nil
	}
	stderrHandler := newHandler(stderr, level, format)
	fileHandler := newHandler(file, slog.LevelDebug, format)
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

func newLogger(w io.Writer, level slog.Level, format string) *slog.Logger {
	return slog.New(newHandler(w, level, format))
}

func newHandler(w io.Writer, level slog.Level, format string) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	if ParseFormat(format) == logFormatText {
		return slog.NewTextHandler(w, opts)
	}
	return slog.NewJSONHandler(w, opts)
}

// ParseFormat maps SPECD_LOG_FORMAT to a slog output mode.
func ParseFormat(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case logFormatText:
		return logFormatText
	case "", logFormatJSON:
		return logFormatJSON
	default:
		return logFormatJSON
	}
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
	Time     string `json:"time,omitempty"`
	Level    string `json:"level,omitempty"`
	Message  string `json:"msg,omitempty"`
	Event    string `json:"event"`
	Slug     string `json:"slug,omitempty"`
	Phase    string `json:"phase,omitempty"`
	Role     string `json:"role,omitempty"`
	Session  string `json:"session,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Worker   string `json:"worker,omitempty"`
	Task     string `json:"task,omitempty"`
	DurMS    int64  `json:"dur_ms,omitempty"`
	Exit     int    `json:"exit,omitempty"`
}

// WithFields annotates ctx with optional slug/phase/role log fields.
func WithFields(ctx context.Context, slug, phase, role string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if slug != "" {
		ctx = context.WithValue(ctx, logFieldSlug, slug)
	}
	if phase != "" {
		ctx = context.WithValue(ctx, logFieldPhase, phase)
	}
	if role != "" {
		ctx = context.WithValue(ctx, logFieldRole, role)
	}
	return ctx
}

func logFieldString(ctx context.Context, key logFieldKey) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(key).(string)
	return v
}

// LogEvent emits one stable Brain event.
func LogEvent(ctx context.Context, logger *slog.Logger, event, session, worker, task string, dur time.Duration, exit int) {
	if logger == nil {
		return
	}
	slug := logFieldString(ctx, logFieldSlug)
	phase := logFieldString(ctx, logFieldPhase)
	role := logFieldString(ctx, logFieldRole)
	attrs := []any{
		"event", event,
		"slug", slug,
		"phase", phase,
		"role", role,
		"session_id", session,
		"session", session,
		"worker", worker,
		"task", task,
	}
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
