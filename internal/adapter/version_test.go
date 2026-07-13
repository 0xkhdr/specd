package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchemaVersionNegotiationPolicyPublished(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "adapter-contract.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	for _, required := range []string{
		"Adapter schema compatibility and negotiation",
		"independent of CLI and on-disk state schemas",
		"exact offered version",
		"no implicit downgrade",
		"breaking change",
		"additive change",
	} {
		if !strings.Contains(doc, required) {
			t.Errorf("version policy missing %q", required)
		}
	}
}

func TestSchemaVersionFailsClosedWithoutExactOffer(t *testing.T) {
	req := sampleRequest()
	req.SchemaVersion = "adapter/v2"
	if err := req.Validate(); err == nil {
		t.Fatal("unsupported adapter version accepted")
	}
	feedback := ReleaseFeedbackV1{SchemaVersion: "release_feedback/v2", SourceSpec: "a", SuccessorSpec: "b", ReleaseID: "r", Environment: "production", GitHead: "h", ObservedAt: "2026-07-13T10:00:00Z", EvidenceRefs: []string{"artifact://e/1"}}
	if _, err := FeedbackRequest(feedback, "req", "corr", "adapter"); err == nil {
		t.Fatal("unsupported payload version accepted")
	}
}
