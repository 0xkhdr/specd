package adapter

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestFeedbackRoundTripPreservesReleaseProvenance(t *testing.T) {
	feedback := ReleaseFeedbackV1{
		SchemaVersion: FeedbackSchemaV1,
		SourceSpec:    "checkout",
		SuccessorSpec: "checkout-fix",
		ReleaseID:     "rel-7",
		Environment:   "production",
		GitHead:       "0123456789abcdef0123456789abcdef01234567",
		ObservedAt:    "2026-07-13T10:00:00Z",
		EvidenceRefs:  []string{"artifact://health/criterion-4"},
	}
	req, err := FeedbackRequest(feedback, "req-1", "corr-1", "runtime-feedback")
	if err != nil {
		t.Fatal(err)
	}
	got, err := FeedbackFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if got.ReleaseID != feedback.ReleaseID || got.SuccessorSpec != feedback.SuccessorSpec || got.EvidenceRefs[0] != feedback.EvidenceRefs[0] {
		t.Fatalf("feedback provenance lost: %+v", got)
	}
	if req.Subject.ReleaseID != feedback.ReleaseID || req.Subject.Environment != feedback.Environment || req.Subject.GitHead != feedback.GitHead || req.Subject.SpecSlug != feedback.SourceSpec {
		t.Fatalf("release identity not pinned in envelope: %+v", req.Subject)
	}
}

func TestFeedbackCommitMustResolveAtImportBoundary(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "test@example.test"}, {"config", "user.name", "Test"}, {"commit", "--allow-empty", "-m", "fixture"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	out, _ := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	f := ReleaseFeedbackV1{SchemaVersion: FeedbackSchemaV1, SourceSpec: "source", SuccessorSpec: "repair", ReleaseID: "r1", Environment: "production", GitHead: strings.TrimSpace(string(out)), ObservedAt: "2026-01-01T00:00:00Z", EvidenceRefs: []string{"artifact://run/1"}}
	if err := ValidateFeedbackCommit(root, f); err != nil {
		t.Fatal(err)
	}
	f.GitHead = strings.Repeat("a", 40)
	if err := ValidateFeedbackCommit(root, f); err == nil {
		t.Fatal("unresolvable feedback commit accepted")
	}
}

func TestFeedbackRejectsIdentityMismatchAndUnknownPayloadField(t *testing.T) {
	feedback := ReleaseFeedbackV1{SchemaVersion: FeedbackSchemaV1, SourceSpec: "checkout", SuccessorSpec: "checkout-fix", ReleaseID: "rel-7", Environment: "production", GitHead: "head", ObservedAt: "2026-07-13T10:00:00Z", EvidenceRefs: []string{"artifact://health/4"}}
	req, err := FeedbackRequest(feedback, "req-1", "corr-1", "runtime-feedback")
	if err != nil {
		t.Fatal(err)
	}
	req.Subject.ReleaseID = "rel-other"
	if _, err := FeedbackFromRequest(req); err == nil {
		t.Fatal("release mismatch accepted")
	}
	req.Subject.ReleaseID = feedback.ReleaseID
	var payload map[string]any
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload["instruction"] = "reopen completed spec"
	req.Payload, _ = json.Marshal(payload)
	if _, err := FeedbackFromRequest(req); err == nil {
		t.Fatal("unknown feedback field accepted")
	}
}

func TestFeedbackRejectsMissingProvenance(t *testing.T) {
	feedback := ReleaseFeedbackV1{SchemaVersion: FeedbackSchemaV1, SourceSpec: "checkout", SuccessorSpec: "checkout-fix", ReleaseID: "rel-7", Environment: "production", GitHead: "head", ObservedAt: "2026-07-13T10:00:00Z"}
	if _, err := FeedbackRequest(feedback, "req-1", "corr-1", "runtime-feedback"); err == nil {
		t.Fatal("feedback without evidence accepted")
	}
}
