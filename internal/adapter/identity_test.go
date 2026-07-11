package adapter

import "testing"

func TestIdentityMatch(t *testing.T) {
	req := sampleRequest()
	res := sampleResult()
	if err := MatchIdentity(req, res); err != nil {
		t.Fatalf("matching identity rejected: %v", err)
	}
}

func TestIdentityMismatch(t *testing.T) {
	req := sampleRequest()
	mutate := map[string]func(*Result){
		"request_id":     func(r *Result) { r.RequestID = "other" },
		"correlation_id": func(r *Result) { r.CorrelationID = "other" },
		"spec_slug":      func(r *Result) { r.Subject.SpecSlug = "other" },
		"task_id":        func(r *Result) { r.Subject.TaskID = "other" },
		"mission_id":     func(r *Result) { r.Subject.MissionID = "other" },
		"git_head":       func(r *Result) { r.Subject.GitHead = "deadbeef" },
		"release_id":     func(r *Result) { r.Subject.ReleaseID = "other" },
		"environment":    func(r *Result) { r.Subject.Environment = "prod" },
		"adapter_name":   func(r *Result) { r.AdapterName = "" },
		"adapter_ver":    func(r *Result) { r.AdapterVersion = "" },
	}
	for name, m := range mutate {
		t.Run(name, func(t *testing.T) {
			res := sampleResult()
			m(&res)
			err := MatchIdentity(req, res)
			if err == nil {
				t.Fatalf("mismatch on %s not rejected", name)
			}
			if f, ok := err.(*Finding); !ok || f.Class != ErrIdentityMismatch {
				t.Fatalf("want ErrIdentityMismatch, got %v", err)
			}
		})
	}
}

func TestIdentityStale(t *testing.T) {
	res := sampleResult() // input digests design=d1, src=d2
	// Current pinned subject unchanged → not historical.
	if Historical(res, map[string]string{"design": "d1", "src": "d2"}) {
		t.Fatal("fresh result marked historical")
	}
	// A drifted input digest → historical, never silently current (R3.3).
	if !Historical(res, map[string]string{"design": "d1", "src": "CHANGED"}) {
		t.Fatal("stale result not marked historical")
	}
}
