package adapter

// MatchIdentity rejects a result whose identity does not match its pinned
// request (R3.1/R3.2). It must be called before a result's status is allowed to
// satisfy any gate, completion, deploy, or eval decision: a status of
// "succeeded" is meaningless if the result does not belong to the request that
// asked for it. Every mismatch returns a stable ErrIdentityMismatch finding.
func MatchIdentity(req Request, res Result) error {
	checks := []struct {
		field     string
		want, got string
	}{
		{"request_id", req.RequestID, res.RequestID},
		{"correlation_id", req.CorrelationID, res.CorrelationID},
		{"spec_slug", req.Subject.SpecSlug, res.Subject.SpecSlug},
		{"task_id", req.Subject.TaskID, res.Subject.TaskID},
		{"mission_id", req.Subject.MissionID, res.Subject.MissionID},
		{"git_head", req.Subject.GitHead, res.Subject.GitHead},
		{"release_id", req.Subject.ReleaseID, res.Subject.ReleaseID},
		{"environment", req.Subject.Environment, res.Subject.Environment},
	}
	for _, c := range checks {
		if c.want != c.got {
			return newFinding(ErrIdentityMismatch, c.field,
				"result "+c.field+" does not match the pinned request")
		}
	}
	// The result must name the adapter that produced it (R3.1); an unnamed or
	// unversioned adapter cannot be trusted to satisfy a gate.
	if res.AdapterName == "" || res.AdapterVersion == "" {
		return newFinding(ErrIdentityMismatch, "adapter_version",
			"result must carry adapter_name and adapter_version")
	}
	return nil
}

// Historical reports whether a result is stale relative to the current pinned
// subject (R3.3): if any input digest it was computed against differs from the
// digest of the current subject, the result describes past state and must be
// marked historical rather than mistaken for current.
func Historical(res Result, current map[string]string) bool {
	for name, dig := range res.InputDigests {
		if cur, ok := current[name]; ok && cur != dig {
			return true
		}
	}
	return false
}
