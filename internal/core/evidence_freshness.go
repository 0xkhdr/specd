package core

// FreshnessSubject is the current, reachable state a required evidence record
// must match to still count as proof (spec 04 R3.3). Revision is the subject
// commit; the digest fields are the configured current subject digests the
// completion policy pins. A zero field means "not configured": that dimension
// is not checked, so an empty subject verifies nothing (parity — legacy specs
// with no quality policy keep completing on verify alone).
type FreshnessSubject struct {
	Revision      string
	DiffDigest    string
	OutputDigest  string
	DatasetDigest string
	RubricDigest  string
	TraceDigest   string
}

// RecordFresh reports whether e is current for subject s. A record is stale
// when it pins a different subject revision, or when it pins a digest that the
// subject also configures and the two disagree. A configured digest missing
// from the record is stale: absent provenance cannot prove currentness. An
// unconfigured subject dimension is not checked (R3.3). Freshness never
// deletes or rewrites records; stale records stay auditable in the store.
func RecordFresh(e EvidenceEnvelopeV1, s FreshnessSubject) bool {
	if s.Revision != "" && e.SubjectRevision != s.Revision {
		return false
	}
	for _, pair := range [][2]string{
		{e.DiffDigest, s.DiffDigest},
		{e.OutputDigest, s.OutputDigest},
		{e.DatasetDigest, s.DatasetDigest},
		{e.RubricDigest, s.RubricDigest},
		{e.TraceDigest, s.TraceDigest},
	} {
		if pair[1] != "" && pair[0] != pair[1] {
			return false
		}
	}
	return true
}

// QualityStatus splits a task's required evidence into missing (no passing
// record at all) and stale (a passing record exists but is not current for the
// subject). OK reports both empty.
type QualityStatus struct {
	Missing []EvidenceRequirement
	Stale   []EvidenceRequirement
}

func (q QualityStatus) OK() bool { return len(q.Missing) == 0 && len(q.Stale) == 0 }

// EvaluateQuality resolves a task's quality contract against the imported eval
// records and the current subject. Only a passing record satisfies a
// requirement, and only a fresh passing record does — a required deterministic
// test that failed leaves the requirement missing regardless of any later eval
// score or review, so a failing test always blocks completion (R3.4). It is a
// pure, allocation-light superset of MissingQualityEvidence with freshness.
func EvaluateQuality(c QualityContract, records []EvidenceEnvelopeV1, s FreshnessSubject) QualityStatus {
	freshPass := map[string]bool{}
	anyPass := map[string]bool{}
	for _, r := range records {
		if r.Verdict != EvalPass {
			continue
		}
		if c.TaskID != "" && r.TaskID != c.TaskID {
			continue
		}
		key := string(r.EvidenceClass) + "/" + r.CheckID
		anyPass[key] = true
		if RecordFresh(r, s) {
			freshPass[key] = true
		}
	}
	var st QualityStatus
	for _, req := range c.Required {
		key := string(req.EvidenceClass) + "/" + req.CheckID
		switch {
		case freshPass[key]:
		case anyPass[key]:
			st.Stale = append(st.Stale, req)
		default:
			st.Missing = append(st.Missing, req)
		}
	}
	return st
}
