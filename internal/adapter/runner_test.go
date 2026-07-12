package adapter

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// fakeAdapterSentinel is the first argument the test binary recognizes as "act
// as a fake adapter" instead of running the test suite. Passing the mode as an
// argument (not an env var) keeps the control channel out of the env allowlist,
// so the secrets test can assert a strict allowlist without breaking dispatch.
const fakeAdapterSentinel = "__specd_fake_adapter__"

// TestMain re-execs the test binary as a fake adapter when invoked with the
// sentinel, so runner tests can drive a real subprocess without shell scripts or
// external tooling (portable, zero runtime dependencies).
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == fakeAdapterSentinel {
		mode := ""
		if len(os.Args) > 2 {
			mode = os.Args[2]
		}
		fakeAdapterMain(mode)
		return
	}
	os.Exit(m.Run())
}

func fakeAdapterMain(mode string) {
	data, _ := io.ReadAll(os.Stdin)
	switch mode {
	case "garbage":
		os.Stdout.WriteString("this is not a json envelope")
		return
	case "boom":
		os.Exit(3)
	case "big":
		os.Stdout.Write(bytes.Repeat([]byte("x"), 100_000))
		return
	}
	req, err := DecodeRequest(data)
	if err != nil {
		os.Exit(9)
	}
	if mode == "slow" {
		time.Sleep(2 * time.Second)
	}
	res := okResult(req)
	switch mode {
	case "wrongid":
		res.RequestID = "someone-elses-request"
	case "echoenv":
		res.Measurements = Measurements{}
		if os.Getenv("SPECD_TOKEN") != "" {
			res.Measurements["saw_token"] = 1
		}
		if os.Getenv("SPECD_SECRET") != "" {
			res.Measurements["saw_secret"] = 1
		}
	}
	out, _ := res.Canonical()
	os.Stdout.Write(out)
}

func okResult(req Request) Result {
	return Result{
		SchemaVersion:       SchemaVersion,
		Kind:                resultKind(req.Kind),
		RequestID:           req.RequestID,
		CorrelationID:       req.CorrelationID,
		Subject:             req.Subject,
		AdapterName:         "fake",
		AdapterVersion:      "1.0.0",
		CapabilitiesOffered: req.CapabilitiesRequired,
		Status:              StatusSucceeded,
		ExitClass:           ExitOK,
		StartedAt:           req.StartedAt,
		FinishedAt:          req.StartedAt,
	}
}

func fakeAdapter(mode string, caps, envAllow []string) Adapter {
	return Adapter{
		Name:         "fake",
		Version:      "1.0.0",
		Path:         os.Args[0],
		Args:         []string{fakeAdapterSentinel, mode},
		Capabilities: caps,
		EnvAllow:     envAllow,
		Enabled:      true,
	}
}

func fakeRequest() Request {
	return Request{
		SchemaVersion: SchemaVersion,
		Kind:          "eval.request",
		RequestID:     "req-1",
		CorrelationID: "corr-1",
		Subject:       Subject{SpecSlug: "demo", TaskID: "T01", GitHead: "abc123"},
		Limits:        Limits{TimeoutMS: 5000, OutputBytes: 65536},
		StartedAt:     "2026-01-01T00:00:00Z",
		AdapterName:   "fake",
	}
}

