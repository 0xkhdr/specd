package cmd

import (
	"context"
	"fmt"
	"os"

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
