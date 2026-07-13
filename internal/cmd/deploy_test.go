package cmd

import (
	"sync"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// freezeReleaseForTest freezes a candidate and returns its release id.
func freezeReleaseForTest(t *testing.T, root string) string {
	t.Helper()
	if err := Run(root, "release", []string{"candidate", "demo"}, map[string]string{
		"artifact-digest": "sha256:abc", "sbom-ref": "sbom://demo", "provenance-ref": "prov://demo",
	}); err != nil {
		t.Fatalf("freeze candidate: %v", err)
	}
	releases, err := core.ReadReleases(core.ReleaseLedgerPath(root, "demo"))
	if err != nil || len(releases) != 1 {
		t.Fatalf("read releases: %v %d", err, len(releases))
	}
	return releases[0].ReleaseID
}

// TestDeployAppend pins `specd deploy` (spec 08 R6.2): each attempt appends one
// record to deployments.jsonl under the spec lock, with a monotonic attempt
// number per deployment, and concurrent attempts never duplicate a number.
func TestDeployAppend(t *testing.T) {
	root := newCriterionSpec(t)
	rel := freezeReleaseForTest(t, root)
	flags := func() map[string]string {
		return map[string]string{
			"release":     rel,
			"environment": "staging",
			"adapter":     "shell",
			"authority":   "ci",
		}
	}

	if err := Run(root, "deploy", []string{"demo"}, flags()); err != nil {
		t.Fatalf("deploy 1: %v", err)
	}
	if err := Run(root, "deploy", []string{"demo"}, flags()); err != nil {
		t.Fatalf("deploy 2: %v", err)
	}

	records, err := core.ReadDeployments(core.DeploymentLedgerPath(root, "demo"))
	if err != nil {
		t.Fatalf("read deployments: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected two attempts, got %d", len(records))
	}
	if records[0].Attempt != 1 || records[1].Attempt != 2 {
		t.Fatalf("attempts not monotonic: %d, %d", records[0].Attempt, records[1].Attempt)
	}
	if records[1].ReleaseID != rel || records[1].Environment != core.EnvironmentStaging {
		t.Fatalf("attempt fields not recorded: %+v", records[1])
	}
	if records[1].ArtifactDigest != "sha256:abc" {
		t.Fatalf("deployment did not bind the frozen artifact digest: %+v", records[1])
	}
	for _, r := range records {
		if err := core.ValidateDeployment(r); err != nil {
			t.Fatalf("recorded attempt invalid: %v", err)
		}
	}
}

// TestDeployConcurrentNoDuplicate pins that racing attempts under the spec lock
// yield distinct monotonic numbers, never a duplicate (spec 08 R6.2).
func TestDeployConcurrentNoDuplicate(t *testing.T) {
	root := newCriterionSpec(t)
	rel := freezeReleaseForTest(t, root)
	const n = 6
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = Run(root, "deploy", []string{"demo"}, map[string]string{
				"release": rel, "environment": "staging", "adapter": "shell", "authority": "ci",
			})
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("concurrent deploy: %v", err)
		}
	}
	records, err := core.ReadDeployments(core.DeploymentLedgerPath(root, "demo"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	seen := map[int]bool{}
	for _, r := range records {
		if seen[r.Attempt] {
			t.Fatalf("duplicate attempt %d", r.Attempt)
		}
		seen[r.Attempt] = true
	}
	if len(records) != n {
		t.Fatalf("expected %d attempts, got %d", n, len(records))
	}
}

// TestDeployFailsClosed pins the usage guard: a missing required field is a
// fail-closed rejection, never a partial ledger record.
func TestDeployFailsClosed(t *testing.T) {
	root := newCriterionSpec(t)
	rel := freezeReleaseForTest(t, root)
	if err := Run(root, "deploy", []string{"demo"}, map[string]string{"environment": "staging", "adapter": "shell", "authority": "ci"}); err == nil {
		t.Fatal("missing --release must fail closed")
	}
	if err := Run(root, "deploy", []string{"demo"}, map[string]string{"release": rel, "environment": "mars", "adapter": "shell", "authority": "ci"}); err == nil {
		t.Fatal("unknown environment must fail closed")
	}
	if err := Run(root, "deploy", []string{"demo"}, map[string]string{"release": "nonexistent", "environment": "staging", "adapter": "shell", "authority": "ci"}); err == nil {
		t.Fatal("unknown release must fail closed")
	}
}

func TestPromote(t *testing.T) {
	deployment := core.DeploymentV1{Schema: core.DeploymentSchemaV1, DeploymentID: "dep-1", Attempt: 1,
		ReleaseID: "rel-1", GitHead: "head", ArtifactDigest: "sha256:a", Environment: core.EnvironmentProduction,
		Status: core.StatusObserving, Strategy: "canary", Population: "10%", Window: "10m", Adapter: "deploy/v1",
		Authority: "release-manager", Actor: "operator", IdempotencyKey: "key", StartedAt: "2026-07-13T11:50:00Z"}
	promotion, err := promotionRecord(deployment, "baseline:prod", []string{"obs:health", "obs:latency"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if promotion.Status != core.StatusHealthy || promotion.Promotion == nil || len(promotion.Promotion.EvidenceRefs) != 2 {
		t.Fatalf("promotion audit missing: %+v", promotion)
	}
	if _, err := promotionRecord(deployment, "baseline:prod", nil, ""); err == nil {
		t.Fatal("promotion without evidence accepted")
	}
	failed := deployment
	failed.Status = core.StatusFailed
	if _, err := promotionRecord(failed, "", nil, "EX-7"); err != nil {
		t.Fatalf("governed exception rejected: %v", err)
	}
	if _, err := promotionRecord(failed, "", nil, ""); err == nil {
		t.Fatal("failed canary accepted without exception or rollback")
	}
}
