package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

const EvalSchemaVersion = "1"

type EvidenceClass string

const (
	EvidenceTest           EvidenceClass = "test"
	EvidenceOutputEval     EvidenceClass = "output_eval"
	EvidenceTrajectoryEval EvidenceClass = "trajectory_eval"
	EvidenceReview         EvidenceClass = "review"
)

type EvalVerdict string

const (
	EvalPass         EvalVerdict = "pass"
	EvalFail         EvalVerdict = "fail"
	EvalInsufficient EvalVerdict = "insufficient"
)

type EvidenceEnvelopeV1 struct {
	SchemaVersion   string        `json:"schema_version"`
	EvidenceID      string        `json:"evidence_id"`
	EvidenceClass   EvidenceClass `json:"evidence_class"`
	SpecSlug        string        `json:"spec_slug"`
	TaskID          string        `json:"task_id"`
	RunID           string        `json:"run_id"`
	Attempt         int           `json:"attempt"`
	SubjectRevision string        `json:"subject_revision"`
	DiffDigest      string        `json:"diff_digest,omitempty"`
	Producer        string        `json:"producer"`
	ProducerVersion string        `json:"producer_version"`
	ConfigDigest    string        `json:"config_digest"`
	CheckID         string        `json:"check_id"`
	Verdict         EvalVerdict   `json:"verdict"`
	Score           *float64      `json:"score,omitempty"`
	CreatedAt       string        `json:"created_at"`
	Actor           string        `json:"actor"`
	ArtifactRef     string        `json:"artifact_ref"`
	ArtifactDigest  string        `json:"artifact_digest"`
	DatasetDigest   string        `json:"dataset_digest,omitempty"`
	RubricDigest    string        `json:"rubric_digest,omitempty"`
	OutputDigest    string        `json:"output_digest,omitempty"`
	TraceDigest     string        `json:"trace_digest,omitempty"`
	RequiredSteps   []string      `json:"required_steps,omitempty"`
	ForbiddenSteps  []string      `json:"forbidden_steps,omitempty"`
}

func ValidateEvidenceEnvelope(e EvidenceEnvelopeV1) error {
	if e.SchemaVersion != EvalSchemaVersion {
		return fmt.Errorf("EVAL_VERSION_UNSUPPORTED: %q", e.SchemaVersion)
	}
	switch e.EvidenceClass {
	case EvidenceTest, EvidenceOutputEval, EvidenceTrajectoryEval, EvidenceReview:
	default:
		return fmt.Errorf("EVAL_CLASS_UNKNOWN: %q", e.EvidenceClass)
	}
	if e.EvidenceID == "" || e.SpecSlug == "" || e.TaskID == "" || e.RunID == "" || e.Attempt < 1 || e.SubjectRevision == "" || e.Producer == "" || e.ProducerVersion == "" || e.ConfigDigest == "" || e.CheckID == "" || e.CreatedAt == "" || e.Actor == "" || e.ArtifactRef == "" || e.ArtifactDigest == "" {
		return fmt.Errorf("EVAL_REQUIRED_FIELD: envelope identity/provenance incomplete")
	}
	if _, err := time.Parse(time.RFC3339, e.CreatedAt); err != nil {
		return fmt.Errorf("EVAL_TIME_INVALID: %v", err)
	}
	if e.Verdict != EvalPass && e.Verdict != EvalFail && e.Verdict != EvalInsufficient {
		return fmt.Errorf("EVAL_VERDICT_UNKNOWN: %q", e.Verdict)
	}
	if e.EvidenceClass == EvidenceOutputEval && (e.DatasetDigest == "" || e.RubricDigest == "" || e.OutputDigest == "") {
		return fmt.Errorf("EVAL_OUTPUT_DIGEST_REQUIRED")
	}
	if e.EvidenceClass == EvidenceTrajectoryEval && e.TraceDigest == "" {
		return fmt.Errorf("EVAL_TRACE_DIGEST_REQUIRED")
	}
	return nil
}

func EvidenceEnvelopeDigest(e EvidenceEnvelopeV1) string {
	sort.Strings(e.RequiredSteps)
	sort.Strings(e.ForbiddenSteps)
	raw, _ := json.Marshal(e)
	return Digest(raw)
}

func AdaptLegacyVerify(slug string, r EvidenceRecord) EvidenceEnvelopeV1 {
	verdict := EvalFail
	if r.ExitCode == 0 {
		verdict = EvalPass
	}
	created := r.Timestamp
	if created == "" {
		created = time.Unix(0, 0).UTC().Format(time.RFC3339)
	}
	actor := r.Actor
	if actor == "" {
		actor = "legacy-verify"
	}
	return EvidenceEnvelopeV1{SchemaVersion: EvalSchemaVersion, EvidenceID: "legacy:" + r.TaskID + ":" + r.GitHead, EvidenceClass: EvidenceTest, SpecSlug: slug, TaskID: r.TaskID, RunID: "legacy:" + r.TaskID, Attempt: 1, SubjectRevision: r.GitHead, Producer: "specd-verify", ProducerVersion: "legacy", ConfigDigest: Digest(nil), CheckID: "verify", Verdict: verdict, CreatedAt: created, Actor: actor, ArtifactRef: r.EvidenceRef, ArtifactDigest: Digest([]byte(r.Command))}
}
