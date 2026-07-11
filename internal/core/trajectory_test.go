package core

import "testing"

func TestEvaluateTrajectoryRequiredForbiddenDigest(t *testing.T) {
	trace := []byte("{\"tool\":\"read\"}\n{\"tool\":\"edit\"}\n{\"tool\":\"verify\"}")
	digest := Digest(trace)

	// clean: required present, forbidden absent, digest matches
	res := EvaluateTrajectory(trace, TrajectoryPolicy{Required: []string{"verify"}, Forbidden: []string{"deploy"}}, digest)
	if res.Verdict != EvalPass || res.Err() != nil {
		t.Fatalf("clean trace failed: %+v (%v)", res, res.Err())
	}

	// missing required step
	res = EvaluateTrajectory(trace, TrajectoryPolicy{Required: []string{"test"}}, "")
	if res.Verdict != EvalFail || len(res.Missing) != 1 || res.Missing[0] != "test" {
		t.Fatalf("missing not detected: %+v", res)
	}

	// forbidden step present
	res = EvaluateTrajectory(trace, TrajectoryPolicy{Forbidden: []string{"edit"}}, "")
	if res.Verdict != EvalFail || len(res.Forbidden) != 1 || res.Forbidden[0] != "edit" {
		t.Fatalf("forbidden not detected: %+v", res)
	}

	// digest mismatch
	res = EvaluateTrajectory(trace, TrajectoryPolicy{}, "not-the-digest")
	if res.Verdict != EvalFail || !res.DigestMismatch {
		t.Fatalf("digest mismatch not detected: %+v", res)
	}

	res = EvaluateTrajectory([]byte("not-json\n"), TrajectoryPolicy{}, "")
	if res.Verdict != EvalFail || !res.Malformed || res.Err() == nil {
		t.Fatalf("malformed trace accepted: %+v", res)
	}
}
