package cmd

import (
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

const observeUsage = "usage: specd observe correlate <payload.json> [--spec <slug>] [--json]  |  specd observe --listen [--spec <slug>]"

// RunObserve implements `specd observe` (V9/P5.2): the offline `correlate`
// transform (the core feature — CI pipes an exported error payload in) and the
// optional localhost `--listen` HTTP receiver. Both deterministically attribute
// an error to a spec and append an evidenced entry to that spec's
// mid-requirements.md, gating high/critical impact for human approval.
func RunObserve(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	if args.Bool("listen") {
		return runObserveListen(root, args)
	}
	if len(args.Pos) >= 1 && args.Pos[0] == "correlate" {
		return runObserveCorrelate(root, args)
	}
	return usageExit(observeUsage)
}

// runObserveCorrelate reads a payload file (size-capped), parses/validates it,
// correlates it to a spec, and applies the midreq entry.
func runObserveCorrelate(root string, args cli.Args) int {
	if len(args.Pos) < 2 {
		return usageExit(observeUsage)
	}
	path := args.Pos[1]
	info, err := os.Stat(path)
	if err != nil {
		return specdExit(core.NotFoundError(fmt.Sprintf("payload file not found: %s", path)))
	}
	cap := observeCap(root)
	if info.Size() > int64(cap) {
		return specdExit(core.GateError(fmt.Sprintf("payload %s exceeds %d bytes", path, cap)))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return specdExit(err)
	}
	payload, err := core.ParseErrorPayload(data)
	if err != nil {
		return specdExit(err)
	}
	corr, err := core.CorrelatePayload(root, payload, strings.TrimSpace(args.Str("spec")))
	if err != nil {
		return specdExit(err)
	}
	if err := applyCorrelation(root, payload, corr); err != nil {
		return specdExit(err)
	}
	if args.Bool("json") {
		return printJSONExit(map[string]interface{}{"ok": true, "spec": corr.Spec, "impact": corr.Impact, "confidence": corr.Confidence, "gated": gatedImpact(corr.Impact)})
	}
	fmt.Printf("✓ observe: correlated to '%s' (impact %s, confidence %s)\n", corr.Spec, corr.Impact, corr.Confidence)
	if gatedImpact(corr.Impact) {
		fmt.Println("⛔ gate set to awaiting-approval — present the fix plan before proceeding.")
	}
	return core.ExitOK
}

// applyCorrelation appends the deterministic midreq entry to the correlated
// spec under its state lock, bumping the turn counter and setting the
// awaiting-approval gate for high/critical impact — mirroring `specd midreq` so
// production-driven changes obey the same evidence discipline.
func applyCorrelation(root string, payload core.ErrorPayload, corr core.Correlation) error {
	_, err := core.WithSpecLock[int](root, corr.Spec, func() (int, error) {
		state, err := core.LoadState(root, corr.Spec)
		if err != nil || state == nil {
			return 0, err
		}
		state.Turn++
		if gatedImpact(corr.Impact) {
			state.Gate = core.GateAwaitingApproval
		}
		if err := core.SaveState(root, corr.Spec, state); err != nil {
			return 0, err
		}
		stamp := core.Clock().UTC().Format("2006-01-02T15:04")
		header := fmt.Sprintf("\n## Turn %d — %s — impact: %s (observe)\n", state.Turn, stamp, corr.Impact)
		entry := header + core.RenderObserveMidreq(payload, corr)
		return 0, core.AppendFile(core.ArtifactPath(root, corr.Spec, "mid-requirements.md"), entry)
	})
	return err
}

// runObserveListen starts the localhost inbound receiver. It binds loopback
// only, requires a configured bearer token on every request, and caps each
// payload. Every accepted payload is correlated and applied like `correlate`.
func runObserveListen(root string, args cli.Args) int {
	ln, srv, err := prepareObserveListener(root, strings.TrimSpace(args.Str("spec")))
	if err != nil {
		return specdExit(err)
	}
	fmt.Printf("observe: listening on http://%s/errors (loopback, token-authed)\n", ln.Addr())
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return specdExit(err)
	}
	return core.ExitOK
}

// prepareObserveListener validates the observe config, binds the loopback
// listener, and builds the server — everything up to (but not including) the
// blocking Serve call, so it is unit-testable. It fails closed when no token is
// configured or the bind address is not loopback.
func prepareObserveListener(root, forceSpec string) (net.Listener, *http.Server, error) {
	cfg := core.LoadConfig(root)
	token := strings.TrimSpace(cfg.Observe.Token)
	if token == "" {
		return nil, nil, core.GateError("observe: no observe.token configured — set config.observe.token to enable the listener")
	}
	addr := strings.TrimSpace(cfg.Observe.Addr)
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	if !isLoopbackAddr(addr) {
		return nil, nil, core.GateError(fmt.Sprintf("observe: listener addr %q is not loopback — the receiver binds localhost only", addr))
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	return ln, newObserveServer(root, token, observeCap(root), forceSpec), nil
}

// newObserveServer builds the inbound HTTP server that routes /errors through
// observeHandle. Split out so tests can drive it against a real listener and
// shut it down cleanly.
func newObserveServer(root, token string, cap int, forceSpec string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/errors", func(w http.ResponseWriter, r *http.Request) {
		observeHandle(w, r, root, token, cap, forceSpec)
	})
	return &http.Server{Handler: mux}
}

// observeHandle authenticates, size-limits, parses, correlates, and applies one
// inbound payload. Failures return specific reasons (V9 §5) without leaking state.
func observeHandle(w http.ResponseWriter, r *http.Request, root, token string, cap int, forceSpec string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !bearerOK(r.Header.Get("Authorization"), token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, int64(cap)+1))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	if len(data) > cap {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	payload, err := core.ParseErrorPayload(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	corr, err := core.CorrelatePayload(root, payload, forceSpec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if err := applyCorrelation(root, payload, corr); err != nil {
		http.Error(w, "apply error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":true,"spec":%q,"impact":%q,"confidence":%q}`, corr.Spec, corr.Impact, corr.Confidence)
}

// bearerOK constant-time compares the request bearer token to the configured one.
func bearerOK(header, token string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

// isLoopbackAddr reports whether addr binds only the loopback interface.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// observeCap returns the effective per-payload byte cap.
func observeCap(root string) int {
	cfg := core.LoadConfig(root)
	if cfg.Observe.MaxPayloadBytes > 0 {
		return cfg.Observe.MaxPayloadBytes
	}
	return core.DefaultMaxObserveBytes
}

// gatedImpact reports whether an impact stops work for human approval.
func gatedImpact(impact string) bool { return impact == "high" || impact == "critical" }
