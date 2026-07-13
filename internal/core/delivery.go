package core

import (
	"fmt"
	"strings"
	"time"
)

const (
	ReleaseCandidateSchemaV1  = "ReleaseCandidateV1"
	EnvironmentSchemaV1       = "EnvironmentV1"
	DeploymentSchemaV1        = "DeploymentV1"
	HealthObservationSchemaV1 = "HealthObservationV1"
	RollbackSchemaV1          = "RollbackV1"
	RollbackSchemaV2          = "RollbackV2"
)

type EnvironmentName string

const (
	EnvironmentDevelopment EnvironmentName = "development"
	EnvironmentStaging     EnvironmentName = "staging"
	EnvironmentProduction  EnvironmentName = "production"
)

func (e EnvironmentName) valid() bool {
	switch e {
	case EnvironmentDevelopment, EnvironmentStaging, EnvironmentProduction:
		return true
	default:
		return false
	}
}

type DeploymentStatus string

const (
	StatusRequested   DeploymentStatus = "requested"
	StatusStarted     DeploymentStatus = "started"
	StatusObserving   DeploymentStatus = "observing"
	StatusHealthy     DeploymentStatus = "healthy"
	StatusFailed      DeploymentStatus = "failed"
	StatusRollingBack DeploymentStatus = "rolling_back"
	StatusRolledBack  DeploymentStatus = "rolled_back"
)

func (s DeploymentStatus) valid() bool {
	switch s {
	case StatusRequested, StatusStarted, StatusObserving, StatusHealthy, StatusFailed, StatusRollingBack, StatusRolledBack:
		return true
	default:
		return false
	}
}

type ReleaseCandidateV1 struct {
	Schema                string `json:"schema"`
	ReleaseID             string `json:"release_id"`
	SpecID                string `json:"spec_id"`
	SpecRevision          int    `json:"spec_revision"`
	GitHead               string `json:"git_head"`
	TaskEvidenceSetDigest string `json:"task_evidence_set_digest"`
	ArtifactDigest        string `json:"artifact_digest"`
	SBOMRef               string `json:"sbom_ref"`
	ProvenanceRef         string `json:"provenance_ref"`
	BootstrapDigest       string `json:"bootstrap_digest"`
	StateSchema           string `json:"state_schema"`
	CreatedAt             string `json:"created_at"`
}

type EnvironmentV1 struct {
	Schema            string          `json:"schema"`
	Name              EnvironmentName `json:"name"`
	Strategy          string          `json:"strategy"`
	RequiredApprover  string          `json:"required_approver,omitempty"`
	RequiredAuthority string          `json:"required_authority,omitempty"`
	HealthCriteria    []string        `json:"health_criteria"`
	ObservationWindow string          `json:"observation_window"`
	Freshness         string          `json:"freshness"`
	RollbackTarget    string          `json:"rollback_target"`
}

type DeploymentV1 struct {
	Schema             string             `json:"schema"`
	DeploymentID       string             `json:"deployment_id"`
	Attempt            int                `json:"attempt"`
	ReleaseID          string             `json:"release_id"`
	GitHead            string             `json:"git_head"`
	ArtifactDigest     string             `json:"artifact_digest"`
	Environment        EnvironmentName    `json:"environment"`
	Status             DeploymentStatus   `json:"status"`
	Strategy           string             `json:"strategy"`
	Population         string             `json:"population"`
	Window             string             `json:"window"`
	Adapter            string             `json:"adapter"`
	Authority          string             `json:"authority"`
	Actor              string             `json:"actor"`
	IdempotencyKey     string             `json:"idempotency_key"`
	StartedAt          string             `json:"started_at"`
	FinishedAt         string             `json:"finished_at,omitempty"`
	TelemetrySource    string             `json:"telemetry_source"`
	EvidenceRef        string             `json:"evidence_ref"`
	AttestationRef     string             `json:"attestation_ref"`
	AdapterTrustSource AdapterTrustSource `json:"adapter_trust_source,omitempty"`
	AdapterMessage     string             `json:"adapter_message,omitempty"`
	Promotion          *PromotionV1       `json:"promotion,omitempty"`
	ExceptionRef       string             `json:"exception_ref,omitempty"`
}

type ObservationFreshness struct {
	ObservedAt      string `json:"observed_at"`
	MaxAge          string `json:"max_age"`
	WindowStartedAt string `json:"window_started_at,omitempty"`
}

