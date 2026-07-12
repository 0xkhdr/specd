package adapter

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// Adapter is the manifest of one opt-in, project-selected integration
// executable. It is data only: the runner drives it over stdin/stdout JSON and
// never links it into the process (the boundary invariant). The same manifest
// backs read-only `specd adapters` inspection, so it carries names, paths,
// versions, and offered capabilities — never secret values (R6.3).
type Adapter struct {
	Name          string   `json:"name"`
	Version       string   `json:"version,omitempty"`
	SchemaVersion string   `json:"schema_version,omitempty"` // adapter-envelope schema the executable speaks
	Path          string   `json:"path"`                     // executable resolved by the project
	Args          []string `json:"args,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"` // capabilities_offered by this adapter
	EnvAllow      []string `json:"env_allow,omitempty"`    // env var NAMES the adapter process may see
	Enabled       bool     `json:"enabled"`                // false ⇒ configured but disabled
}

// NegotiateCapabilities reports whether an adapter offering `offered` can satisfy
// a request requiring `required` (R7.1). Acceptance must occur before any side
// effect, so Run negotiates before invoking the executable. A missing capability
// returns a stable finding naming it; nil means every requirement is met.
func NegotiateCapabilities(required, offered []string) *Finding {
	have := make(map[string]bool, len(offered))
	for _, c := range offered {
		have[c] = true
	}
	for _, need := range required {
		if !have[need] {
			return newFinding(ErrInvalidValue, "capabilities_required",
				"adapter does not offer required capability "+need)
		}
	}
	return nil
}

// Run invokes adapter a for req, feeding the canonical request JSON on stdin and
// decoding the result envelope from stdout, bounded by req.Limits and a.EnvAllow.
//
// It fails closed on every path (R6.1/R6.2): a missing binary, timeout, oversized
// output, malformed result, or non-zero exit each yields a typed failing Result
// pinned to the request — none is recorded as success and none falls back to
// unsafe local execution. Capability negotiation (R7.1) and identity validation
// (R3) run around the invocation so an adapter result can never satisfy a gate
// before it is proven to belong to the request.
//
// Secrets reach the adapter only through the allowlisted execution environment
// (R6.3): Run passes exactly the env entries whose NAME is in a.EnvAllow and
// reads nothing from .specd artifacts or model context. The non-nil (possibly
// empty) env means the child never inherits the parent's environment.
//
// The returned error, when non-nil, is the *Finding explaining the failing or
// rejected record; callers record the Result as blocked/failed evidence.
func Run(a Adapter, req Request, env []string) (Result, error) {
	// R7.1: negotiate capabilities before any side effect. An unmet capability
	// is a semantic rejection — the executable is never started.
	if f := NegotiateCapabilities(req.CapabilitiesRequired, a.Capabilities); f != nil {
		return a.reject(req), f
	}

	reqJSON, err := req.Canonical()
	if err != nil {
		f := newFinding(ErrMalformed, "", "request not encodable: "+err.Error())
		return a.fail(req, StatusFailed, ExitMalformed), f
	}

	ctx := context.Background()
	if timeout := time.Duration(req.Limits.TimeoutMS) * time.Millisecond; timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, a.Path, a.Args...)
	cmd.Stdin = bytes.NewReader(reqJSON)
	cmd.Env = allowedEnv(env, a.EnvAllow)
	out := &capWriter{limit: req.Limits.OutputBytes}
	cmd.Stdout = out
	// Adapter stderr is diagnostics, not part of the result contract; discard it.

	runErr := cmd.Run()

	switch {
	case ctx.Err() == context.DeadlineExceeded:
		return a.fail(req, StatusTimedOut, ExitTimeout),
			newFinding(ErrInvalidValue, "", "adapter exceeded timeout")
	case out.truncated:
		return a.fail(req, StatusFailed, ExitOversized),
			newFinding(ErrInvalidValue, "", "adapter output exceeded the configured cap")
	case runErr != nil:
		return a.execFailure(req, runErr)
	}

	res, derr := DecodeResult(out.Bytes())
	if derr != nil {
		return a.fail(req, StatusFailed, ExitMalformed), derr
	}
	// R3.2: a result that does not belong to this request is rejected before its
	// status can satisfy any gate, even though the process exited cleanly.
	if ierr := MatchIdentity(req, res); ierr != nil {
		return a.reject(req), ierr
	}
	return res, nil
}