// TestRunnerSuccess: a well-behaved adapter yields a valid succeeded result.
func TestRunnerSuccess(t *testing.T) {
	res, err := Run(fakeAdapter("ok", nil, nil), fakeRequest(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusSucceeded || res.ExitClass != ExitOK {
		t.Fatalf("status=%s exit=%s, want succeeded/ok", res.Status, res.ExitClass)
	}
	if verr := res.Validate(); verr != nil {
		t.Fatalf("result must be a valid envelope: %v", verr)
	}
}

// TestRunnerFailureModes: every transport failure produces a typed failing
// record, never a success and never a fallback (R6.1/R6.2).
func TestRunnerFailureModes(t *testing.T) {
	cases := []struct {
		name       string
		adapter    Adapter
		req        Request
		wantStatus Status
		wantExit   ExitClass
	}{
		{"missing_binary", func() Adapter { a := fakeAdapter("ok", nil, nil); a.Path = "/no/such/specd-adapter"; return a }(), fakeRequest(), StatusUnavailable, ExitMissingBinary},
		{"timeout", fakeAdapter("slow", nil, nil), func() Request { r := fakeRequest(); r.Limits.TimeoutMS = 50; return r }(), StatusTimedOut, ExitTimeout},
		{"oversized", fakeAdapter("big", nil, nil), func() Request { r := fakeRequest(); r.Limits.OutputBytes = 1000; return r }(), StatusFailed, ExitOversized},
		{"malformed", fakeAdapter("garbage", nil, nil), fakeRequest(), StatusFailed, ExitMalformed},
		{"nonzero_exit", fakeAdapter("boom", nil, nil), fakeRequest(), StatusFailed, ExitNonZero},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Run(tc.adapter, tc.req, nil)
			if err == nil {
				t.Fatal("expected a finding error for a failing record")
			}
			if res.Status != tc.wantStatus || res.ExitClass != tc.wantExit {
				t.Fatalf("status=%s exit=%s, want %s/%s", res.Status, res.ExitClass, tc.wantStatus, tc.wantExit)
			}
			if res.Status == StatusSucceeded {
				t.Fatal("a failure mode must never be recorded as success")
			}
			if verr := res.Validate(); verr != nil {
				t.Fatalf("failing record must still be a valid envelope: %v", verr)
			}
			if res.RequestID != tc.req.RequestID {
				t.Fatalf("failing record must be pinned to the request, got %q", res.RequestID)
			}
		})
	}
}

// TestRunnerIdentityMismatch: a clean exit that returns another request's result
// is rejected before it can satisfy any gate (R3.2).
func TestRunnerIdentityMismatch(t *testing.T) {
	res, err := Run(fakeAdapter("wrongid", nil, nil), fakeRequest(), nil)
	if err == nil {
		t.Fatal("identity mismatch must be reported")
	}
	if res.Status != StatusRejected {
		t.Fatalf("status=%s, want rejected", res.Status)
	}
}

// TestRunnerCapabilityNegotiation: an adapter that does not offer a required
// capability is rejected before the executable is ever started (R7.1).
func TestRunnerCapabilityNegotiation(t *testing.T) {
	req := fakeRequest()
	req.CapabilitiesRequired = []string{"gpu"}
	// The adapter would exit 3 ("boom") if it ran; a rejected status proves it did not.
	res, err := Run(fakeAdapter("boom", []string{"cpu"}, nil), req, nil)
	if err == nil {
		t.Fatal("unmet capability must be reported")
	}
	if res.Status != StatusRejected {
		t.Fatalf("status=%s, want rejected (pre-exec, no side effect)", res.Status)
	}

	// When the offered set covers the required set, negotiation passes.
	req.CapabilitiesRequired = []string{"cpu"}
	if _, err := Run(fakeAdapter("ok", []string{"cpu", "gpu"}, nil), req, nil); err != nil {
		t.Fatalf("satisfied capability must accept: %v", err)
	}
}

// TestRunnerSecretsViaEnvAllowlist: only allowlisted env vars reach the adapter;
// a secret outside the allowlist never crosses into the adapter process (R6.3).
func TestRunnerSecretsViaEnvAllowlist(t *testing.T) {
	env := []string{"SPECD_TOKEN=abc", "SPECD_SECRET=xyz"}
	res, err := Run(fakeAdapter("echoenv", nil, []string{"SPECD_TOKEN"}), fakeRequest(), env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Measurements["saw_token"] != 1 {
		t.Fatal("allowlisted SPECD_TOKEN should have reached the adapter")
	}
	if _, saw := res.Measurements["saw_secret"]; saw {
		t.Fatal("SPECD_SECRET is not allowlisted and must not reach the adapter")
	}
}

func TestAllowedEnvNeverNil(t *testing.T) {
	if got := allowedEnv([]string{"A=1"}, nil); got == nil {
		t.Fatal("allowedEnv must return non-nil so the child never inherits the parent env")
	} else if len(got) != 0 {
		t.Fatalf("empty allowlist must pass no env, got %v", got)
	}
	if !strings.Contains(strings.Join(allowedEnv([]string{"A=1", "B=2"}, []string{"B"}), ","), "B=2") {
		t.Fatal("allowlisted var must pass through")
	}
}
