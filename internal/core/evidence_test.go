package core

import (
	"strings"
	"testing"
)

func TestTruncateEvidenceOutputReportsLimit(t *testing.T) {
	output := strings.Repeat("x", EvidenceOutputLimit+200)
	truncated := TruncateEvidenceOutput(output)
	if !strings.Contains(truncated, "output truncated to 65536 of 65736 bytes") {
		t.Fatalf("missing truncation marker: %q", truncated[len(truncated)-80:])
	}
	if len(truncated) >= len(output) {
		t.Fatalf("expected shortened output, got %d >= %d", len(truncated), len(output))
	}
}
