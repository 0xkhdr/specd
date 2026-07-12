package cmd

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestReleaseCandidate pins `specd release candidate` (spec 08 R6.1): it freezes
// an immutable, reproducible candidate identity into releases.jsonl, builds and
// uploads nothing, and is idempotent for identical inputs.
func TestReleaseCandidate(t *testing.T) {
	root := newCriterionSpec(t) // approved spec ⇒ revision > 0

	flags := map[string]string{
		"artifact-digest": "sha256:abc",
		"sbom-ref":        "sbom://demo",
		"provenance-ref":  "prov://demo",
	}
	if err := Run(root, "release", []string{"candidate", "demo"}, flags); err != nil {
		t.Fatalf("release candidate: %v", err)
	}

	releases, err := core.ReadReleases(core.ReleaseLedgerPath(root, "demo"))
	if err != nil {
		t.Fatalf("read releases: %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("expected one frozen candidate, got %d", len(releases))
	}
	r := releases[0]
	if r.ReleaseID == "" || r.SpecID != "demo" || r.ArtifactDigest != "sha256:abc" ||
		r.SBOMRef != "sbom://demo" || r.ProvenanceRef != "prov://demo" ||
		r.BootstrapDigest == "" || r.TaskEvidenceSetDigest == "" || r.GitHead == "" {
		t.Fatalf("candidate missing identity fields: %+v", r)
	}
	if err := core.ValidateReleaseCandidate(r); err != nil {
		t.Fatalf("frozen candidate invalid: %v", err)
	}

	// Reproducible + immutable: re-freezing the same inputs adds no record.
	if err := Run(root, "release", []string{"candidate", "demo"}, flags); err != nil {
		t.Fatalf("re-release: %v", err)
	}
	again, _ := core.ReadReleases(core.ReleaseLedgerPath(root, "demo"))
	if len(again) != 1 || again[0].ReleaseID != r.ReleaseID {
		t.Fatalf("re-freeze must be idempotent, got %d records", len(again))
	}
}

// TestReleaseCandidateFailsClosed pins the usage guards: missing subcommand or a
// missing required reference is a fail-closed rejection (exit 2), never a
// partial candidate.
func TestReleaseCandidateFailsClosed(t *testing.T) {
	root := newCriterionSpec(t)
	if err := Run(root, "release", []string{"candidate", "demo"}, map[string]string{"sbom-ref": "s", "provenance-ref": "p"}); err == nil {
		t.Fatal("missing artifact-digest must fail closed")
	}
	if err := Run(root, "release", []string{"demo"}, nil); err == nil {
		t.Fatal("missing candidate subcommand must fail closed")
	}
	if _, err := core.ReadReleases(core.ReleaseLedgerPath(root, "demo")); err != nil {
		t.Fatalf("no ledger should have been written: %v", err)
	}
}
