package core

import (
	"strings"
	"testing"
)

// TestParseErrorPayloadValid accepts a well-formed payload.
func TestParseErrorPayloadValid(t *testing.T) {
	p, err := ParseErrorPayload([]byte(`{
		"service": "billing", "environment": "production", "severity": "error",
		"message": "nil pointer", "frames": [{"file": "internal/billing/charge.go", "line": 42}]
	}`))
	if err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	if p.Service != "billing" || len(p.Frames) != 1 {
		t.Fatalf("unexpected payload: %#v", p)
	}
}

// TestParseErrorPayloadHostile rejects the adversarial payload matrix: unknown
// fields, missing message, bad severity, absolute/traversing frame paths, and
// negative lines (V9 §5 hostile-input suite).
func TestParseErrorPayloadHostile(t *testing.T) {
	cases := map[string]string{
		"unknown field":   `{"severity":"error","message":"x","evil":1}`,
		"missing message": `{"severity":"error","message":"  "}`,
		"bad severity":    `{"severity":"meltdown","message":"x"}`,
		"absolute frame":  `{"severity":"error","message":"x","frames":[{"file":"/etc/passwd"}]}`,
		"traversal frame": `{"severity":"error","message":"x","frames":[{"file":"../../etc/passwd"}]}`,
		"negative line":   `{"severity":"error","message":"x","frames":[{"file":"a.go","line":-1}]}`,
		"not json":        `nope`,
	}
	for name, body := range cases {
		if _, err := ParseErrorPayload([]byte(body)); err == nil {
			t.Errorf("%s: expected rejection, got nil", name)
		}
	}
}

// TestSeverityImpact is the deterministic severity→impact mapping table.
func TestSeverityImpact(t *testing.T) {
	cases := map[string]string{
		"critical": "critical", "fatal": "critical",
		"error":   "high",
		"warning": "medium", "warn": "medium",
		"info": "low", "debug": "low",
		"ERROR": "high", // case-insensitive
		"bogus": "",
	}
	for sev, want := range cases {
		if got := SeverityImpact(sev); got != want {
			t.Errorf("SeverityImpact(%q) = %q, want %q", sev, got, want)
		}
	}
}

// TestValidateFramePathClean allows repo-relative paths and normalizes them.
func TestValidateFramePathClean(t *testing.T) {
	for _, ok := range []string{"", "a/b.go", "internal/x/y.go", "./a.go"} {
		if err := validateFramePath(ok); err != nil {
			t.Errorf("validateFramePath(%q) = %v, want nil", ok, err)
		}
	}
}

// TestRenderObserveMidreq renders a deterministic, evidenced entry body.
func TestRenderObserveMidreq(t *testing.T) {
	p := ErrorPayload{Service: "billing", Environment: "production", Severity: "error", Message: "boom", Fingerprint: "abc"}
	c := Correlation{Spec: "billing-svc", Impact: "high", Confidence: "high", MatchedFiles: []string{"a.go"}, Facts: []string{"fact one"}}
	body := RenderObserveMidreq(p, c)
	for _, want := range []string{"production error (observe)", "impact high", "confidence:** high", "fact one", `"boom"`} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered body missing %q:\n%s", want, body)
		}
	}
}
