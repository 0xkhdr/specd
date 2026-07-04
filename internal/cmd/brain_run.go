package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

const brainLeaseTTL = 15 * time.Minute

// runBrain drives the deterministic orchestration controller (R13.7). Dispatch
// authority is fail-closed: without --authority the controller waits and writes
// nothing. No LLM sits in this path — Decide/Sense are pure functions of state.
func runBrain(root string, args []string, flags map[string]string) error {
	if len(args) != 2 {
		return errors.New("usage: specd brain <start|step|run|status> <spec>")
	}
	sub, slug := args[0], args[1]
	sessionPath := filepath.Join(core.SpecdDir(root), "specs", slug, "session.json")
	acpPath := filepath.Join(core.SpecdDir(root), "specs", slug, "acp.jsonl")

	switch sub {
	case "status":
		session, err := orchestration.LoadSession(sessionPath)
		if err != nil {
			return err
		}
		return writeJSON(session)
	case "start":
		if err := requireBrainStartPreconditions(root, slug); err != nil {
			return err
		}
		if err := orchestration.SaveSessionCAS(root, sessionPath, 0, orchestration.Session{}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "brain start: session initialized for %s\n", slug)
		return nil
	case "step", "run":
		// handled below
	default:
		return fmt.Errorf("unsupported brain subcommand %q", sub)
	}

	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return err
	}
	spec, err := loadSpec(root, slug)
	if err != nil {
		return err
	}
	frontier, err := core.Frontier(spec.Tasks, taskStatus(spec.Tasks))
	if err != nil {
		return err
	}
	session, err := orchestration.LoadSession(sessionPath)
	if err != nil {
		return err
	}

	now := time.Now()
	snapshot := orchestration.Sense(state, frontier, session.Leases, now)
	authority := orchestration.Authority{Enabled: flagEnabled(flags, "authority")}
	limits := orchestration.DecisionLimitsForAuthority(authority, orchestration.DecisionLimits{MaxRetries: 1})

	dispatcher := &sessionDispatcher{acpPath: acpPath, now: now, session: &session}
	decision, err := orchestration.DispatchFrontier(snapshot, limits, dispatcher)
	if err != nil {
		return err
	}
	if decision.Action == orchestration.ActionDispatch {
		if err := orchestration.SaveSessionCAS(root, sessionPath, session.Revision, session); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stdout, "brain %s: %s %s (%s)\n", sub, decision.Action, decision.TaskID, decision.Reason)
	return nil
}

// sessionDispatcher records a dispatch as ACP evidence and a session lease. It is
// the only mutation surface for a controller step.
func requireBrainStartPreconditions(root, slug string) error {
	config, diagnostics := core.LoadConfig(core.ConfigPaths{Project: filepath.Join(root, "project.yml")}, getenv())
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return fmt.Errorf("load config: %s", diagnostic.Message)
		}
	}
	if !config.Orchestration.Enabled {
		return errors.New("missing precondition: orchestration.enabled must be true")
	}
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return err
	}
	if state.Mode != "orchestrated" {
		return fmt.Errorf("missing precondition: spec mode must be orchestrated (got %q)", state.Mode)
	}
	return nil
}

type sessionDispatcher struct {
	acpPath string
	now     time.Time
	session *orchestration.Session
}

func (d *sessionDispatcher) Dispatch(task core.FrontierTask) error {
	if err := orchestration.AppendACP(d.acpPath, orchestration.ACPEvent{
		Time:   d.now,
		Kind:   "dispatch",
		TaskID: task.ID,
	}); err != nil {
		return err
	}
	d.session.Leases = append(d.session.Leases, orchestration.Lease{
		TaskID:    task.ID,
		WorkerID:  "brain",
		ExpiresAt: d.now.Add(brainLeaseTTL),
	})
	return nil
}
