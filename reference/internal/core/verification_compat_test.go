package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVerificationRecordCompat(t *testing.T) {
	// A record written before ChangedFiles/Coverage existed must still parse,
	// leaving the new fields at their zero values.
	legacy := `{
      "command": "go test ./...",
      "exitCode": 0,
      "verified": true,
      "timedOut": false,
      "stdoutTail": "ok",
      "stderrTail": "",
      "durationMs": 12,
      "ranAt": "2026-01-01T00:00:00Z",
      "gitHead": "abc123"
    }`
	var rec VerificationRecord
	if err := json.Unmarshal([]byte(legacy), &rec); err != nil {
		t.Fatalf("legacy record failed to parse: %v", err)
	}
	if !rec.Verified || rec.Command != "go test ./..." {
		t.Errorf("legacy fields lost: %+v", rec)
	}
	if rec.ChangedFiles != nil || rec.Coverage != "" {
		t.Errorf("new fields should be zero for legacy record: %+v", rec)
	}

	// Empty new fields are omitted from output (byte-compat with old writers).
	out, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "changedFiles") || strings.Contains(string(out), "coverage") {
		t.Errorf("empty new fields must be omitempty, got: %s", out)
	}

	// Populated fields round-trip.
	rec.ChangedFiles = []string{"a.go", "b.go"}
	rec.Coverage = "84.2%"
	out, _ = json.Marshal(rec)
	var back VerificationRecord
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if strings.Join(back.ChangedFiles, ",") != "a.go,b.go" || back.Coverage != "84.2%" {
		t.Errorf("round-trip lost data: %+v", back)
	}
}
