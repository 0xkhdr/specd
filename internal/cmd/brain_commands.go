package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/obs"
)

// brainCompact implements `specd brain compact` and (with reasonOverride
// "manual-clear") `specd brain clear`. It renders + persists a phase summary and
// ledger checkpoint, emits a context.compact observability event, then prints
// the outcome. The host performs the real /clear on the back of this.
func brainCompact(root string, args cli.Args, reasonOverride string) int {
	if len(args.Pos) != 2 || args.Str("session") == "" {
		return usageExit("usage: specd brain compact <slug> --session <id> [--reason <text>] [--json]")
	}
	slug := args.Pos[1]
	if code, ok := requireOrchestratedSpec(root, slug); !ok {
		return code
	}
	sessionID := args.Str("session")
	reason := reasonOverride
	if reason == "" {
		if reason = args.Str("reason"); reason == "" {
			reason = "manual-clear"
		}
	}
	outcome, err := core.CompactOrchestrationSession(root, slug, sessionID, reason)
	if err != nil {
		return specdExit(err)
	}
	logger, closer := obs.NewSessionLogger(root, sessionID)
	if closer != nil {
		defer closer.Close()
	}
	ctx := obs.WithFields(context.Background(), slug, string(outcome.Entry.Phase), "")
	obs.LogContextCompact(ctx, logger, sessionID, outcome.PreEstimatedTokens, outcome.Entry.HostReportedTokens, outcome.Entry.Budget, outcome.Entry.Reason, outcome.SummaryFile)
	return printCommandResult(args, outcome)
}

// brainLedger implements `specd brain ledger`: a read-only print of the session
// context ledger (peak tokens, compaction points, budget history). It performs
// zero LLM calls; --json emits the raw, machine-parseable ledger.
func brainLedger(root string, args cli.Args) int {
	if len(args.Pos) != 2 || args.Str("session") == "" {
		return usageExit("usage: specd brain ledger <slug> --session <id> [--json]")
	}
	slug := args.Pos[1]
	if code, ok := requireOrchestratedSpec(root, slug); !ok {
		return code
	}
	session, err := core.LoadOrchestrationSession(root, args.Str("session"))
	if err != nil {
		return specdExit(err)
	}
	if args.Bool("json") {
		return printCommandResult(args, map[string]any{
			"session":            session.SessionID,
			"peakTokens":         session.PeakTokens,
			"lastCompactionStep": session.LastCompactionStep,
			"ledger":             session.ContextLedger,
		})
	}
	fmt.Printf("brain ledger — session %s\n", session.SessionID)
	fmt.Printf("peak tokens: %d\n", session.PeakTokens)
	fmt.Printf("last compaction step: %d\n", session.LastCompactionStep)
	if len(session.ContextLedger) == 0 {
		fmt.Println("(no ledger entries)")
		return core.ExitOK
	}
	fmt.Printf("%-6s %-9s %-9s %-9s %-9s %-9s %-9s %s\n", "step", "phase", "action", "est", "host", "budget", "soft", "reason")
	for _, e := range session.ContextLedger {
		fmt.Printf("%-6d %-9s %-9s %-9d %-9d %-9d %-9d %s\n",
			e.StepSequence, e.Phase, e.Action, e.EstimatedTokens, e.HostReportedTokens, e.Budget, e.SoftCeiling, e.Reason)
	}
	return core.ExitOK
}

// brainCheckpoint implements `specd brain checkpoint <slug> --session <id>`:
// force-checkpoint every worker holding an active lease before the host sheds
// context (Req 3). Each worker's task is handed back with the supplied reason so
// the Brain resumes it rather than restarting. Exits 0 with a message when no
// worker is active or the resilience gate is off.
func brainCheckpoint(root string, args cli.Args) int {
	if len(args.Pos) != 2 || args.Str("session") == "" {
		return usageExit("usage: specd brain checkpoint <slug> --session <id> [--reason <text>] [--json]")
	}
	slug := args.Pos[1]
	if code, ok := requireOrchestratedSpec(root, slug); !ok {
		return code
	}
	cfg := core.LoadConfig(root).Orchestration
	if !checkpointEnabled(cfg) {
		fmt.Println("checkpointing disabled (set orchestration.resilience.checkpointEnabled)")
		return core.ExitOK
	}
	reason := args.Str("reason")
	if reason == "" {
		reason = "host-clear"
	}
	records, err := core.ForceCheckpointAll(root, args.Str("session"), reason, cfg)
	if err != nil {
		return specdExit(err)
	}
	if len(records) == 0 {
		if args.Bool("json") {
			return printCommandResult(args, []core.CheckpointRecord{})
		}
		fmt.Println("no active workers to checkpoint")
		return core.ExitOK
	}
	if args.Bool("json") {
		return printCommandResult(args, records)
	}
	fmt.Printf("checkpointed %d worker(s) for session %s (reason: %s)\n", len(records), args.Str("session"), reason)
	return core.ExitOK
}

