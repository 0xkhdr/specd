package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	speccontext "github.com/0xkhdr/specd/internal/context"
	"github.com/0xkhdr/specd/internal/core"
)

// DriveEnvelope is the single response of R1.1: everything a host needs to take
// the next legal action, in one call.
//
// It is a projection, not a second source of truth. Every field is derived from
// an existing primitive — the driver guide, the frontier, the session, the
// authority builder, the context manifest — so `drive` can never disagree with
// the granular commands it summarizes (R1.3). Adding a field here means
// projecting an existing value, never computing a new one.
type DriveEnvelope struct {
	ProtocolVersion string `json:"protocol_version"`
	Slug            string `json:"spec_slug"`

	// SessionID is empty when no session is open. That is not an error: it is
	// the state a host is in before `specd session open`, and NextOperation
	// names that as the recovery.
	SessionID string `json:"session_id,omitempty"`
	Driver    string `json:"driver,omitempty"`

	Revision      int64                      `json:"revision"`
	Phase         core.Phase                 `json:"phase"`
	Status        core.Status                `json:"status"`
	Assurance     core.AssuranceLevel        `json:"assurance"`
	RequestMode   core.RequestModeResolution `json:"request_routing"`
	ExecutionMode core.Mode                  `json:"execution_mode"`

	// UnmetControls names each host-contract clause the invoking host did not
	// assert (R5.4). It is why the assurance is what it is, so an operator can
	// see what to fix rather than only that the session is advisory.
	UnmetControls []string `json:"unmet_controls"`

	// PermittedActor is who may act now. It is "human" whenever the only legal
	// forward move is a human-only operation, so a host cannot read a blocked
	// spec as its own turn.
	PermittedActor string `json:"permitted_actor"`

	LegalOperations     []core.NextAction   `json:"legal_operations"`
	HumanOnly           []string            `json:"human_only"`
	Handoffs            []core.RouteHandoff `json:"handoffs"`
	RouteBlockers       []core.RouteBlocker `json:"route_blockers"`
	contextAcknowledged bool
	authorityBound      bool

	SelectedTask *DriveTask        `json:"selected_task,omitempty"`
	Authority    *core.AuthorityV1 `json:"authority,omitempty"`

	ContextManifestDigest string `json:"context_manifest_digest,omitempty"`

	Blockers []core.DriverFinding `json:"blockers"`

	// NextOperation is the exact command to run next — not a description of one.
	NextOperation string `json:"next_operation"`
}

type DriveTask struct {
	ID            string   `json:"id"`
	Role          string   `json:"role"`
	Verify        string   `json:"verify"`
	DeclaredFiles []string `json:"declared_files"`
	Acceptance    string   `json:"acceptance"`
	// Worker/WorkerDisposition project the row's dispatch policy (spec R6.3):
	// host-chooses, a fresh worker, or a continued active worker.
	Worker            string `json:"worker,omitempty"`
	WorkerDisposition string `json:"worker_disposition,omitempty"`
}

func runDrive(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("drive")
	}
	slug := args[0]

	envelope, err := buildDriveEnvelope(root, slug, flagEnabled(flags, "sandbox"), time.Now())
	if err != nil {
		return err
	}
	if flagEnabled(flags, "json") {
		return writeJSON(envelope)
	}
	return renderDrive(os.Stdout, envelope)
}

