package cmd

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/orchestration"
)

func TestReportMissionAuditReferenceIsPolicyKeyedAndSanitized(t *testing.T) {
	e := orchestration.ACPEvent{AuditID: 3, AuditKind: "diff", RunID: "run-1", MissionID: "mission-1", TaskID: "T1", PolicyDigest: "sha256:abc", Payload: "internal detail"}
	got := acpReference(e)
	for _, want := range []string{"run=run-1", "mission=mission-1", "task=T1", "policy=sha256:abc", "stage=diff", "audit_id=3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("reference %q missing %q", got, want)
		}
	}
	if strings.Contains(got, e.Payload) {
		t.Fatal("raw payload leaked")
	}
}
