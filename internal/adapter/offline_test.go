package adapter

import (
	"strings"
	"testing"
)

// TestProviderOutage pins R8.2 at the runner: an unreachable provider yields a
// durable, typed record blocked with an exact external cause — never an implicit
// success and never a fallback. The finding names the binary that could not be
// reached so an operator sees the precise external cause.
func TestProviderOutage(t *testing.T) {
	a := fakeAdapter("ok", nil, nil)
	a.Path = "/no/such/provider-adapter"

	res, err := Run(a, fakeRequest(), nil)
	if err == nil {
		t.Fatal("a provider outage must report a finding, not silently pass")
	}
	if res.Status != StatusUnavailable || res.ExitClass != ExitMissingBinary {
		t.Fatalf("status=%s exit=%s, want unavailable/missing_binary", res.Status, res.ExitClass)
	}
	if res.Status == StatusSucceeded {
		t.Fatal("an outage must never be recorded as success")
	}
	// R8.2: the cause is exact — the unreachable binary is named in the finding.
	if !strings.Contains(err.Error(), a.Path) {
		t.Fatalf("outage finding must name the unreachable binary %q, got %v", a.Path, err)
	}
	// A durable typed record: the blocked evidence is itself a valid envelope,
	// pinned to the request so it is never mistaken for another subject.
	if verr := res.Validate(); verr != nil {
		t.Fatalf("blocked record must still be a valid envelope: %v", verr)
	}
	if res.RequestID != fakeRequest().RequestID {
		t.Fatalf("blocked record must be pinned to the request, got %q", res.RequestID)
	}
}

// TestOfflineFailuresNeverPass pins the "never an implicit success or timeout-
// pass" half of R8.2: an unavailable provider and a timed-out one both resolve
// to a non-succeeded status with a non-ok exit class, so no gate, completion, or
// deploy decision can ever read an outage as a pass.
func TestOfflineFailuresNeverPass(t *testing.T) {
	cases := []struct {
		name    string
		adapter Adapter
		req     Request
	}{
		{"outage", func() Adapter { a := fakeAdapter("ok", nil, nil); a.Path = "/no/such/provider"; return a }(), fakeRequest()},
		{"timeout", fakeAdapter("slow", nil, nil), func() Request { r := fakeRequest(); r.Limits.TimeoutMS = 50; return r }()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Run(tc.adapter, tc.req, nil)
			if err == nil {
				t.Fatal("an unavailable provider must surface a finding")
			}
			if res.Status == StatusSucceeded {
				t.Fatalf("%s must not resolve to succeeded", tc.name)
			}
			if res.ExitClass == ExitOK {
				t.Fatalf("%s must not carry the ok exit class", tc.name)
			}
			// Retryable is bounded: an outage/timeout is retryable, so callers
			// deterministically re-dispatch rather than pass the stale record.
			if !res.Retryable {
				t.Fatalf("%s should be retryable so it is retried, not passed", tc.name)
			}
		})
	}
}
