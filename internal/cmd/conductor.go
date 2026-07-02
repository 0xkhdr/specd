package cmd

import (
	"fmt"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func RunConductor(args cli.Args) int {
	if len(args.Pos) < 2 {
		return usageExit("usage: specd conductor <slug> <start|step|accept|reject|stop|replay|switch|status> [micro] [--json] [--reason <text>]")
	}
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd conductor <slug> <start|step|accept|reject|stop|replay|switch|status> [micro] [--json] [--reason <text>]")
	if !ok {
		return code
	}
	action := args.Pos[1]
	switch action {
	case "replay":
		return conductorReplay(root, slug, args.Bool("json"))
	case "status":
		return conductorStatus(root, slug, args.Bool("json"))
	case "start", "step", "accept", "reject", "stop", "switch":
	default:
		return usageExit("usage: specd conductor <slug> <start|step|accept|reject|stop|replay|switch|status> [micro] [--json] [--reason <text>]")
	}

	rc, err := core.WithSpecLock[int](root, slug, func() (int, error) {
		state, err := core.LoadState(root, slug)
		if err != nil {
			return core.ExitGate, err
		}
		plan, err := core.LoadConductorPlan(root, slug)
		if err != nil {
			return core.ExitGate, err
		}
		events, err := core.ReadConductorEvents(root, slug)
		if err != nil {
			return core.ExitGate, err
		}
		var ev core.ConductorEvent
		switch action {
		case "start":
			micro, err := selectMicro(plan, events, argMicro(args))
			if err != nil {
				return core.ExitGate, err
			}
			sessionID := fmt.Sprintf("%s-%s-%d", slug, strings.ReplaceAll(micro.ID, "/", "-"), len(events)+1)
			state.Conductor = &core.ConductorSession{SessionID: sessionID, Task: micro.TaskID, Micro: micro.ID, StartedAt: core.Clock().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")}
			ev = core.ConductorEvent{SessionID: sessionID, Action: "start", Task: micro.TaskID, Micro: micro.ID, Status: "active"}
		case "step":
			micro, err := selectMicro(plan, events, argMicro(args))
			if err != nil {
				return core.ExitGate, err
			}
			sessionID := ""
			if state.Conductor != nil {
				sessionID = state.Conductor.SessionID
			}
			ev = core.ConductorEvent{SessionID: sessionID, Action: "step", Task: micro.TaskID, Micro: micro.ID, Status: "ready"}
		case "accept", "reject":
			if state.Conductor == nil {
				return core.ExitGate, core.GateError("no active conductor session")
			}
			if action == "reject" && strings.TrimSpace(args.Str("reason")) == "" {
				// The rejection reason is the training signal; it is mandatory.
				return core.ExitGate, core.GateError("conductor reject requires --reason")
			}
			ev = core.ConductorEvent{SessionID: state.Conductor.SessionID, Action: action, Task: state.Conductor.Task, Micro: state.Conductor.Micro, Status: action, Reason: args.Str("reason")}
			// Both outcomes end the active attempt: accept advances the frontier,
			// reject re-queues the micro-task for another step.
			state.Conductor = nil
		case "stop":
			if state.Conductor == nil {
				return core.ExitGate, core.GateError("no active conductor session")
			}
			ev = core.ConductorEvent{SessionID: state.Conductor.SessionID, Action: "stop", Task: state.Conductor.Task, Micro: state.Conductor.Micro, Status: "stopped", Reason: args.Str("reason")}
			state.Conductor = nil
		case "switch":
			micro, err := selectMicro(plan, events, argMicro(args))
			if err != nil {
				return core.ExitGate, err
			}
			sessionID := fmt.Sprintf("%s-%s-%d", slug, strings.ReplaceAll(micro.ID, "/", "-"), len(events)+1)
			state.Conductor = &core.ConductorSession{SessionID: sessionID, Task: micro.TaskID, Micro: micro.ID, StartedAt: core.Clock().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")}
			ev = core.ConductorEvent{SessionID: sessionID, Action: "switch", Task: micro.TaskID, Micro: micro.ID, Status: "active", Reason: args.Str("reason")}
		}
		if err := core.AppendConductorEvent(root, slug, ev); err != nil {
			return core.ExitGate, err
		}
		if err := core.SaveState(root, slug, state); err != nil {
			return core.ExitGate, err
		}
		if args.Bool("json") {
			return core.ExitOK, core.PrintJSON(ev)
		}
		fmt.Printf("%s %s/%s\n", ev.Action, ev.Task, ev.Micro)
		return core.ExitOK, nil
	})
	if err != nil {
		return specdExit(err)
	}
	return rc
}

func conductorReplay(root, slug string, jsonOut bool) int {
	events, err := core.ReadConductorEvents(root, slug)
	if err != nil {
		return specdExit(err)
	}
	if jsonOut {
		if err := core.PrintJSON(events); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	for _, ev := range events {
		fmt.Printf("%s %s %s/%s %s\n", ev.Time, ev.Action, ev.Task, ev.Micro, ev.Status)
	}
	return core.ExitOK
}

func conductorStatus(root, slug string, jsonOut bool) int {
	state, err := core.LoadState(root, slug)
	if err != nil {
		return specdExit(err)
	}
	plan, err := core.LoadConductorPlan(root, slug)
	if err != nil {
		return specdExit(err)
	}
	events, err := core.ReadConductorEvents(root, slug)
	if err != nil {
		return specdExit(err)
	}
	status := core.BuildConductorStatus(state, plan, events)
	if jsonOut {
		if err := core.PrintJSON(status); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	if status.Active != nil {
		fmt.Printf("active %s/%s\n", status.Active.Task, status.Active.Micro)
	} else {
		fmt.Println("active none")
	}
	for _, micro := range status.Frontier {
		fmt.Printf("ready %s/%s %s\n", micro.TaskID, micro.ID, micro.Title)
	}
	return core.ExitOK
}

// runConductorReport prints the rejection-reason clustering for a spec's
// conductor ledger (exact-string reasons + counts). It is the `--conductor`
// projection of `specd report` and reads only the append-only ledger, so the
// output is deterministic from the recorded events.
func runConductorReport(root, slug string, jsonOut bool) int {
	events, err := core.ReadConductorEvents(root, slug)
	if err != nil {
		return specdExit(err)
	}
	clusters := core.ConductorRejectionReport(events)
	if jsonOut {
		if err := core.PrintJSON(map[string]interface{}{"spec": slug, "rejections": clusters}); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	if len(clusters) == 0 {
		fmt.Println("no conductor rejections recorded")
		return core.ExitOK
	}
	fmt.Printf("=== CONDUCTOR REJECTIONS: %s ===\n", slug)
	for _, c := range clusters {
		fmt.Printf("  x%-3d %s\n", c.Count, c.Reason)
	}
	return core.ExitOK
}

func argMicro(args cli.Args) string {
	if len(args.Pos) >= 3 {
		return args.Pos[2]
	}
	return ""
}

func selectMicro(plan core.ConductorPlan, events []core.ConductorEvent, requested string) (core.MicroTask, error) {
	frontier := core.ConductorFrontier(plan, events)
	if requested == "" {
		if len(frontier) == 0 {
			return core.MicroTask{}, core.GateError("no runnable micro tasks")
		}
		return frontier[0], nil
	}
	for _, micro := range frontier {
		if micro.ID == requested || micro.Key == requested {
			return micro, nil
		}
	}
	return core.MicroTask{}, core.GateError(fmt.Sprintf("micro task %s is not runnable", requested))
}