// execFailure maps a start/run error onto a typed failing record: a binary that
// could not be started is `unavailable`/missing_binary; a clean non-zero exit is
// `failed`/nonzero_exit; anything else fails closed as unavailable (R6.2).
func (a Adapter) execFailure(req Request, runErr error) (Result, error) {
	var execErr *exec.Error
	if errors.As(runErr, &execErr) || errors.Is(runErr, exec.ErrNotFound) {
		return a.fail(req, StatusUnavailable, ExitMissingBinary),
			newFinding(ErrInvalidValue, "", "adapter binary not runnable: "+runErr.Error())
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return a.fail(req, StatusFailed, ExitNonZero),
			newFinding(ErrInvalidValue, "", "adapter exited non-zero")
	}
	return a.fail(req, StatusUnavailable, ExitMissingBinary),
		newFinding(ErrInvalidValue, "", "adapter run error: "+runErr.Error())
}

// fail builds a typed failing Result pinned to req's identity (R6.2). It is not
// routed through MatchIdentity — it is synthesized here to carry the request's
// own identity, so it is always current and never mistaken for another request.
func (a Adapter) fail(req Request, status Status, class ExitClass) Result {
	now := core.Clock().UTC().Format(time.RFC3339)
	started := req.StartedAt
	if started == "" {
		started = now
	}
	return Result{
		SchemaVersion:  SchemaVersion,
		Kind:           resultKind(req.Kind),
		RequestID:      req.RequestID,
		CorrelationID:  req.CorrelationID,
		Subject:        req.Subject,
		AdapterName:    a.Name,
		AdapterVersion: a.Version,
		Status:         status,
		ExitClass:      class,
		Retryable:      status.Retryable(),
		StartedAt:      started,
		FinishedAt:     now,
	}
}

// reject builds a rejected record for a semantic refusal (capability unmet or
// identity mismatch). The transport itself was fine — the executable either was
// never started or exited cleanly — so exit_class is ok while status is rejected.
func (a Adapter) reject(req Request) Result {
	return a.fail(req, StatusRejected, ExitOK)
}

// resultKind derives the result kind from a request kind: "<domain>.request"
// becomes "<domain>.result". A request that omits the suffix still yields a
// well-formed result kind.
func resultKind(reqKind string) string {
	return strings.TrimSuffix(reqKind, ".request") + ".result"
}

// allowedEnv returns only the "NAME=value" entries whose NAME is in allow. This
// is the sole channel by which secrets reach an adapter (R6.3). The result is
// always non-nil so the child process never inherits the parent environment: an
// empty allowlist yields an empty (not nil) environment.
func allowedEnv(env, allow []string) []string {
	want := make(map[string]bool, len(allow))
	for _, name := range allow {
		want[name] = true
	}
	out := make([]string, 0, len(allow))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i >= 0 && want[kv[:i]] {
			out = append(out, kv)
		}
	}
	return out
}

// capWriter buffers adapter stdout up to limit bytes and records whether more was
// offered. A zero limit means unbounded. It never returns a short write, so the
// child is not killed by a broken pipe before the cap is observed here.
type capWriter struct {
	buf       bytes.Buffer
	limit     int64
	truncated bool
}

func (w *capWriter) Write(p []byte) (int, error) {
	if w.limit > 0 {
		remaining := w.limit - int64(w.buf.Len())
		if remaining <= 0 {
			w.truncated = true
			return len(p), nil
		}
		if int64(len(p)) > remaining {
			w.buf.Write(p[:remaining])
			w.truncated = true
			return len(p), nil
		}
	}
	return w.buf.Write(p)
}

func (w *capWriter) Bytes() []byte { return w.buf.Bytes() }
