package verify

import (
	"io"
	"regexp"
	"strings"
)

const Redacted = "[REDACTED]"

var credentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+)[A-Za-z0-9._~+/=-]{8,}`),
	regexp.MustCompile(`(?i)((?:api[_-]?key|access[_-]?token|secret|password)\s*[=:]\s*)[^\s,;]{4,}`),
}

// homePathPattern masks an absolute home directory so evidence and telemetry
// carry no username or machine-specific home path (spec 07 R5.2). The user's
// home root (`/home/<u>`, `/Users/<u>`, `/root`) collapses to `~`, keeping the
// path meaningful without leaking identity. Non-home absolute paths are left
// intact. The replacement is deterministic — it reads the input, never the host.
var homePathPattern = regexp.MustCompile(`/home/[^/\s:"]+|/Users/[^/\s:"]+|/root`)

type Redactor struct{ secrets []string }

func NewRedactor(secrets []string) Redactor {
	clean := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		if len(secret) >= 4 {
			clean = append(clean, secret)
		}
	}
	return Redactor{secrets: clean}
}

func (r Redactor) String(value string) string {
	for _, secret := range r.secrets {
		value = strings.ReplaceAll(value, secret, Redacted)
	}
	for _, pattern := range credentialPatterns {
		value = pattern.ReplaceAllString(value, `${1}`+Redacted)
	}
	value = homePathPattern.ReplaceAllString(value, "~")
	return value
}

type RedactingWriter struct {
	dst io.Writer
	r   Redactor
	buf strings.Builder
}

func NewRedactingWriter(dst io.Writer, secrets []string) *RedactingWriter {
	return &RedactingWriter{dst: dst, r: NewRedactor(secrets)}
}

func (w *RedactingWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }

func (w *RedactingWriter) Close() error {
	_, err := io.WriteString(w.dst, w.r.String(w.buf.String()))
	w.buf.Reset()
	return err
}

func environmentSecrets(env []string) []string {
	var secrets []string
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		upper := strings.ToUpper(key)
		if ok && value != "" && (strings.Contains(upper, "TOKEN") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "API_KEY")) {
			secrets = append(secrets, value)
		}
	}
	return secrets
}
