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