// buildDriveEnvelope assembles the R1.1 envelope. hostSandbox is what the
// invoking host declares about itself; it can only ever lower the assurance
// level reported, never raise it (R5.4).
func buildDriveEnvelope(root, slug string, hostSandbox bool, now time.Time) (DriveEnvelope, error) {
	spec, err := loadSpec(root, slug)
	if err != nil {
		// R1.2: an unreadable spec is the commonest "not driveable" case, and a
		// bare filesystem error tells a host neither what is wrong nor who fixes
		// it.
		return DriveEnvelope{}, core.Refusef("SPEC_NOT_DRIVEABLE", "spec %s cannot be loaded: %v", slug, err).
			WithRecovery(core.RefusalActorHuman, "specd status --json").Wrapping(err)
	}
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return DriveEnvelope{}, err
	}
	guide, err := driverGuideForSpec(root, slug)
	if err != nil {
		return DriveEnvelope{}, err
	}
	guidance, err := guidanceForSpec(root, slug)
	if err != nil {
		return DriveEnvelope{}, err
	}

	// R5: the assurance reported comes from the host contract, not from the
	// sandbox flag alone. A CLI invocation asserts only what it is told to,
	// so every control it cannot speak for is reported unmet.
	conformance := core.EvaluateHostContract(core.HostContract{Sandbox: hostSandbox})

	envelope := DriveEnvelope{
		ProtocolVersion: core.DriverProtocolVersion,
		Slug:            slug,
		Revision:        state.Revision,
		Phase:           guide.Phase,
		Status:          guide.Status,
		Assurance:       conformance.Assurance,
		UnmetControls:   conformance.Unmet,
		LegalOperations: guide.NextActions,
		HumanOnly:       guidance.HumanOnly,
		Blockers:        guide.Blockers,
		ExecutionMode:   state.Mode,
	}
	envelope.RequestMode, err = core.ResolveRequestMode(core.RequestModeInput{ExplicitDirective: core.RequestModeManaged, SelectedSpec: slug})
	if err != nil {
		return DriveEnvelope{}, err
	}

	session, err := core.LoadDriverSession(core.DriverSessionPath(root, slug))
	if err != nil {
		return DriveEnvelope{}, err
	}
	// An expired session is reported as absent rather than as open: a host that
	// sees an id will use it, and this one would refuse on every operation.
	if session.ID != "" && !session.Expired(now) {
		envelope.SessionID = session.ID
		envelope.Driver = session.Driver
		envelope.contextAcknowledged = session.ContextReceipt != nil && session.ContextReceipt.Complete()
		envelope.authorityBound = session.AuthorityDigest != ""
	}
	route := routeContextForSpec(root, slug, core.RouteCLI, false)
	route.Phase = guide.Phase
	for _, action := range guide.NextActions {
		decision := core.ProjectRoute(action.ID, route)
		if decision.Handoff != nil {
			envelope.Handoffs = append(envelope.Handoffs, *decision.Handoff)
		}
		envelope.RouteBlockers = append(envelope.RouteBlockers, decision.Blockers...)
	}

	// The selected task is the frontier head. Frontier order is deterministic,
	// so two hosts calling `drive` against the same state select the same task.
	if len(guide.Frontier) > 0 {
		if task, ok := findTaskRow(spec.Tasks, guide.Frontier[0]); ok {
			envelope.SelectedTask = &DriveTask{
				ID:                task.ID,
				Role:              task.Role,
				Verify:            task.Verify,
				DeclaredFiles:     append([]string(nil), task.DeclaredFiles...),
				Acceptance:        task.Acceptance,
				Worker:            strings.TrimSpace(task.Worker),
				WorkerDisposition: core.WorkerDisposition(selectedFrontierTask(spec.Tasks, task.ID)),
			}
			if manifest, err := speccontext.BuildManifest(root, slug, spec.Tasks, task.ID, contextBudget(root)); err == nil {
				envelope.ContextManifestDigest = speccontext.ManifestDigest(manifest)
			}
			// The authority packet is reported so a host knows the shape of the
			// grant before requesting it. Reporting is not issuing: it becomes
			// live only once bound to a session (core.BindAuthority).
			//
			// The actor is the session driver when one is open, and the
			// placeholder otherwise — a packet must name an actor to be valid,
			// and a host with no session has not yet identified itself.
			actor := session.Driver
			if actor == "" {
				actor = "unbound"
			}
			config, _ := core.LoadConfig(configPaths(root), getenv())
			authority, err := core.BuildAuthority(task, actor, actor, slug, string(guide.Phase),
				fmt.Sprintf("%d", state.Revision), core.BootstrapHandshake(config).ConfigDigest,
				sandboxProfile(hostSandbox), now, now.Add(core.DriverSessionTTL))
			if err != nil {
				// R1.1 names the authority packet as required. Dropping it
				// silently would hand back an envelope that looks complete and
				// leaves the host with no permissions to derive.
				return DriveEnvelope{}, core.Refusef("AUTHORITY_DENIED", "cannot build an authority packet for task %s: %v", task.ID, err).Wrapping(err)
			}
			envelope.Authority = &authority
		}
	}

	// A machine surface must distinguish "none" from "absent": a nil slice
	// serializes as null, which reads as missing data rather than an empty set.
	if envelope.Blockers == nil {
		envelope.Blockers = []core.DriverFinding{}
	}
	if envelope.LegalOperations == nil {
		envelope.LegalOperations = []core.NextAction{}
	}
	if envelope.HumanOnly == nil {
		envelope.HumanOnly = []string{}
	}
	if envelope.Handoffs == nil {
		envelope.Handoffs = []core.RouteHandoff{}
	}
	if envelope.RouteBlockers == nil {
		envelope.RouteBlockers = []core.RouteBlocker{}
	}

	envelope.PermittedActor, envelope.NextOperation = driveNextStep(envelope, slug)

	// R1.2: a spec with no legal forward move at all is not driveable, and the
	// refusal must name who unblocks it rather than leaving the host to retry.
	if envelope.NextOperation == "" {
		return DriveEnvelope{}, core.Refusef("SPEC_NOT_DRIVEABLE", "spec %s has no legal next operation in phase %s", slug, guide.Phase).
			WithRecovery(core.RefusalActorHuman, "specd status "+slug+" --guide")
	}
	return envelope, nil
}

