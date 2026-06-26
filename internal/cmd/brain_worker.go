package cmd

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"syscall"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/obs"
	"github.com/0xkhdr/specd/internal/worker"
)

// brainRunner is the injectable worker.Runner seam at the cmd/driver layer. The
// hardcoded literal lives here (not inside the driver functions) so production
// keeps byte-identical worker.ShellRunner{} behavior while tests can swap in a
// recording fake — driving the whole live path without spawning real `sh`.
var brainRunner worker.Runner = worker.ShellRunner{}

// brainRunProgramWorker adapts the single-spec worker callback to the program
// driver: each child dispatch reuses the same shell-out contract (mission via
// temp file + SPECD_* env). An empty workerCmd returns nil — the loop then stops
// at the first child dispatch.
func brainRunProgramWorker(runner worker.Runner, root, workerCmd string, logger *slog.Logger) func(core.ProgramDriverDispatch) error {
	inner := brainRunWorker(runner, root, workerCmd, logger)
	if inner == nil {
		return nil
	}
	return func(d core.ProgramDriverDispatch) error {
		return inner(d.Dispatch)
	}
}

// brainRunWorker returns a DriveOrchestration worker callback that shells out to
// workerCmd per dispatch, passing the mission via a temp file and SPECD_* env.
// An empty workerCmd returns nil — the loop then stops at the first dispatch.
//
// Each worker runs under a deadline derived from the mission deadline (GAP-12):
// a hung or runaway Pinky can no longer stall Brain forever. The command runs in
// its own process group so a timeout kills the whole tree (`sh -c` spawns
// children; killing only the shell would orphan them). On timeout the callback
// returns an error, which the driver turns into a retryable failure so the next
// step applies the retry/escalate policy. Because workers now run concurrently
// (GAP-11), output is line-prefixed with the worker id so logs stay legible.
func brainRunWorker(runner worker.Runner, root, workerCmd string, logger *slog.Logger) func(core.DriverDispatch) error {
	if workerCmd == "" {
		return nil
	}
	return func(d core.DriverDispatch) error {
		m := worker.Mission{
			Command:   workerCmd,
			MissionID: d.Mission.DispatchDigest,
			SessionID: d.Mission.SessionID,
			WorkerID:  d.Mission.WorkerID,
			Spec:      d.Mission.Spec,
			TaskID:    d.Mission.TaskID,
			Role:      d.Mission.Role,
			Files:     d.Mission.Files,
			Deadline:  d.Mission.Deadline,
			Payload:   d.Mission,
		}
		ctx := obs.WithFields(context.Background(), m.Spec, missionPhase(root, m.Spec), m.Role)
		res, err := runner.Run(ctx, m)
		exit := workerExitCode(err)
		obs.LogEvent(ctx, logger, "complete", m.SessionID, m.WorkerID, m.TaskID, res.Duration, exit)
		if res.TimedOut {
			obs.LogEvent(ctx, logger, "timeout", m.SessionID, m.WorkerID, m.TaskID, res.Duration, exit)
		}
		return err
	}
}

// brainRunBootstrap applies the deterministic preflight remedies it can
// (creating the spec); it never fabricates a workspace or steering. Returns true
// if the spec is now ready to drive.
func brainRunBootstrap(root, slug string, args cli.Args, items []core.PreflightItem) bool {
	for _, item := range items {
		if item.Kind == "spec" && args.Bool("bootstrap") {
			newFlags := map[string]string{}
			if title := args.Str("title"); title != "" {
				newFlags["title"] = title
			}
			if code := RunNew(cli.Args{Pos: []string{slug}, Flags: newFlags}); code != core.ExitOK {
				errLine("brain run: bootstrap `%s` failed", item.Remedy)
				return false
			}
			continue
		}
		errLine("brain run: blocked — %s. Fix with `%s`%s.", item.Message, item.Remedy, bootstrapHint(item))
		return false
	}
	return true
}

func bootstrapHint(item core.PreflightItem) string {
	if item.Kind == "spec" {
		return " (or pass --bootstrap)"
	}
	return ""
}

func brainObserver(root string, logger *slog.Logger) core.DriverObserver {
	return func(ev core.DriverEvent) {
		slug, phase, role := brainEventScope(root, ev.Session, ev.Task)
		ctx := obs.WithFields(context.Background(), slug, phase, role)
		obs.LogEvent(ctx, logger, ev.Event, ev.Session, ev.Worker, ev.Task, 0, 0)
	}
}

func brainEventScope(root, sessionID, taskID string) (slug, phase, role string) {
	session, err := core.LoadOrchestrationSession(root, sessionID)
	if err != nil {
		return "", "", ""
	}
	slug = session.Spec
	state, err := core.LoadState(root, slug)
	if err != nil || state == nil {
		return slug, "", ""
	}
	phase = string(state.Phase)
	if taskID != "" {
		if task, ok := state.Tasks[taskID]; ok {
			role = task.Role
		}
	}
	return slug, phase, role
}

func missionPhase(root, slug string) string {
	state, err := core.LoadState(root, slug)
	if err != nil || state == nil {
		return ""
	}
	return string(state.Phase)
}

func workerExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return 1
}
