package core

import (
	"encoding/json"
	"fmt"
	"sort"
)

const (
	ManifestDataset = "dataset"
	ManifestRubric  = "rubric"
	ReviewDraft     = "draft"
	ReviewApproved  = "approved"
)

type EvalCase struct {
	ID     string   `json:"id"`
	Labels []string `json:"labels,omitempty"`
	Ref    string   `json:"ref"`
	Digest string   `json:"digest"`
}

type EvalManifestV1 struct {
	SchemaVersion string      `json:"schema_version"`
	Kind          string      `json:"kind"`
	ID            string      `json:"id"`
	Owner         string      `json:"owner"`
	Version       string      `json:"version"`
	Digest        string      `json:"digest"`
	Cases         []EvalCase  `json:"cases"`
	CriticalCases []string    `json:"critical_cases,omitempty"`
	Redaction     string      `json:"redaction"`
	Source        string      `json:"source"`
	ReviewState   string      `json:"review_state"`
	Repetitions   int         `json:"repetitions"`
	Aggregation   Aggregation `json:"aggregation"`
	Threshold     float64     `json:"threshold"`
}

func EvalManifestDigest(m EvalManifestV1) (string, error) {
	if m.SchemaVersion == "" {
		m.SchemaVersion = EvalSchemaVersion
	}
	m.Digest = ""
	for i := range m.Cases {
		sort.Strings(m.Cases[i].Labels)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return Digest(raw), nil
}

func ValidateEvalManifest(m EvalManifestV1) error {
	if m.SchemaVersion == "" {
		m.SchemaVersion = EvalSchemaVersion
	}
	if m.SchemaVersion != EvalSchemaVersion {
		return fmt.Errorf("EVAL_MANIFEST_VERSION_UNSUPPORTED")
	}
	if m.Kind != ManifestDataset && m.Kind != ManifestRubric {
		return fmt.Errorf("EVAL_MANIFEST_KIND_UNKNOWN")
	}
	if m.ID == "" || m.Owner == "" || m.Version == "" || m.Digest == "" || m.Source == "" {
		return fmt.Errorf("EVAL_MANIFEST_REQUIRED_FIELD")
	}
	if len(m.Cases) == 0 || m.Redaction != "refs-only" || m.Repetitions < 1 || m.ReviewState != ReviewApproved {
		return fmt.Errorf("EVAL_MANIFEST_GOVERNANCE_INVALID")
	}
	seen := map[string]bool{}
	for _, c := range m.Cases {
		if c.ID == "" || c.Ref == "" || c.Digest == "" || seen[c.ID] {
			return fmt.Errorf("EVAL_MANIFEST_CASE_INVALID")
		}
		seen[c.ID] = true
	}
	for _, id := range m.CriticalCases {
		if !seen[id] {
			return fmt.Errorf("EVAL_MANIFEST_CRITICAL_CASE_UNKNOWN")
		}
	}
	digest, err := EvalManifestDigest(m)
	if err != nil {
		return err
	}
	if digest != m.Digest {
		return fmt.Errorf("EVAL_MANIFEST_DIGEST_MISMATCH")
	}
	return nil
}

// ValidateEvalEvidenceManifests binds imported labelled evidence to the exact
// immutable dataset and rubric versions used to produce it.
func ValidateEvalEvidenceManifests(e EvidenceEnvelopeV1, dataset, rubric EvalManifestV1) error {
	if err := ValidateEvalManifest(dataset); err != nil {
		return err
	}
	if err := ValidateEvalManifest(rubric); err != nil {
		return err
	}
	if e.EvidenceClass != EvidenceOutputEval {
		return fmt.Errorf("EVAL_MANIFEST_CLASS_MISMATCH")
	}
	if e.DatasetDigest != dataset.Digest || e.RubricDigest != rubric.Digest {
		return fmt.Errorf("EVAL_EVIDENCE_MANIFEST_STALE")
	}
	return nil
}
