package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
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

// VerifyProducer is the producer identity stamped on envelopes `specd verify`
// writes for declared test/* checks (spec R2.1). Every other evidence class
// keeps `specd eval import` as its only producer (R2.2).
const VerifyProducer = "specd-verify"

// VerifyStamp carries the provenance of one passing `specd verify` run so it
// can be re-recorded as class-tagged evidence. SubjectRevision is the same
// pinned git HEAD written to evidence.jsonl; RunID/Attempt come from the shared
// run allocator, keeping both schemas on one attempt chain.
type VerifyStamp struct {
	SpecSlug        string
	TaskID          string
	RunID           string
	Attempt         int
	SubjectRevision string
	ProducerVersion string
	ConfigDigest    string
	ArtifactRef     string
	ArtifactDigest  string
	CreatedAt       time.Time
}

// BuildVerifyEnvelopes projects a passing verify run into one passing
// EvidenceEnvelopeV1 per declared `test/<check-id>` requirement (spec R2.1),
// closing the verify → complete-task loop for test-class contracts. Non-test
// requirements (output_eval, trajectory_eval, review) yield nothing: a verify
// run cannot attest to them, so external import stays their only producer
// (R2.2). This is a pure projection — it records the same exit-0 + pinned-HEAD
// fact in a second schema and never weakens the evidence gate.
func BuildVerifyEnvelopes(c QualityContract, stamp VerifyStamp) []EvidenceEnvelopeV1 {
	var envelopes []EvidenceEnvelopeV1
	for _, req := range c.Required {
		if req.EvidenceClass != EvidenceTest {
			continue
		}
		identity := strings.Join([]string{stamp.SpecSlug, stamp.TaskID, req.CheckID, stamp.RunID, strconv.Itoa(stamp.Attempt), stamp.SubjectRevision}, "\x00")
		envelopes = append(envelopes, EvidenceEnvelopeV1{
			SchemaVersion:   EvalSchemaVersion,
			EvidenceID:      Digest([]byte(identity))[:16],
			EvidenceClass:   EvidenceTest,
			SpecSlug:        stamp.SpecSlug,
			TaskID:          stamp.TaskID,
			RunID:           stamp.RunID,
			Attempt:         stamp.Attempt,
			SubjectRevision: stamp.SubjectRevision,
			Producer:        VerifyProducer,
			ProducerVersion: stamp.ProducerVersion,
			ConfigDigest:    stamp.ConfigDigest,
			CheckID:         req.CheckID,
			Verdict:         EvalPass,
			CreatedAt:       stamp.CreatedAt.Format(time.RFC3339),
			Actor:           VerifyProducer,
			ArtifactRef:     stamp.ArtifactRef,
			ArtifactDigest:  stamp.ArtifactDigest,
		})
	}
	return envelopes
}

func EvidenceEnvelopeDigest(e EvidenceEnvelopeV1) string {
	sort.Strings(e.RequiredSteps)
	sort.Strings(e.ForbiddenSteps)
	raw, _ := json.Marshal(e)
	return Digest(raw)
}