// driveNextStep picks the actor and the exact command to run next. Order is
// deliberate: a blocker outranks available work, and a human-only gate outranks
// anything an agent could do, so an agent is never told to proceed past a gate
// it cannot clear.
func driveNextStep(e DriveEnvelope, slug string) (actor, operation string) {
	// A blocker's RecoveryAction is prose written for a human reader. R1.1 asks
	// for the exact operation, so drive names the command that re-runs the gates
	// rather than passing the sentence through.
	if len(e.Blockers) > 0 {
		return core.RefusalActorAgent, "specd check " + slug
	}
	agentActions := make([]core.NextAction, 0, len(e.LegalOperations))
	for _, action := range e.LegalOperations {
		if action.Actor == core.RefusalActorAgent {
			agentActions = append(agentActions, action)
		}
	}
	// No session yet: opening one is the precondition for every mutable
	// operation, so it is the next step regardless of what else is legal.
	if e.SessionID == "" && e.SelectedTask != nil {
		return core.RefusalActorAgent, "specd session open " + slug + " --driver <host>"
	}
	if e.SelectedTask != nil && e.SessionID != "" && (!e.contextAcknowledged || !e.authorityBound) {
		return core.RefusalActorAgent, "specd session ack " + slug + " " + e.SelectedTask.ID + " --tokens <n>"
	}
	if e.SelectedTask != nil && e.SessionID != "" && e.authorityBound {
		return core.RefusalActorAgent, "specd session action " + slug + " --json"
	}
	if len(agentActions) == 0 {
		if len(e.Handoffs) > 0 && e.Handoffs[0].Command != "" {
			command := e.Handoffs[0].Command
			if e.Handoffs[0].Actor == core.ActorHuman {
				command += " " + slug
			}
			return string(e.Handoffs[0].Actor), command
		}
		return "", ""
	}
	preferred := agentActions[0]
	// Prefer the operation that advances the selected task over generic
	// inspection: a host that can act should act, not re-read status.
	for _, action := range agentActions {
		if action.Command == "context" || action.Command == "verify.task" || action.Command == "complete-task" {
			preferred = action
			break
		}
	}
	command := preferred.Command
	if operation, ok := core.OperationByID(preferred.ID); ok {
		command = operation.Command
	}
	return core.RefusalActorAgent, strings.TrimSpace("specd " + command + " " + strings.Join(preferred.Args, " "))
}

func sandboxProfile(sandbox bool) string {
	if sandbox {
		return "host-declared"
	}
	return "none"
}

func findTaskRow(tasks []core.TaskRow, id string) (core.TaskRow, bool) {
	for _, task := range tasks {
		if task.ID == id {
			return task, true
		}
	}
	return core.TaskRow{}, false
}

// selectedFrontierTask returns the projected frontier entry for id (spec R6.3),
// so its worker continuation is derived from active-worker state exactly as
// `next` derives it. A task absent from the frontier (or a projection error)
// falls back to a bare entry, so host-chooses/fresh still render.
func selectedFrontierTask(tasks []core.TaskRow, id string) core.FrontierTask {
	frontier, err := core.Frontier(tasks, taskStatus(tasks))
	if err == nil {
		for _, ft := range frontier {
			if ft.ID == id {
				return ft
			}
		}
	}
	row, _ := findTaskRow(tasks, id)
	return core.FrontierTask{ID: id, Worker: strings.TrimSpace(row.Worker)}
}

func renderDrive(out *os.File, e DriveEnvelope) error {
	fmt.Fprintf(out, "spec %s (revision %d)\nphase %s (status %s)\nassurance %s\n", e.Slug, e.Revision, e.Phase, e.Status, e.Assurance)
	if e.SessionID != "" {
		fmt.Fprintf(out, "session %s (driver %s)\n", e.SessionID, e.Driver)
	} else {
		fmt.Fprintln(out, "session none")
	}
	if e.SelectedTask != nil {
		fmt.Fprintf(out, "task %s (role %s)\n  files %s\n  verify %s\n  dispatch %s\n",
			e.SelectedTask.ID, e.SelectedTask.Role, strings.Join(e.SelectedTask.DeclaredFiles, ", "), e.SelectedTask.Verify, e.SelectedTask.WorkerDisposition)
	}
	for _, blocker := range e.Blockers {
		fmt.Fprintf(out, "blocker %s: %s\n", blocker.Code, blocker.Message)
	}
	fmt.Fprintf(out, "actor %s\nnext %s\n", e.PermittedActor, e.NextOperation)
	return nil
}