// PromotionV1 preserves exact health evidence and comparison baseline used by
// a promotion. References remain external; no raw telemetry enters the ledger.
type PromotionV1 struct {
	Baseline     string   `json:"baseline"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type DeliveryIdentity struct {
	ReleaseID      string          `json:"release_id"`
	ArtifactDigest string          `json:"artifact_digest"`
	Environment    EnvironmentName `json:"environment"`
}

type HealthObservationV1 struct {
	Schema          string               `json:"schema"`
	DeploymentID    string               `json:"deployment_id"`
	CriterionID     string               `json:"criterion_id"`
	HealthCheck     string               `json:"health_check"`
	Threshold       string               `json:"threshold"`
	Observation     string               `json:"observation"`
	Freshness       ObservationFreshness `json:"freshness"`
	ReleaseIdentity DeliveryIdentity     `json:"release_identity"`
	Source          string               `json:"source"`
}

type RollbackHealth struct {
	CriterionID string `json:"criterion_id"`
	Observation string `json:"observation"`
	ObservedAt  string `json:"observed_at"`
}

type RollbackV1 struct {
	Schema             string         `json:"schema"`
	DeploymentID       string         `json:"deployment_id"`
	FailedReleaseID    string         `json:"failed_release_id"`
	RollbackTarget     string         `json:"rollback_target"`
	Reason             string         `json:"reason"`
	Adapter            string         `json:"adapter"`
	AdapterIdentity    string         `json:"adapter_identity"`
	ActionResult       string         `json:"action_result"`
	PostRollbackHealth RollbackHealth `json:"post_rollback_health"`
	CapabilityClass    string         `json:"capability_class"`
	HumanRequired      bool           `json:"human_required"`
}

const (
	RollbackCapabilityAutomatic     = "automatic"
	RollbackCapabilityHumanRequired = "human_required"
)

func ValidateReleaseCandidate(r ReleaseCandidateV1) error {
	if r.Schema != ReleaseCandidateSchemaV1 {
		return fmt.Errorf("unknown release schema %q", r.Schema)
	}
	if missing := firstEmpty(
		"release_id", r.ReleaseID, "spec_id", r.SpecID, "git_head", r.GitHead,
		"task_evidence_set_digest", r.TaskEvidenceSetDigest, "artifact_digest", r.ArtifactDigest,
		"sbom_ref", r.SBOMRef, "provenance_ref", r.ProvenanceRef, "bootstrap_digest", r.BootstrapDigest,
		"state_schema", r.StateSchema, "created_at", r.CreatedAt,
	); missing != "" {
		return fmt.Errorf("release candidate missing %s", missing)
	}
	if r.SpecRevision < 1 {
		return fmt.Errorf("release candidate spec_revision must be positive")
	}
	if _, err := parseRFC3339("created_at", r.CreatedAt); err != nil {
		return err
	}
	return nil
}

func ValidateEnvironment(e EnvironmentV1) error {
	if e.Schema != EnvironmentSchemaV1 {
		return fmt.Errorf("unknown environment schema %q", e.Schema)
	}
	if !e.Name.valid() {
		return fmt.Errorf("unknown environment %q", e.Name)
	}
	if missing := firstEmpty("strategy", e.Strategy, "observation_window", e.ObservationWindow, "freshness", e.Freshness, "rollback_target", e.RollbackTarget); missing != "" {
		return fmt.Errorf("environment missing %s", missing)
	}
	if len(e.HealthCriteria) == 0 {
		return fmt.Errorf("environment missing health_criteria")
	}
	if _, err := time.ParseDuration(e.ObservationWindow); err != nil {
		return fmt.Errorf("invalid observation_window: %w", err)
	}
	if _, err := time.ParseDuration(e.Freshness); err != nil {
		return fmt.Errorf("invalid freshness: %w", err)
	}
	return nil
}

func ValidateDeployment(d DeploymentV1) error {
	if d.Schema != DeploymentSchemaV1 {
		return fmt.Errorf("unknown deployment schema %q", d.Schema)
	}
	if !d.Environment.valid() {
		return fmt.Errorf("unknown environment %q", d.Environment)
	}
	if !d.Status.valid() {
		return fmt.Errorf("unknown deployment status %q", d.Status)
	}
	if missing := firstEmpty(
		"deployment_id", d.DeploymentID, "release_id", d.ReleaseID, "git_head", d.GitHead,
		"artifact_digest", d.ArtifactDigest, "strategy", d.Strategy, "population", d.Population,
		"window", d.Window, "adapter", d.Adapter, "authority", d.Authority, "actor", d.Actor,
		"idempotency_key", d.IdempotencyKey, "started_at", d.StartedAt,
	); missing != "" {
		return fmt.Errorf("deployment missing %s", missing)
	}
	if d.Attempt < 1 {
		return fmt.Errorf("deployment attempt must be positive")
	}
	if d.AdapterTrustSource != "" && !d.AdapterTrustSource.valid() {
		return fmt.Errorf("deployment adapter trust_source %q is not allowlisted", d.AdapterTrustSource)
	}
	if len(d.AdapterMessage) > MaxDeploymentAdapterMessageBytes {
		return fmt.Errorf("deployment adapter message exceeds %d bytes", MaxDeploymentAdapterMessageBytes)
	}
	if _, err := parseRFC3339("started_at", d.StartedAt); err != nil {
		return err
	}
	if d.FinishedAt != "" {
		if _, err := parseRFC3339("finished_at", d.FinishedAt); err != nil {
			return err
		}
	}
	return nil
}

func ValidateHealthObservation(h HealthObservationV1) error {
	if h.Schema != HealthObservationSchemaV1 {
		return fmt.Errorf("unknown health schema %q", h.Schema)
	}
	if !h.ReleaseIdentity.Environment.valid() {
		return fmt.Errorf("unknown environment %q", h.ReleaseIdentity.Environment)
	}
	if missing := firstEmpty(
		"deployment_id", h.DeploymentID, "criterion_id", h.CriterionID, "health_check", h.HealthCheck,
		"threshold", h.Threshold, "observation", h.Observation, "observed_at", h.Freshness.ObservedAt,
		"max_age", h.Freshness.MaxAge, "release_id", h.ReleaseIdentity.ReleaseID,
		"artifact_digest", h.ReleaseIdentity.ArtifactDigest, "source", h.Source,
	); missing != "" {
		return fmt.Errorf("health observation missing %s", missing)
	}
	if _, err := parseRFC3339("observed_at", h.Freshness.ObservedAt); err != nil {
		return err
	}
	if _, err := time.ParseDuration(h.Freshness.MaxAge); err != nil {
		return fmt.Errorf("invalid max_age: %w", err)
	}
	if h.Freshness.WindowStartedAt != "" {
		if _, err := parseRFC3339("window_started_at", h.Freshness.WindowStartedAt); err != nil {
			return err
		}
	}
	return nil
}

func ValidateRollback(r RollbackV1) error {
	if r.Schema != RollbackSchemaV1 && r.Schema != RollbackSchemaV2 {
		return fmt.Errorf("unknown rollback schema %q", r.Schema)
	}
	if r.Schema == RollbackSchemaV1 {
		if missing := firstEmpty("deployment_id", r.DeploymentID, "rollback_target", r.RollbackTarget, "reason", r.Reason, "adapter", r.Adapter, "action_result", r.ActionResult, "capability_class", r.CapabilityClass); missing != "" {
			return fmt.Errorf("rollback missing %s", missing)
		}
		return nil
	}
	if missing := firstEmpty("deployment_id", r.DeploymentID, "failed_release_id", r.FailedReleaseID, "rollback_target", r.RollbackTarget, "reason", r.Reason, "adapter", r.Adapter, "adapter_identity", r.AdapterIdentity, "action_result", r.ActionResult, "capability_class", r.CapabilityClass); missing != "" {
		return fmt.Errorf("rollback missing %s", missing)
	}
	if r.CapabilityClass != RollbackCapabilityAutomatic && r.CapabilityClass != RollbackCapabilityHumanRequired {
		return fmt.Errorf("unknown rollback capability_class %q", r.CapabilityClass)
	}
	if r.CapabilityClass == RollbackCapabilityHumanRequired && !r.HumanRequired {
		return fmt.Errorf("human_required capability requires human_required=true")
	}
	if missing := firstEmpty("post_rollback_health.criterion_id", r.PostRollbackHealth.CriterionID, "post_rollback_health.observation", r.PostRollbackHealth.Observation, "post_rollback_health.observed_at", r.PostRollbackHealth.ObservedAt); missing != "" {
		return fmt.Errorf("rollback incomplete: missing %s", missing)
	}
	if _, err := parseRFC3339("post_rollback_health.observed_at", r.PostRollbackHealth.ObservedAt); err != nil {
		return err
	}
	return nil
}

// ValidateCanaryWindow proves observation lasted for the declared duration and
// every supplied fact binds the exact deployment/release/artifact/environment.
func ValidateCanaryWindow(d DeploymentV1, observations []HealthObservationV1, now time.Time) error {
	if d.Status != StatusObserving {
		return fmt.Errorf("canary must be observing, got %s", d.Status)
	}
	if now.IsZero() {
		return fmt.Errorf("canary verdict time required")
	}
	started, err := parseRFC3339("started_at", d.StartedAt)
	if err != nil {
		return err
	}
	window, err := time.ParseDuration(d.Window)
	if err != nil || window <= 0 {
		return fmt.Errorf("invalid canary window %q", d.Window)
	}
	windowStart := now.Add(-window)
	if started.After(windowStart) {
		return fmt.Errorf("canary observation window incomplete")
	}
	if len(observations) == 0 {
		return fmt.Errorf("canary missing observations")
	}
	for _, o := range observations {
		if err := ValidateHealthObservation(o); err != nil {
			return err
		}
		if o.DeploymentID != d.DeploymentID || o.ReleaseIdentity.ReleaseID != d.ReleaseID || o.ReleaseIdentity.ArtifactDigest != d.ArtifactDigest || o.ReleaseIdentity.Environment != d.Environment {
			return fmt.Errorf("health observation identity mismatch")
		}
		if o.Freshness.WindowStartedAt == "" {
			return fmt.Errorf("health observation missing window_started_at")
		}
		observedStart, _ := time.Parse(time.RFC3339, o.Freshness.WindowStartedAt)
		if observedStart.After(windowStart) {
			return fmt.Errorf("health observation does not cover full window")
		}
	}
	return nil
}

type DeliveryTransition struct {
	From           DeploymentStatus
	To             DeploymentStatus
	Release        ReleaseCandidateV1
	Deployment     DeploymentV1
	Health         *HealthObservationV1
	RollbackTarget string
	Now            time.Time
}

var deliveryTransitions = map[DeploymentStatus]map[DeploymentStatus]struct{}{
	StatusRequested:   {StatusStarted: {}},
	StatusStarted:     {StatusObserving: {}},
	StatusObserving:   {StatusHealthy: {}, StatusFailed: {}},
	StatusFailed:      {StatusRollingBack: {}},
	StatusRollingBack: {StatusRolledBack: {}},
}

func ValidateDeliveryTransition(t DeliveryTransition) error {
	if !t.From.valid() {
		return fmt.Errorf("unknown deployment status %q", t.From)
	}
	if !t.To.valid() {
		return fmt.Errorf("unknown deployment status %q", t.To)
	}
	if _, ok := deliveryTransitions[t.From][t.To]; !ok {
		return fmt.Errorf("invalid delivery transition %s -> %s", t.From, t.To)
	}
	if err := ValidateReleaseCandidate(t.Release); err != nil {
		return err
	}
	if err := ValidateDeployment(t.Deployment); err != nil {
		return err
	}
	if t.Deployment.ReleaseID != t.Release.ReleaseID {
		return fmt.Errorf("release identity mismatch: deployment %q candidate %q", t.Deployment.ReleaseID, t.Release.ReleaseID)
	}
	if t.Deployment.GitHead != t.Release.GitHead {
		return fmt.Errorf("git HEAD mismatch: deployment %q candidate %q", t.Deployment.GitHead, t.Release.GitHead)
	}
	if t.Deployment.ArtifactDigest != t.Release.ArtifactDigest {
		return fmt.Errorf("artifact digest mismatch: deployment %q candidate %q", t.Deployment.ArtifactDigest, t.Release.ArtifactDigest)
	}
	if t.To == StatusFailed || t.From == StatusFailed || t.From == StatusRollingBack {
		if strings.TrimSpace(t.RollbackTarget) == "" {
			return fmt.Errorf("rollback target required for transition to or from failed")
		}
	}
	if t.To != StatusHealthy {
		return nil
	}
	if t.Health == nil {
		return fmt.Errorf("health observation required for healthy transition")
	}
	if err := ValidateHealthObservation(*t.Health); err != nil {
		return err
	}
	if t.Health.DeploymentID != t.Deployment.DeploymentID ||
		t.Health.ReleaseIdentity.ReleaseID != t.Release.ReleaseID ||
		t.Health.ReleaseIdentity.ArtifactDigest != t.Release.ArtifactDigest ||
		t.Health.ReleaseIdentity.Environment != t.Deployment.Environment {
		return fmt.Errorf("health observation identity mismatch")
	}
	observed, _ := time.Parse(time.RFC3339, t.Health.Freshness.ObservedAt)
	maxAge, _ := time.ParseDuration(t.Health.Freshness.MaxAge)
	if t.Now.IsZero() {
		return fmt.Errorf("transition time required for freshness check")
	}
	if observed.After(t.Now) || t.Now.Sub(observed) > maxAge {
		return fmt.Errorf("health observation stale at %s", t.Now.Format(time.RFC3339))
	}
	return nil
}

func firstEmpty(fields ...string) string {
	for i := 0; i+1 < len(fields); i += 2 {
		if strings.TrimSpace(fields[i+1]) == "" {
			return fields[i]
		}
	}
	return ""
}

func parseRFC3339(field, value string) (time.Time, error) {
	v, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s: %w", field, err)
	}
	return v, nil
}
