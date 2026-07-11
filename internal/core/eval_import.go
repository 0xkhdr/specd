package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// ImportExpect constrains what an imported adapter artifact may contain. Zero
// value means "no constraint": import still validates schema/provenance, but
// does not pin the record to a task/check/artifact. Every field is a local,
// caller-supplied fact — import never contacts a provider, model, or network
// (spec 04 R3.1).
type ImportExpect struct {
	SpecSlug string
	TaskID   string
	// CheckIDs, when non-empty, restricts accepted records to these check ids: a
	// record naming any other check fails closed (R3.2 wrong-check).
	CheckIDs []string
	// Artifacts maps a record's artifact_ref to the referenced content. When an
	// imported record's ArtifactRef has an entry, import recomputes its digest and
	// rejects a mismatch (R3.2 wrong-digest). Absent ref ⇒ digest not verifiable
	// here, left to the storing gate.
	Artifacts map[string][]byte
	// Traces maps a trajectory record's artifact_ref to the normalized trace
	// bytes; import recomputes their digest and rejects a TraceDigest mismatch
	// (spec 04 R4.2 trace digest validation).
	Traces map[string][]byte
}

// ImportFinding is one stable, ordered reason an artifact record was rejected.
// Index is the record's position in the artifact (0-based); findings sort by
// Index then Code so the same bad artifact always reports the same sequence.
type ImportFinding struct {
	Index      int
	EvidenceID string
	Code       string
	Message    string
}

// splitEvalArtifact yields one raw JSON object per record. It accepts a JSON
// array (multi-line pretty artifact) or JSONL (one object per line) — both are
// common adapter outputs — without guessing beyond the leading byte.
func splitEvalArtifact(raw []byte) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return nil, err
		}
		return arr, nil
	}
	var out []json.RawMessage
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		out = append(out, json.RawMessage(bytes.TrimSpace(line)))
	}
	return out, nil
}

// ImportEvals validates an adapter artifact deterministically and offline. It
// returns the accepted records only when there are zero findings: imported
// content cannot become proof before schema/policy validation passes (R3.2).
// A malformed/truncated/duplicate/wrong-task/wrong-check/wrong-digest record
// yields a stable ordered finding rather than a partial accept.
func ImportEvals(raw []byte, expect ImportExpect) ([]EvidenceEnvelopeV1, []ImportFinding) {
	items, err := splitEvalArtifact(raw)
	if err != nil {
		return nil, []ImportFinding{{Index: 0, Code: "EVAL_IMPORT_MALFORMED", Message: err.Error()}}
	}
	allow := map[string]bool{}
	for _, id := range expect.CheckIDs {
		allow[id] = true
	}
	var records []EvidenceEnvelopeV1
	var findings []ImportFinding
	seen := map[string]bool{}
	for i, item := range items {
		var env EvidenceEnvelopeV1
		decoder := json.NewDecoder(bytes.NewReader(item))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&env); err != nil {
			findings = append(findings, ImportFinding{Index: i, Code: "EVAL_IMPORT_MALFORMED", Message: err.Error()})
			continue
		}
		if err := ValidateEvidenceEnvelope(env); err != nil {
			findings = append(findings, ImportFinding{Index: i, EvidenceID: env.EvidenceID, Code: "EVAL_IMPORT_INVALID", Message: err.Error()})
			continue
		}
		if expect.SpecSlug != "" && env.SpecSlug != expect.SpecSlug {
			findings = append(findings, ImportFinding{Index: i, EvidenceID: env.EvidenceID, Code: "EVAL_IMPORT_SPEC_MISMATCH", Message: fmt.Sprintf("spec_slug %q != expected %q", env.SpecSlug, expect.SpecSlug)})
			continue
		}
		if expect.TaskID != "" && env.TaskID != expect.TaskID {
			findings = append(findings, ImportFinding{Index: i, EvidenceID: env.EvidenceID, Code: "EVAL_IMPORT_TASK_MISMATCH", Message: fmt.Sprintf("task_id %q != expected %q", env.TaskID, expect.TaskID)})
			continue
		}
		if len(allow) > 0 && !allow[env.CheckID] {
			findings = append(findings, ImportFinding{Index: i, EvidenceID: env.EvidenceID, Code: "EVAL_IMPORT_CHECK_UNKNOWN", Message: fmt.Sprintf("check_id %q not in declared checks", env.CheckID)})
			continue
		}
		if seen[env.EvidenceID] {
			findings = append(findings, ImportFinding{Index: i, EvidenceID: env.EvidenceID, Code: "EVAL_IMPORT_DUPLICATE", Message: "duplicate evidence_id within artifact"})
			continue
		}
		if want, ok := expect.Artifacts[env.ArtifactRef]; ok && Digest(want) != env.ArtifactDigest {
			findings = append(findings, ImportFinding{Index: i, EvidenceID: env.EvidenceID, Code: "EVAL_IMPORT_DIGEST_MISMATCH", Message: fmt.Sprintf("artifact_digest %q does not match %s", env.ArtifactDigest, env.ArtifactRef)})
			continue
		}
		if env.EvidenceClass == EvidenceTrajectoryEval {
			if trace, ok := expect.Traces[env.ArtifactRef]; ok && Digest(trace) != env.TraceDigest {
				findings = append(findings, ImportFinding{Index: i, EvidenceID: env.EvidenceID, Code: "EVAL_IMPORT_TRACE_MISMATCH", Message: fmt.Sprintf("trace_digest %q does not match normalized trace %s", env.TraceDigest, env.ArtifactRef)})
				continue
			}
		}
		seen[env.EvidenceID] = true
		records = append(records, env)
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Index != findings[j].Index {
			return findings[i].Index < findings[j].Index
		}
		return findings[i].Code < findings[j].Code
	})
	if len(findings) > 0 {
		return nil, findings
	}
	return records, nil
}
