package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// runDeploy implements `specd deploy <spec>` (spec 08 R6.2). It records one
// deployment attempt against a previously frozen release candidate: it appends a
// monotonic attempt to deployments.jsonl under the spec lock and drives no
// external system. The attempt binds the frozen candidate's git HEAD and
// artifact digest, so a deployment can never claim an artifact the release did
// not freeze.
func runDeploy(root string, args []string, flags map[string]string) error {
	if len(args) < 1 {
		return usageError("deploy")
	}
	slug := args[0]

	releaseID := strings.TrimSpace(flags["release"])
	environment := strings.TrimSpace(flags["environment"])
	adapter := strings.TrimSpace(flags["adapter"])
	authority := strings.TrimSpace(flags["authority"])
	for name, value := range map[string]string{"release": releaseID, "environment": environment, "adapter": adapter, "authority": authority} {
		if value == "" {
			return fmt.Errorf("%w: deploy requires --%s", ErrUsage, name)
		}
	}

	releases, err := core.ReadReleases(core.ReleaseLedgerPath(root, slug))
	if err != nil {
		return fmt.Errorf("%w: read releases: %v", ErrUsage, err)
	}
	var candidate *core.ReleaseCandidateV1
	for i := range releases {
		if releases[i].ReleaseID == releaseID {
			candidate = &releases[i]
			break
		}
	}
	if candidate == nil {
		return fmt.Errorf("%w: no frozen release candidate %q for %s; run `specd release candidate` first", ErrUsage, releaseID, slug)
	}

	env := core.EnvironmentName(environment)
	deployment := core.DeploymentV1{
		Schema:         core.DeploymentSchemaV1,
		DeploymentID:   deploymentID(slug, releaseID, environment),
		ReleaseID:      candidate.ReleaseID,
		GitHead:        candidate.GitHead,
		ArtifactDigest: candidate.ArtifactDigest,
		Environment:    env,
		Status:         core.StatusRequested,
		Strategy:       orDefault(flags["strategy"], "all-at-once"),
		Population:     orDefault(flags["population"], "all"),
		Window:         orDefault(flags["window"], "none"),
		Adapter:        adapter,
		Authority:      authority,
		IdempotencyKey: orDefault(flags["idempotency-key"], deploymentID(slug, releaseID, environment)),
	}

	attempt, err := core.AppendDeploymentAttempt(root, slug, deployment)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUsage, err)
	}
	fmt.Printf("deployment %s attempt %d recorded for %s (%s)\n", attempt.DeploymentID, attempt.Attempt, slug, env)
	return nil
}

func promotionRecord(deployment core.DeploymentV1, baseline string, evidenceRefs []string, exceptionRef string) (core.DeploymentV1, error) {
	if deployment.Status == core.StatusFailed {
		if strings.TrimSpace(exceptionRef) == "" {
			return core.DeploymentV1{}, fmt.Errorf("failed canary requires governed exception or rollback")
		}
		deployment.ExceptionRef = exceptionRef
		return deployment, nil
	}
	if deployment.Status != core.StatusObserving {
		return core.DeploymentV1{}, fmt.Errorf("only observing canary may promote")
	}
	if strings.TrimSpace(baseline) == "" || len(evidenceRefs) == 0 {
		return core.DeploymentV1{}, fmt.Errorf("promotion requires baseline and evidence references")
	}
	refs := append([]string(nil), evidenceRefs...)
	for _, ref := range refs {
		if strings.TrimSpace(ref) == "" {
			return core.DeploymentV1{}, fmt.Errorf("promotion evidence reference is empty")
		}
	}
	sort.Strings(refs)
	deployment.Status = core.StatusHealthy
	deployment.Promotion = &core.PromotionV1{Baseline: baseline, EvidenceRefs: refs}
	return deployment, nil
}

// deploymentID is the deterministic identity of a (spec, release, environment)
// deployment. Retries of the same release into the same environment share this
// id and accrue monotonic attempts; a different environment is a distinct
// deployment starting at attempt 1.
func deploymentID(slug, releaseID, environment string) string {
	return core.Digest([]byte(slug + "\x00" + releaseID + "\x00" + environment))[:12]
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
