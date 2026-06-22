package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// requireRootAndSlug resolves the .specd root and the first positional slug
// shared by every single-spec command. On failure it returns ok=false and the
// exit code the caller must return verbatim, collapsing the root-locate +
// slug-extract + empty-check prologue duplicated across the spec commands.
func requireRootAndSlug(args cli.Args, usage string) (root, slug string, code int, ok bool) {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return "", "", specdExit(err), false
	}
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if slug == "" {
		return "", "", usageExit(usage), false
	}
	return root, slug, core.ExitOK, true
}

// modeOriginOrDefault returns a spec's recorded mode origin, defaulting to
// "default" for specs that never opted in (empty ModeOrigin). Shared by the
// status/report/context surfaces that name the execution mode.
func modeOriginOrDefault(state *core.State) string {
	if state.ModeOrigin == "" {
		return core.OriginDefault
	}
	return state.ModeOrigin
}

// modeBriefing returns the one-line briefing naming what the execution mode
// implies for whoever reads the context: a Base host owns every step itself; an
// orchestrated spec may be driven by Brain dispatching Pinky workers.
func modeBriefing(mode string) string {
	if mode == core.ModeOrchestrated {
		return "orchestrated — Brain may dispatch Pinky workers across the wave DAG"
	}
	return "base — you own every step; drive it with `specd next` / `specd verify`"
}

func specdExit(err error) int {
	var se *core.SpecdError
	if errors.As(err, &se) {
		core.Error(se.Message)
		return se.Code
	}
	core.Error(err.Error())
	return core.ExitGate
}

func usageExit(msg string) int {
	core.Error(msg)
	return core.ExitUsage
}

// gatedPayload is the typed schema for the awaiting-approval JSON response.
// Fields are ordered to match the previous map[string]interface{} output (the
// JSON encoder sorts map keys), keeping the wire bytes identical.
type gatedPayload struct {
	Gate core.Gate `json:"gate"`
	Kind string    `json:"kind"`
}

// approvalGateBlocked reports whether the spec is parked at the awaiting-approval
// gate without --force and, if so, emits the standard gated response on the
// requested channel (JSON {"kind":"gated"} or a stderr notice). When it returns
// blocked=true the caller must return the accompanying exit code verbatim.
func approvalGateBlocked(args cli.Args, state *core.State, slug string, jsonOut bool) (code int, blocked bool) {
	if state.Gate != core.GateAwaitingApproval || args.Bool("force") {
		return core.ExitOK, false
	}
	if jsonOut {
		if err := core.PrintJSON(gatedPayload{Gate: state.Gate, Kind: "gated"}); err != nil {
			return specdExit(err), true
		}
	} else {
		errLine("⛔ gate awaiting-approval — present the revised plan, then `specd approve %s` (override: --force).", slug)
	}
	return core.ExitGate, true
}

// frontierStuckReason renders the human-readable line explaining why the
// runnable frontier is empty, given the scheduler's verdict. completeMsg lets
// each caller phrase the all-complete case in its own terms; an unrecognized
// verdict yields the empty string (the caller should print nothing).
func frontierStuckReason(r core.NextResult, completeMsg string) string {
	switch r.Kind {
	case core.NextAllComplete:
		return completeMsg
	case core.NextAllBlocked:
		return fmt.Sprintf("⚠ all remaining tasks blocked: %v", r.Blocked)
	case core.NextWaiting:
		return fmt.Sprintf("… waiting — frontier gated by incomplete deps: %v", r.Blocking)
	}
	return ""
}

// errLine writes a diagnostic line to stderr. It is the canonical stderr path
// for command-level `fail …` / `✗ …` output, keeping machine-readable results
// on stdout. A trailing newline is always appended. For styled single-line
// errors prefer core.Error; use errLine for the multi-line gate dumps.
func errLine(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}