// brainResumeList implements `specd brain resume --list`: print every session
// worth resuming after a host restart (running|paused), most-recent first,
// optionally bounded by --max-age-minutes (R5 Req 1). It is a pure read; --json
// emits the machine-parseable array a host adapter consumes on startup, and an
// empty result is `[]` with exit 0.
func brainResumeList(root string, args cli.Args) int {
	var maxAge time.Duration
	if args.Has("max-age-minutes") {
		minutes, ok := parseNonNegativeIntFlag(args, "max-age-minutes")
		if !ok {
			return usageExit("specd brain resume --list: --max-age-minutes must be a non-negative integer")
		}
		maxAge = time.Duration(minutes) * time.Minute
	}
	sessions, err := core.ListResumableSessions(root, maxAge)
	if err != nil {
		return specdExit(err)
	}
	if args.Bool("json") || core.IsJSONMode() {
		if err := core.PrintJSON(sessions); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	if len(sessions) == 0 {
		fmt.Println("no resumable sessions")
		return core.ExitOK
	}
	fmt.Printf("%-34s %-16s %-10s %-26s %s\n", "session", "spec", "status", "updatedAt", "lastDecision")
	for _, s := range sessions {
		fmt.Printf("%-34s %-16s %-10s %-26s %s\n", s.SessionID, s.Spec, s.Status, s.UpdatedAt, s.LastDecision)
	}
	return core.ExitOK
}

func brainDirective(root string, args cli.Args) int {
	if len(args.Pos) != 1 || args.Str("session") == "" || args.Str("worker") == "" || args.Str("spec") == "" || args.Str("task") == "" || args.Str("action") == "" || args.Str("reason") == "" {
		return usageExit("usage: specd brain directive --session <id> --worker <id> --spec <slug> --task <id> --attempt <n> --action <continue|retry|cancel|reassign|escalate> --reason <text> [--in-reply-to <message-id>] [--json]")
	}
	attempt, ok := parsePositiveIntFlag(args, "attempt")
	if !ok {
		return usageExit("specd brain directive: --attempt must be a positive integer")
	}
	cfg := core.LoadConfig(root).Orchestration
	event, err := core.RecordBrainDirective(root, core.BrainDirective{
		SessionID: args.Str("session"),
		WorkerID:  args.Str("worker"),
		Spec:      args.Str("spec"),
		TaskID:    args.Str("task"),
		Attempt:   attempt,
		Action:    args.Str("action"),
		Reason:    args.Str("reason"),
		InReplyTo: args.Str("in-reply-to"),
	}, cfg)
	if err != nil {
		return specdExit(err)
	}
	return printCommandResult(args, event)
}

func brainProgramStatus(root string, args cli.Args) int {
	cfg := core.LoadConfig(root).Orchestration
	report, err := core.SenseProgramOrchestration(root, args.Str("session"), cfg)
	if err != nil {
		return specdExit(err)
	}
	return printCommandResult(args, report)
}

func brainSessionControl(root string, args cli.Args, fn func(string, string) (core.OrchestrationSession, error), verb string) int {
	if len(args.Pos) != 1 || args.Str("session") == "" {
		return usageExit(fmt.Sprintf("usage: specd brain %s --session <id> [--json]", verb))
	}
	session, err := fn(root, args.Str("session"))
	if err != nil {
		if core.IsOrchestrationSessionNotFound(err) {
			return specdExit(core.NotFoundError(err.Error()))
		}
		return specdExit(err)
	}
	return printCommandResult(args, session)
}

func brainProgramSessionControl(root string, args cli.Args, fn func(string, string) (core.ProgramSession, error), verb string) int {
	if len(args.Pos) != 1 || args.Str("session") == "" {
		return usageExit(fmt.Sprintf("usage: specd brain %s --program --session <id> [--json]", verb))
	}
	session, err := fn(root, args.Str("session"))
	if err != nil {
		if core.IsOrchestrationSessionNotFound(err) {
			return specdExit(core.NotFoundError(err.Error()))
		}
		return specdExit(err)
	}
	return printCommandResult(args, session)
}

func brainWhy(root string, args cli.Args) int {
	sessionID := args.Str("session")
	if sessionID == "" {
		return usageExit("usage: specd brain why --session <id> [--json]")
	}
	events, err := obs.ReadTimeline(root, sessionID)
	if err != nil {
		if os.IsNotExist(err) {
			return specdExit(core.NotFoundError(fmt.Sprintf("no structured timeline for session %q", sessionID)))
		}
		return specdExit(err)
	}
	if len(events) == 0 {
		return specdExit(core.NotFoundError(fmt.Sprintf("no structured timeline for session %q", sessionID)))
	}
	if args.Bool("json") || core.IsJSONMode() {
		return printCommandResult(args, map[string]any{"session": sessionID, "events": events})
	}
	fmt.Printf("brain why — %s\n", sessionID)
	fmt.Println(" event      worker       task        dur_ms  exit")
	for _, ev := range events {
		fmt.Printf(" %-10s %-12s %-10s %-7d %d\n", ev.Event, ev.Worker, ev.Task, ev.DurMS, ev.Exit)
	}
	return core.ExitOK
}

func printCommandResult(args cli.Args, v any) int {
	if args.Bool("json") {
		_ = core.PrintJSON(v)
		return core.ExitOK
	}
	fmt.Printf("%+v\n", v)
	return core.ExitOK
}
