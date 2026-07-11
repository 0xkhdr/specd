package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// TrajectoryPolicy is the deterministic contract a trajectory must satisfy: a
// set of tool/action identities that must appear, and a set that must not
// (spec 04 R4.2). It is declarative and offline — no scorer, model, or network.
type TrajectoryPolicy struct {
	Required  []string
	Forbidden []string
}

// TrajectoryResult reports how a normalized trace measured against a policy.
// Missing lists required tools absent from the trace; Forbidden lists forbidden
// tools present; DigestMismatch is true when the trace does not match the
// expected pinned digest. Verdict is pass only when all three are clean.
type TrajectoryResult struct {
	Verdict        EvalVerdict
	Missing        []string
	Forbidden      []string
	DigestMismatch bool
	Malformed      bool
}

// EvaluateTrajectory checks a normalized trace (JSONL, one observable event per
// line) against a policy and an expected trace digest. It lives in core and is
// intentionally decoupled from the orchestration trace producer — it re-reads
// only the tool identity from each line, so completion gates never depend on
// the orchestration package. Passing an expectedDigest of "" skips the digest
// check (the caller did not pin one). No hidden reasoning is consulted: only
// observable tool identities drive the verdict (spec 04 R4.1/R4.2).
func EvaluateTrajectory(normalized []byte, policy TrajectoryPolicy, expectedDigest string) TrajectoryResult {
	present := map[string]bool{}
	for _, line := range bytes.Split(normalized, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var ev struct {
			Tool string `json:"tool"`
		}
		if err := json.Unmarshal(line, &ev); err != nil || ev.Tool == "" {
			res := TrajectoryResult{Verdict: EvalFail, Malformed: true}
			if expectedDigest != "" && Digest(normalized) != expectedDigest {
				res.DigestMismatch = true
			}
			return res
		}
		present[ev.Tool] = true
	}
	res := TrajectoryResult{Verdict: EvalPass}
	for _, req := range policy.Required {
		if !present[req] {
			res.Missing = append(res.Missing, req)
		}
	}
	for _, forb := range policy.Forbidden {
		if present[forb] {
			res.Forbidden = append(res.Forbidden, forb)
		}
	}
	sort.Strings(res.Missing)
	sort.Strings(res.Forbidden)
	if expectedDigest != "" && Digest(normalized) != expectedDigest {
		res.DigestMismatch = true
	}
	if len(res.Missing) > 0 || len(res.Forbidden) > 0 || res.DigestMismatch {
		res.Verdict = EvalFail
	}
	return res
}

// Err returns a stable ordered failure describing a non-passing result, or nil.
func (r TrajectoryResult) Err() error {
	if r.Verdict == EvalPass {
		return nil
	}
	switch {
	case r.Malformed:
		return fmt.Errorf("TRAJECTORY_MALFORMED: trace is not normalized observable JSONL")
	case r.DigestMismatch:
		return fmt.Errorf("TRAJECTORY_DIGEST_MISMATCH: trace does not match pinned digest")
	case len(r.Forbidden) > 0:
		return fmt.Errorf("TRAJECTORY_FORBIDDEN_STEP: %v", r.Forbidden)
	default:
		return fmt.Errorf("TRAJECTORY_MISSING_STEP: %v", r.Missing)
	}
}
