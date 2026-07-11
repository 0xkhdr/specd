package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleRequest() Request {
	return Request{
		SchemaVersion: SchemaVersion,
		Kind:          "eval.request",
		RequestID:     "req-1",
		CorrelationID: "corr-1",
		Subject: Subject{
			SpecSlug: "10-scope", TaskID: "T04", MissionID: "m-1",
			GitHead: "abc123", ReleaseID: "r-9", Environment: "staging",
		},
		Actor:        "brain",
		AuthorityRef: "role/craftsman",
		InputRefs: []Ref{
			{Name: "design", Digest: "d1", Class: ClassSpecText},
			{Name: "src", Digest: "d2", Class: ClassSourcePath},
		},
		CapabilitiesRequired: []string{"eval.run"},
		Limits:               Limits{TimeoutMS: 30000, OutputBytes: 1 << 20},
		StartedAt:            "2026-07-11T00:00:00Z",
		AdapterName:          "fake-eval",
	}
}

func sampleResult() Result {
	return Result{
		SchemaVersion:       SchemaVersion,
		Kind:                "eval.result",
		RequestID:           "req-1",
		CorrelationID:       "corr-1",
		Subject:             sampleRequest().Subject,
		AdapterName:         "fake-eval",
		AdapterVersion:      "1.0.0",
		CapabilitiesOffered: []string{"eval.run"},
		Status:              StatusSucceeded,
		ExitClass:           ExitOK,
		Retryable:           false,
		OutputRefs:          []Ref{{Name: "score", Digest: "o1", Class: ClassPublicMetadata}},
		Measurements:        Measurements{"score": 0.91},
		InputDigests:        map[string]string{"design": "d1", "src": "d2"},
		StartedAt:           "2026-07-11T00:00:00Z",
		FinishedAt:          "2026-07-11T00:00:05Z",
	}
}

func TestEnvelopeRoundTrip(t *testing.T) {
	req := sampleRequest()
	b1, err := req.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	// Byte-semantic stability: re-encoding an identical envelope is identical.
	b2, _ := req.Canonical()
	if string(b1) != string(b2) {
		t.Fatalf("request encoding not byte-stable")
	}
	got, err := DecodeRequest(b1)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	b3, _ := got.Canonical()
	if string(b1) != string(b3) {
		t.Fatalf("request did not round-trip byte-identically")
	}
	d1, _ := req.Digest()
	d2, _ := got.Digest()
	if d1 != d2 || len(d1) != 64 {
		t.Fatalf("digest unstable: %q vs %q", d1, d2)
	}

	res := sampleResult()
	rb, err := res.Canonical()
	if err != nil {
		t.Fatalf("result canonical: %v", err)
	}
	gotRes, err := DecodeResult(rb)
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	rb2, _ := gotRes.Canonical()
	if string(rb) != string(rb2) {
		t.Fatalf("result did not round-trip byte-identically")
	}
}

func TestEnvelopeGoldenFixtures(t *testing.T) {
	for _, name := range []string{"request_v1.json", "result_v1.json"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(name, "request") {
			r, err := DecodeRequest(data)
			if err != nil {
				t.Fatalf("golden %s rejected: %v", name, err)
			}
			// A golden fixture must survive a canonical round-trip unchanged.
			enc, _ := r.Canonical()
			again, err := DecodeRequest(enc)
			if err != nil || again.RequestID != r.RequestID {
				t.Fatalf("golden %s failed round-trip", name)
			}
		} else {
			r, err := DecodeResult(data)
			if err != nil {
				t.Fatalf("golden %s rejected: %v", name, err)
			}
			enc, _ := r.Canonical()
			again, err := DecodeResult(enc)
			if err != nil || again.Status != r.Status {
				t.Fatalf("golden %s failed round-trip", name)
			}
		}
	}
}

func TestEnvelopeReject(t *testing.T) {
	base, _ := sampleRequest().Canonical()
	cases := []struct {
		name string
		data string
		want ErrorClass
	}{
		{"malformed", "{not json", ErrMalformed},
		{"unknown version", strings.Replace(string(base), `"adapter/v1"`, `"adapter/v9"`, 1), ErrUnknownVersion},
		{"unknown kind", strings.Replace(string(base), `"eval.request"`, `"eval.banana"`, 1), ErrUnknownKind},
		{"unknown field", `{"schema_version":"adapter/v1","kind":"eval.request","request_id":"x","correlation_id":"y","started_at":"t","bogus":1}`, ErrUnknownField},
		{"empty id", `{"schema_version":"adapter/v1","kind":"eval.request","request_id":"","correlation_id":"y","started_at":"t"}`, ErrInvalidValue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := DecodeRequest([]byte(c.data))
			if err == nil {
				t.Fatalf("expected rejection")
			}
			f, ok := err.(*Finding)
			if !ok {
				t.Fatalf("want *Finding, got %T", err)
			}
			if f.Class != c.want {
				t.Fatalf("class = %q, want %q", f.Class, c.want)
			}
		})
	}
}

func TestEnvelopeResultReject(t *testing.T) {
	// Bad status enum, bad exit class, and inconsistent retryable each fail closed.
	bad := sampleResult()
	bad.Status = "bogus"
	if _, err := DecodeResultValue(bad); err == nil {
		t.Fatal("bad status accepted")
	}
	bad = sampleResult()
	bad.ExitClass = "explode"
	if _, err := DecodeResultValue(bad); err == nil {
		t.Fatal("bad exit class accepted")
	}
	// R2.4: retryable must be deterministically bounded by status.
	bad = sampleResult()
	bad.Status = StatusRejected
	bad.Retryable = true
	if _, err := DecodeResultValue(bad); err == nil {
		t.Fatal("retryable=true on non-retryable status accepted")
	}
}

func TestStatusRetryable(t *testing.T) {
	retry := map[Status]bool{
		StatusSucceeded:   false,
		StatusRejected:    false,
		StatusFailed:      false,
		StatusTimedOut:    true,
		StatusUnavailable: true,
	}
	for s, want := range retry {
		if s.Retryable() != want {
			t.Errorf("%s.Retryable() = %v, want %v", s, s.Retryable(), want)
		}
	}
	for _, s := range []Status{StatusSucceeded, StatusRejected, StatusFailed, StatusTimedOut, StatusUnavailable} {
		if !s.Valid() {
			t.Errorf("%s should be valid", s)
		}
	}
	if Status("nope").Valid() {
		t.Error("unknown status validated")
	}
}

func TestExitClass(t *testing.T) {
	all := []ExitClass{ExitOK, ExitMissingBinary, ExitTimeout, ExitOversized, ExitMalformed, ExitNonZero}
	for _, e := range all {
		if !e.Valid() {
			t.Errorf("%s should be valid", e)
		}
	}
	if ExitClass("explode").Valid() {
		t.Error("unknown exit class validated")
	}
	// An unknown exit class must fail closed when a result carries it (R2.4/R6.2).
	bad := sampleResult()
	bad.ExitClass = "explode"
	if _, err := DecodeResultValue(bad); err == nil {
		t.Fatal("result with unknown exit class accepted")
	}
}

func TestInlineSizeBound(t *testing.T) {
	req := sampleRequest()
	req.InputRefs = append(req.InputRefs, Ref{
		Name: "big", Digest: "d3", Class: ClassSourceContent,
		Inline: strings.Repeat("x", MaxInlineBytes+1),
	})
	if err := req.Validate(); err == nil {
		t.Fatal("oversized inline content accepted (R4.3 bound)")
	}
}
