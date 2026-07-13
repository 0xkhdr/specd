package gates

import (
	"fmt"
	"sort"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// DeliveryInput is the full on-disk evidence a delivery verdict is computed
// from: the closed environment policy, the immutable release candidate, the
// deployment record, its health observations, and an explicit clock. Now is a
// field (never time.Now()) so the verdict is a pure function of its inputs —
// same inputs always yield the same findings (spec 08 R7.1).
type DeliveryInput struct {
	Policy       core.EnvironmentV1
	Release      core.ReleaseCandidateV1
	Deployment   core.DeploymentV1
	Observations []core.HealthObservationV1
	Now          time.Time
}

type CanaryVerdictResult struct {
	Healthy      bool
	EvidenceRefs []string
	Findings     []string
}

func CanaryVerdict(in DeliveryInput, allowedSources []string) CanaryVerdictResult {
	result := CanaryVerdictResult{}
	if err := core.ValidateCanaryWindow(in.Deployment, in.Observations, in.Now); err != nil {
		result.Findings = append(result.Findings, err.Error())
	}
	allowed := make(map[string]bool, len(allowedSources))
	for _, source := range allowedSources {
		allowed[source] = true
	}
	byCriterion := make(map[string]core.HealthObservationV1)
	for _, observation := range in.Observations {
		if !allowed[observation.Source] {
			result.Findings = append(result.Findings, "criterion "+observation.CriterionID+" source is not allowlisted")
			continue
		}
		observed, err := time.Parse(time.RFC3339, observation.Freshness.ObservedAt)
		maxAge, maxErr := time.ParseDuration(observation.Freshness.MaxAge)
		policyAge, policyErr := time.ParseDuration(in.Policy.Freshness)
		if err != nil || maxErr != nil || policyErr != nil || observed.After(in.Now) || in.Now.Sub(observed) > maxAge || in.Now.Sub(observed) > policyAge {
			result.Findings = append(result.Findings, "criterion "+observation.CriterionID+" observation is stale or malformed")
			continue
		}
		if observation.Observation != "pass" {
			result.Findings = append(result.Findings, "criterion "+observation.CriterionID+" failed")
			continue
		}
		byCriterion[observation.CriterionID] = observation
	}
	criteria := append([]string(nil), in.Policy.HealthCriteria...)
	sort.Strings(criteria)
	for _, criterion := range criteria {
		o, ok := byCriterion[criterion]
		if !ok {
			result.Findings = append(result.Findings, "criterion "+criterion+" missing passing observation")
			continue
		}
		result.EvidenceRefs = append(result.EvidenceRefs, fmt.Sprintf("health:%s:%s:%s", o.DeploymentID, o.CriterionID, o.Freshness.ObservedAt))
	}
	sort.Strings(result.EvidenceRefs)
	sort.Strings(result.Findings)
	result.Healthy = len(result.Findings) == 0
	return result
}

// delivery is the pure, offline delivery gate. It returns findings in a fixed
// code order (no map iteration, no wall clock), so the verdict is deterministic.
// Production carries the full control set (R7.2); an artifact swapped after the
// candidate was frozen fails the digest check (R7.3); lower environments may opt
// out of the production-only controls without weakening any check here.
func delivery(in DeliveryInput) []Finding {
	var findings []Finding
	fail := func(msg string) {
		findings = append(findings, Finding{Gate: "delivery", Severity: Error, Message: msg})
	}

	if !isKnownEnvironment(in.Deployment.Environment) {
		fail("unknown environment " + string(in.Deployment.Environment))
	}
	if in.Policy.Name != in.Deployment.Environment {
		fail("deployment environment " + string(in.Deployment.Environment) + " does not match policy " + string(in.Policy.Name))
	}
	if in.Deployment.ReleaseID != in.Release.ReleaseID {
		fail("release identity mismatch: deployment " + in.Deployment.ReleaseID + " candidate " + in.Release.ReleaseID)
	}
	// R7.3: the deployed artifact must be the exact digest the candidate froze.
	if in.Deployment.ArtifactDigest != in.Release.ArtifactDigest {
		fail("artifact digest mismatch: deployment " + in.Deployment.ArtifactDigest + " candidate " + in.Release.ArtifactDigest)
	}
	if in.Deployment.GitHead != in.Release.GitHead {
		fail("git HEAD mismatch: deployment " + in.Deployment.GitHead + " candidate " + in.Release.GitHead)
	}

	// R7.2: production requires the full control set; it is not relaxable by task
	// text, prompt, or adapter response — this is a pure policy check on-disk.
	if in.Deployment.Environment == core.EnvironmentProduction {
		if in.Deployment.Adapter == "" {
			fail("production deployment missing adapter")
		}
		if in.Deployment.Authority == "" {
			fail("production deployment missing authority")
		}
		if in.Deployment.ArtifactDigest == "" || in.Release.ArtifactDigest == "" {
			fail("production deployment missing artifact identity")
		}
		if in.Policy.RollbackTarget == "" {
			fail("production policy missing rollback target")
		}
		if !hasFreshObservation(in) {
			fail("production deployment missing fresh matching health observation")
		}
	}

	return findings
}

func isKnownEnvironment(e core.EnvironmentName) bool {
	switch e {
	case core.EnvironmentDevelopment, core.EnvironmentStaging, core.EnvironmentProduction:
		return true
	default:
		return false
	}
}

// hasFreshObservation reports whether some observation matches the deployment's
// exact release/artifact/environment and is fresh at in.Now (never healthy by a
// timeout default — a zero clock or a stale/mismatched observation is not fresh).
func hasFreshObservation(in DeliveryInput) bool {
	if in.Now.IsZero() {
		return false
	}
	for _, o := range in.Observations {
		if o.ReleaseIdentity.ReleaseID != in.Release.ReleaseID ||
			o.ReleaseIdentity.ArtifactDigest != in.Release.ArtifactDigest ||
			o.ReleaseIdentity.Environment != in.Deployment.Environment {
			continue
		}
		observed, err := time.Parse(time.RFC3339, o.Freshness.ObservedAt)
		if err != nil {
			continue
		}
		maxAge, err := time.ParseDuration(o.Freshness.MaxAge)
		if err != nil {
			continue
		}
		if observed.After(in.Now) || in.Now.Sub(observed) > maxAge {
			continue
		}
		return true
	}
	return false
}
