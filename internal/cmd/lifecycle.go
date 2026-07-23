package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
	corescope "github.com/0xkhdr/specd/internal/core/scope"
	"github.com/0xkhdr/specd/internal/orchestration"
)

var taskCompleteAtomicWrite = core.AtomicWrite

// runNew creates a spec workspace: requirements.md, design.md, tasks.md, and state.json at
// revision 0 (R13.3). Creation is a fresh write under the per-spec lock, not a
// compare-and-swap; SaveStateCAS with expected revision 0 would ratchet to 1.
func runNew(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("new")
	}
	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return err
	}
	title := strings.TrimSpace(flags["title"])
	if title == "" {
		title = slug
	}
	if strings.ContainsAny(title, "\r\n") {
		return errors.New("usage: --title must be one line")
	}
	specDir := filepath.Join(core.SpecdDir(root), "specs", slug)
	statePath := core.StatePath(root, slug)
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		if _, err := os.Stat(statePath); err == nil {
			return struct{}{}, fmt.Errorf("spec %q already exists", slug)
		}
		if err := os.MkdirAll(specDir, 0o755); err != nil {
			return struct{}{}, err
		}
		if err := core.AtomicWrite(filepath.Join(specDir, "requirements.md"), requirementsStub(title)); err != nil {
			return struct{}{}, err
		}
		if err := core.AtomicWrite(filepath.Join(specDir, "design.md"), designStub(title)); err != nil {
			return struct{}{}, err
		}
		if err := core.AtomicWrite(filepath.Join(specDir, "tasks.md"), tasksStub(title)); err != nil {
			return struct{}{}, err
		}
		if err := core.AtomicWrite(core.SpecMemoryPath(root, slug), memoryStub(slug)); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, core.SaveState(statePath, core.InitialState(slug))
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "created spec %s at %s\n", slug, specDir)
	return nil
}

// runApprove refuses the gate transition when readiness gates emit errors and
// leaves state untouched; on green it ratchets the phase and appends an
// approval record via CAS (R13.4).
func runApprove(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("approve")
	}
	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return err
	}
	return approveSpec(root, slug, nil)
}

// delegatedApproval is the delegation identity a delegated approval records. It
// is nil for interactive approval, and that nil is the *only* difference
// between the two paths: both run approveSpec, so an operator's grant can never
// advance a spec that a human standing at the same commit could not (R3.1).
type delegatedApproval struct {
	GrantID   string
	RequestID string
	Reason    string
	Actor     core.ActorContext
	// ExpectedRevision is the revision the grant use was authorized against.
	// The reservation is taken before the spec lock, so the approval refuses
	// rather than committing against a state that moved underneath it.
	ExpectedRevision int64
}

// approvalGateFor is the gate name an approval of current records under. Shared
// so the delegated path names the same gate the interactive path will write,
// and the grant's transition scope is checked against the real one.
func approvalGateFor(current core.Status) string {
	if core.NextStatus(current) == core.StatusComplete {
		return string(core.StatusComplete)
	}
	return string(current)
}

// approveSpec is the one approval transaction. Interactive approval and
// delegated approval differ only in the audit they record and in the grant
// bookkeeping the caller wraps around it — never in which gates run or in what
// a passing gate set permits.
func approveSpec(root, slug string, delegated *delegatedApproval) error {
	var approvedFrom, approvedTarget core.Status
	var approvedPlan string
	var approvedRevision int64
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		statePath := core.StatePath(root, slug)
		state, err := core.LoadState(statePath)
		if err != nil {
			return struct{}{}, err
		}
		current := state.Status
		if delegated != nil && state.Revision != delegated.ExpectedRevision {
			return struct{}{}, core.Refusef("REVISION_CONFLICT", "delegated approval authorized revision %d, spec %s is at %d",
				delegated.ExpectedRevision, slug, state.Revision)
		}
		if current == core.StatusExecuting {
			events, err := core.ReadWorkflowEvents(core.WorkflowEventPath(root, slug))
			if err != nil {
				return struct{}{}, err
			}
			reopened := false
			for _, event := range events {
				if strings.HasPrefix(event.Transition, core.ReopenSpecTransitionPrefix) {
					reopened = true
					break
				}
			}
			if reopened {
				spec, err := loadSpec(root, slug)
				if err != nil {
					return struct{}{}, err
				}
				states, err := core.ProjectTaskStates(spec.Tasks, state.TaskStatus, core.ReopenTaskFacts(events, nil))
				if err != nil {
					return struct{}{}, err
				}
				if pending := core.PendingCompletionBlockers(states); len(pending) > 0 {
					return struct{}{}, core.Refusef("GATE_FAILED",
						"pending-completion gate: tasks %s are pending; complete them or record an accepted terminal disposition before approving executing",
						strings.Join(pending, ", ")).
						WithRecovery(core.RefusalActorAgent, "specd status "+slug+" --guide")
				}
			}
		}
		readiness, err := buildReadiness(root, slug, state)
		if err != nil {
			return struct{}{}, err
		}
		plan := readiness.Envelope.Plan
		if plan.Terminal {
			return struct{}{}, core.Refusef("NO_SUCCESSOR", "status %q has no lifecycle successor", current).
				WithContext(slug, string(current), "a legal lifecycle successor").
				WithSuccessor(core.RefusalActorAgent, "new", "specd new <successor>")
		}
		target := plan.Target
		approvedPlan, approvedRevision = plan.PlanDigest, plan.StateRevision
		phase, err := core.AdvanceStatus(current, target)
		if err != nil {
			return struct{}{}, err
		}
		approvedFrom, approvedTarget = current, target
		gate := approvalGateFor(current)
		if gates.HasErrors(readiness.Findings) {
			for _, finding := range readiness.Findings {
				if finding.Severity == gates.Error {
					fmt.Fprintf(os.Stderr, "%s %s: %s\n", finding.Severity, finding.Gate, finding.Message)
				}
			}
			return struct{}{}, readinessRefusal(slug, readiness, false)
		}
		state.Status = target
		state.Phase = phase
		rec := core.Record{Kind: "approval", Gate: gate, Text: fmt.Sprintf("%s → %s", current, target), ApprovedRevision: state.Revision}
		if delegated != nil {
			// Scope marks the approval delegated on the record every reader
			// already reads, and the companion record carries the grant
			// identity. A reader that ignores both still sees "scope=delegated"
			// on the approval itself, so a delegated approval can never be
			// mistaken for a human one (R3.4, R6.3).
			rec.Scope = "delegated"
			if err := appendRecord(root, &state, "delegation:"+gate, core.Record{
				Kind:             "delegation",
				Gate:             gate,
				Scope:            delegated.GrantID,
				Text:             delegated.audit(),
				ApprovedRevision: state.Revision,
			}); err != nil {
				return struct{}{}, err
			}
		}
		// Pin the approved artifact's source digest so a later amendment can
		// detect drift (spec 01 R2.1 "and digest", R5 staleness).
		if artifact := approvalArtifact(gate); artifact != "" {
			if b, err := os.ReadFile(filepath.Join(core.SpecdDir(root), "specs", slug, artifact)); err == nil {
				rec.SourceDigest = core.Digest(b)
			}
		}
		if err := appendRecord(root, &state, "approval:"+gate, rec); err != nil {
			return struct{}{}, err
		}
		if err := recordCompatibilityApproval(root, &state, gate, core.ApprovalPins{
			ArtifactDigest: approvalArtifactDigest(rec.SourceDigest),
			StateRevision:  plan.StateRevision,
			PlanDigest:     plan.PlanDigest,
			ConfigDigest:   plan.ConfigDigest,
		}); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, core.SaveStateCAS(statePath, state.Revision, state)
	})
	if err != nil {
		return err
	}
	if delegated != nil {
		fmt.Fprintf(os.Stdout, "approved %s (delegated): %s → %s revision %d plan %s grant %s\n",
			slug, approvedFrom, approvedTarget, approvedRevision, approvedPlan, delegated.GrantID)
		return nil
	}
	fmt.Fprintf(os.Stdout, "approved %s: %s → %s revision %d plan %s\n", slug, approvedFrom, approvedTarget, approvedRevision, approvedPlan)
	return nil
}

func runMode(root string, args []string, flags map[string]string) error {
	if len(args) == 1 {
		if err := core.ValidateSlug(args[0]); err != nil {
			return err
		}
		state, err := core.LoadState(core.StatePath(root, args[0]))
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "mode: %s\n", state.Mode)
		return nil
	}
	if len(args) != 2 || args[1] != string(core.ModeOrchestrated) {
		return usageError("mode")
	}
	if err := core.ValidateSlug(args[0]); err != nil {
		return err
	}
	return runApproveOrchestrated(root, args[0])
}

// runApproveOrchestrated is the supported human-only transition into the
// opt-in controller mode. Configuration arms orchestration; this approval
// records human intent and changes mode under the same per-spec lock and state
// revision CAS. Refused transitions write nothing.
func runApproveOrchestrated(root, slug string) error {
	config, diagnostics := core.LoadConfig(configPaths(root), getenv())
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return fmt.Errorf("load config: %s", diagnostic.Message)
		}
	}
	if !config.Orchestration.Enabled {
		return errors.New("approve orchestrated refused: orchestration.enabled must be true")
	}

	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		statePath := core.StatePath(root, slug)
		state, err := core.LoadState(statePath)
		if err != nil {
			return struct{}{}, err
		}
		if state.Mode == core.ModeOrchestrated {
			return struct{}{}, errors.New("approve orchestrated refused: spec mode is already orchestrated")
		}
		rec := core.Record{Kind: "approval", Gate: string(core.ModeOrchestrated), ApprovedRevision: state.Revision}
		if err := appendRecord(root, &state, "approval:orchestrated", rec); err != nil {
			return struct{}{}, err
		}
		state.Mode = core.ModeOrchestrated
		return struct{}{}, core.SaveStateCAS(statePath, state.Revision, state)
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "approved %s → orchestrated mode\n", slug)
	return nil
}

// runTaskComplete marks a task complete: it requires a current passing evidence
// record pinned to HEAD, then writes the ✅ marker to
// tasks.md and the machine-truth status to state.json under one lock+CAS so the
// two never drift (the Sync gate enforces that agreement).
func runTaskComplete(root string, args []string, flags map[string]string) error {
	if len(args) != 2 {
		return usageError("complete-task")
	}
	slug, id := args[0], args[1]
	if err := core.ValidateSlug(slug); err != nil {
		return err
	}
	annotations, err := parseAnnotations(flags)
	if err != nil {
		return err
	}
	_, err = core.WithSpecLock(root, func() (struct{}, error) {
		statePath := core.StatePath(root, slug)
		state, err := core.LoadState(statePath)
		if err != nil {
			return struct{}{}, err
		}
		spec, err := loadSpec(root, slug)
		if err != nil {
			return struct{}{}, err
		}
		tasksPath := filepath.Join(core.SpecdDir(root), "specs", slug, "tasks.md")
		raw, err := os.ReadFile(tasksPath)
		if err != nil {
			return struct{}{}, err
		}
		var task core.TaskRow
		found := false
		for _, row := range spec.Tasks {
			if row.ID == id {
				task, found = row, true
				break
			}
		}
		if !found {
			return struct{}{}, fmt.Errorf("task %s not found", id)
		}
		evidence, ok := spec.Evidence[id]
		if !ok {
			return struct{}{}, taskEvidenceRefusal("EVIDENCE_MISSING", slug, id, "no task verify record", "passing evidence pinned to current HEAD", nil)
		}
		if evidence.ExitCode != 0 {
			return struct{}{}, taskEvidenceRefusal("EVIDENCE_FAILING", slug, id, fmt.Sprintf("verify exit_code=%d", evidence.ExitCode), "verify exit_code=0 at current HEAD", evidence)
		}
		if !core.HeadPinned(evidence.GitHead) {
			return struct{}{}, taskEvidenceRefusal("EVIDENCE_STALE", slug, id, fmt.Sprintf("unresolvable git_head=%q", evidence.GitHead), "passing evidence pinned to current HEAD", evidence)
		}
		currentHead := gitHead(root)
		if !core.HeadPinned(currentHead) || evidence.GitHead != currentHead {
			return struct{}{}, taskEvidenceRefusal("EVIDENCE_STALE", slug, id, fmt.Sprintf("verified HEAD %s; current HEAD %s", evidence.GitHead, currentHead), "evidence pinned to current HEAD", evidence)
		}
		contract, err := core.ParseQualityContract(task)
		if err != nil {
			return struct{}{}, err
		}
		evals, err := core.LoadEvals(core.EvalStorePath(root, slug))
		if err != nil {
			return struct{}{}, err
		}
		if refusal := qualityEvidenceRefusal(slug, id, contract, evals, currentHead); refusal != nil {
			return struct{}{}, refusal
		}
		// Validate bindings before the non-mutating gates, but spend the nonce
		// only once those gates pass.
		if err := validateSessionBinding(root, slug, id, state, flags, time.Now()); err != nil {
			return struct{}{}, err
		}
		// R4.5: diff-scope is a core invariant, not a production-profile extra.
		// It runs here on every transport and every profile; what varies is
		// whether a baseline was ever pinned, not whether the rule applies.
		if err := enforceDiffScope(root, slug, id, task); err != nil {
			return struct{}{}, err
		}
		// Escalation ratchet (spec 06 R2): a task blocked by N consecutive verify
		// failures cannot complete until a human override resets the counter. The
		// override is not a bypass — CompleteTask below still demands a passing
		// verify record.
		if count, err := taskFailCount(root, slug, id); err != nil {
			return struct{}{}, err
		} else if core.IsEscalated(count, escalationMaxFails(root)) {
			return struct{}{}, fmt.Errorf("task %s is escalated after %d consecutive verify failures; clear it with `specd task %s --override --reason <text>` first", id, count, id)
		}
		if err := enforceSessionBinding(root, slug, id, state, flags, time.Now()); err != nil {
			return struct{}{}, err
		}
		updated, err := core.CompleteTaskWithQuality(raw, id, spec.Evidence, contract, evals, core.FreshnessSubject{Revision: currentHead})
		if err != nil {
			return struct{}{}, err
		}
		rollbackState, err := core.LoadState(statePath)
		if err != nil {
			return struct{}{}, err
		}
		if state.TaskStatus == nil {
			state.TaskStatus = map[string]core.TaskRunStatus{}
		}
		state.TaskStatus[id] = core.TaskComplete
		if err := core.SaveStateCAS(statePath, state.Revision, state); err != nil {
			return struct{}{}, err
		}
		if err := taskCompleteAtomicWrite(tasksPath, string(updated)); err != nil {
			writeErr := err
			if rollbackErr := core.SaveStateCAS(statePath, state.Revision+1, rollbackState); rollbackErr != nil {
				return struct{}{}, fmt.Errorf("completion tasks write failed: %v; state rollback failed: %w", writeErr, rollbackErr)
			}
			return struct{}{}, writeErr
		}
		// Optional telemetry (spec 10 R1): completion carries the worker's
		// verbatim cost as a supplementary evidence record. CompleteTask already
		// required a passing verify record above, so this record only annotates —
		// it never manufactures passing evidence.
		if annotations != nil {
			rec := core.EvidenceRecord{TaskID: id, Command: "complete-task", ExitCode: 0, GitHead: currentHead, Telemetry: annotations}
			if err := core.AppendEvidence(core.EvidencePath(root, slug), rec); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "completed %s %s\n", slug, id)
	return nil
}

func taskEvidenceRefusal(code, slug, taskID, observed, expected string, evidence any) error {
	detail := fmt.Sprintf("task %s: %s", taskID, observed)
	if code == "EVIDENCE_MISSING" {
		detail = fmt.Sprintf("task %s requires passing evidence pinned to current HEAD: %s", taskID, observed)
	} else if code == "EVIDENCE_STALE" {
		detail = fmt.Sprintf("task %s evidence is stale: %s", taskID, observed)
	}
	refusal := core.Refuse(code, detail).
		WithContext(slug+"/"+taskID, observed, expected).
		WithRecovery(core.RefusalActorAgent, "specd verify "+slug+" "+taskID)
	if evidence != nil {
		raw, _ := json.Marshal(evidence)
		refusal = refusal.WithInput("evidence.jsonl", raw)
	}
	return refusal
}

func qualityEvidenceRefusal(slug, taskID string, contract core.QualityContract, evals []core.EvidenceEnvelopeV1, head string) error {
	status := core.EvaluateQuality(contract, evals, core.FreshnessSubject{Revision: head})
	var failing []string
	var failingRequirements []core.EvidenceRequirement
	for _, requirement := range append(append([]core.EvidenceRequirement{}, status.Missing...), status.Stale...) {
		for i := len(evals) - 1; i >= 0; i-- {
			record := evals[i]
			if record.TaskID == taskID && record.EvidenceClass == requirement.EvidenceClass && record.CheckID == requirement.CheckID {
				if record.Verdict != core.EvalPass {
					failing = append(failing, string(requirement.EvidenceClass)+"/"+requirement.CheckID+"="+string(record.Verdict))
					failingRequirements = append(failingRequirements, requirement)
				}
				break
			}
		}
	}
	code, observed, requirements := "", "", status.Missing
	switch {
	case len(failing) > 0:
		code, observed, requirements = "EVIDENCE_FAILING", strings.Join(failing, ","), failingRequirements
	case len(status.Missing) > 0:
		code, observed = "EVIDENCE_MISSING", core.FormatRequirements(status.Missing)
	case len(status.Stale) > 0:
		code, observed, requirements = "EVIDENCE_STALE", core.FormatRequirements(status.Stale), status.Stale
	default:
		return nil
	}
	command := "specd verify " + slug + " " + taskID
	if len(requirements) > 0 && requirements[0].EvidenceClass != core.EvidenceTest {
		command = "specd eval import " + slug + " <workspace-relative-file> --task " + taskID + " --check " + requirements[0].CheckID
	}
	raw, _ := json.Marshal(evals)
	return core.Refusef(code, "task %s evidence %s", taskID, observed).
		WithContext(slug+"/"+taskID, observed, "fresh passing evidence for "+core.FormatRequirements(requirements)).
		WithInput("eval evidence", raw).
		WithRecovery(core.RefusalActorAgent, command)
}

// runSpike records a bounded exploratory-learning spike (spec 01 R7.3). It
// appends a spike record under the per-spec lock+CAS and touches no lifecycle
// status, task status, or approval record. Completion still demands a passing
// verify record (CompleteTask) and architecture still demands a human design
// approval — a spike is a distinct record kind neither path reads, so recording
// one can never complete a task or approve architecture. Required-field and
// bound enforcement lives in core.Spike.Validate (via AppendSpike).
func runSpike(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("spike")
	}
	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return err
	}
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		statePath := core.StatePath(root, slug)
		state, err := core.LoadState(statePath)
		if err != nil {
			return struct{}{}, err
		}
		spike := core.StampSpike(core.Spike{
			Question:  strings.TrimSpace(flags["question"]),
			Scope:     strings.TrimSpace(flags["scope"]),
			Expiry:    strings.TrimSpace(flags["expiry"]),
			OutputRef: strings.TrimSpace(flags["output"]),
		}, gitHead(root))
		if err := state.AppendSpike(spike); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, core.SaveStateCAS(statePath, state.Revision, state)
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "recorded spike for %s\n", slug)
	return nil
}

func runMidreq(root string, args []string, flags map[string]string) error {
	return appendScoped(root, args, flags, "midreq", "usage: specd midreq <spec> --text <text> [--scope <scope>]")
}

func runDecision(root string, args []string, flags map[string]string) error {
	return appendScoped(root, args, flags, "decision", "usage: specd decision <spec> --text <text> [--scope <scope>]")
}

// appendScoped appends a scoped record to state via CAS without touching
// unrelated core fields (R13.5). --text is required (R3.1): a decision or
// midreq gate that records nothing observes nothing. --scope is optional.
func appendScoped(root string, args []string, flags map[string]string, kind, usage string) error {
	if len(args) != 1 {
		return errors.New(usage)
	}
	text := strings.TrimSpace(flags["text"])
	if text == "" {
		return errors.New(usage)
	}
	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return err
	}
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		statePath := core.StatePath(root, slug)
		state, err := core.LoadState(statePath)
		if err != nil {
			return struct{}{}, err
		}
		key := fmt.Sprintf("%s:%d", kind, countPrefix(state.Records, kind+":"))
		if err := appendRecord(root, &state, key, core.Record{Kind: kind, Text: text, Scope: flags["scope"]}); err != nil {
			return struct{}{}, err
		}
		if kind == "midreq" {
			affected := splitScope(flags["scope"])
			if len(affected) == 0 {
				affected = []string{"requirements"}
			}
			// Mid-course changes conservatively invalidate downstream intent;
			// later re-approval can narrow the active contract again.
			affected = appendUnique(affected, "design", "tasks")
			amendment := core.StampAmendment(core.Amendment{
				ChangeID:         fmt.Sprintf("midreq-%d", countPrefix(state.Records, "amendment:")),
				AffectedIDs:      affected,
				Rationale:        text,
				RequiredRechecks: []string{"design", "tasks", "execution"},
			}, gitHead(root))
			if err := state.AppendAmendment(amendment); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, core.SaveStateCAS(statePath, state.Revision, state)
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "recorded %s for %s\n", kind, slug)
	return nil
}

func splitScope(scope string) []string {
	var ids []string
	for _, value := range strings.Split(scope, ",") {
		if value = strings.TrimSpace(value); value != "" {
			ids = appendUnique(ids, value)
		}
	}
	return ids
}

func appendUnique(values []string, extra ...string) []string {
	seen := make(map[string]bool, len(values)+len(extra))
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range extra {
		if !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}

// runHelp renders usage from core.Commands metadata; --json is machine-readable
// (R13.9).
func runHelp(root string, args []string, flags map[string]string) error {
	if len(args) > 1 {
		return errors.New("usage: specd help [command] [--json]")
	}
	if len(args) == 1 {
		command, ok := findCommand(args[0])
		if !ok {
			return fmt.Errorf("unknown command %q", args[0])
		}
		if flagEnabled(flags, "json") {
			return writeJSON(command)
		}
		fmt.Fprintf(os.Stdout, "%s\n  %s\n", command.Usage, command.Description)
		for _, flag := range command.Flags {
			fmt.Fprintln(os.Stdout, flagHelpLine(flag))
		}
		return nil
	}
	if flagEnabled(flags, "json") {
		return writeJSON(core.BuildHelpPayload())
	}
	fmt.Fprintln(os.Stdout, "usage: specd <command> [args] [--flag value|--flag=value]")
	for _, command := range core.Commands {
		fmt.Fprintf(os.Stdout, "  %-10s %s\n", command.Name, command.Description)
	}
	return nil
}

// runTask prints the parsed task row matching id across the project's specs
// (R13.9).
func runTask(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("task")
	}
	id := args[0]
	if flagEnabled(flags, "override") {
		return runTaskOverride(root, id, flags)
	}
	entries, err := os.ReadDir(filepath.Join(core.SpecdDir(root), "specs"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		spec, err := loadSpec(root, entry.Name())
		if err != nil {
			continue
		}
		for _, task := range spec.Tasks {
			if task.ID == id {
				if flagEnabled(flags, "json") {
					return writeJSON(task)
				}
				fmt.Fprintf(os.Stdout, "%s [%s] %s\n", task.ID, entry.Name(), task.Role)
				fmt.Fprintf(os.Stdout, "  files:      %s\n", task.Files)
				fmt.Fprintf(os.Stdout, "  depends-on: %s\n", strings.Join(task.DependsOn, ", "))
				fmt.Fprintf(os.Stdout, "  verify:     %s\n", task.Verify)
				fmt.Fprintf(os.Stdout, "  acceptance: %s\n", task.Acceptance)
				return nil
			}
		}
	}
	return fmt.Errorf("task %s not found", id)
}

// appendRecord stamps rec with the provenance triple (timestamp/git_head/actor
// via core.StampRecord) and stores it under key. Every record kind routes
// through here, so no record reaches the ledger unstamped.
func appendRecord(root string, state *core.State, key string, rec core.Record) error {
	raw, err := json.Marshal(core.StampRecord(rec, gitHead(root)))
	if err != nil {
		return err
	}
	if state.Records == nil {
		state.Records = map[string]json.RawMessage{}
	}
	state.Records[key] = raw
	return nil
}

// approvalArtifact maps an approval gate to the spec artifact whose bytes it
// pins into the approval record (spec 01 R2.1). Gates without a source artifact
// (tasks/executing/verifying/complete) pin nothing.
func approvalArtifact(gate string) string {
	switch core.Status(gate) {
	case core.StatusRequirements:
		return "requirements.md"
	case core.StatusDesign:
		return "design.md"
	}
	return ""
}

// approvalRequestTTL bounds the compatibility request opened by `specd approve`.
// The request is opened and answered inside one invocation, so the window only
// has to outlive that call.
const approvalRequestTTL = time.Hour

// approvalArtifactDigest is the artifact identity a compatibility request pins.
// Gates that govern no source artifact (tasks/executing/verifying/complete) pin
// the explicit "none" so the pin is still a value that can be compared for
// drift rather than an empty field (R5.1).
func approvalArtifactDigest(sourceDigest string) string {
	if sourceDigest == "" {
		return "none"
	}
	return sourceDigest
}

// recordCompatibilityApproval keeps the interactive `specd approve` on the
// immutable request model (R5.4): it reuses a request that is still open for
// this gate — so the inputs it pinned are checked against current and refuse as
// stale when they drifted (R5.3) — and otherwise opens one pinned to the
// current identities, then appends the approved transition. Both transitions
// are separate append-only records; neither edits the other.
func recordCompatibilityApproval(root string, state *core.State, gate string, pins core.ApprovalPins) error {
	id := core.ApprovalRequestID(gate, state.Cycle)
	existing, err := state.ApprovalRequests()
	if err != nil {
		return err
	}
	if !core.ApprovalRequestPending(existing, id) {
		if err := appendApprovalRequest(root, state, core.ApprovalRequestRecord{
			ID:            id,
			Transition:    core.ApprovalRequested,
			EntityKind:    core.ApprovalEntitySpec,
			EntityID:      state.Slug,
			EntityVersion: gate,
			Pins:          pins,
			ExpiresAt:     core.Clock().Add(approvalRequestTTL).Format(time.RFC3339),
		}); err != nil {
			return err
		}
	}
	return appendApprovalRequest(root, state, core.ApprovalRequestRecord{ID: id, Transition: core.ApprovalApproved, Pins: pins})
}

// appendApprovalRequest stamps rec, validates the transition against the chain
// already in state, and stores it under the key core.PlanApprovalRequest chose.
// Keys are never reused, so an approval can only add history.
func appendApprovalRequest(root string, state *core.State, rec core.ApprovalRequestRecord) error {
	existing, err := state.ApprovalRequests()
	if err != nil {
		return err
	}
	rec = core.StampApprovalRequest(rec, gitHead(root))
	rec.Requester = rec.Actor
	key, planned, err := core.PlanApprovalRequest(existing, rec)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(planned)
	if err != nil {
		return err
	}
	if state.Records == nil {
		state.Records = map[string]json.RawMessage{}
	}
	state.Records[key] = raw
	return nil
}

func countPrefix(records map[string]json.RawMessage, prefix string) int {
	count := 0
	for key := range records {
		if strings.HasPrefix(key, prefix) {
			count++
		}
	}
	return count
}

func findCommand(name string) (core.Command, bool) {
	for _, command := range core.Commands {
		if command.Name == name {
			return command, true
		}
	}
	return core.Command{}, false
}

func memoryStub(slug string) string {
	return fmt.Sprintf("# Memory — %s\n\n> Steering-memory patterns. Append with `specd memory %s add`.\n", slug, slug)
}

func requirementsStub(slug string) string {
	return core.RequirementsScaffold(slug)
}

func designStub(slug string) string {
	return fmt.Sprintf("# Design — %s\n\n"+
		"> Replace prompts. Trace every decision to approved requirement IDs.\n\n"+
		"- references: <R1, R1.1>\n- disposition: <accepted|deferred|rejected>\n- owner: <human decision owner>\n\n"+
		"## Boundaries\n\n- <owned modules and excluded responsibilities>\n\n"+
		"## Interfaces\n\n- <API, file, or protocol contracts>\n\n"+
		"## Invariants\n\n- <property preserved across success and failure>\n\n"+
		"## Failure\n\n- <failure mode, containment, recovery>\n\n"+
		"## Integration\n\n- <dependency and compatibility behavior>\n\n"+
		"## Alternatives\n\n- <option and reason accepted/rejected/deferred>\n\n"+
		"## Verification\n\n- <proof for each invariant and interface>\n\n"+
		"## Deployment\n\n- <rollout, observation, ownership>\n\n"+
		"## Rollback\n\n- <trigger and safe restoration path>\n", slug)
}

func tasksStub(slug string) string {
	return core.TasksScaffold(slug)
}

// enforceDiffScope compares the whole worktree diff against the task's declared
// scope (R4.1 to R4.4) and runs on every transport and profile (R4.5).
//
// Baseline resolution is graduated, because what a baseline means depends on how
// the work was dispatched:
//
//   - a brain mission pins one explicitly (the production path);
//   - otherwise the driver session's git HEAD at open bounds the work;
//   - otherwise nothing pinned the work at all.
//
// The last case proceeds rather than refusing. There is no reference point to
// measure against, so a refusal would be arbitrary rather than earned — and a
// session that pinned nothing is already reported as advisory (R5.4), which is
// the honest label for work the harness did not bound. This is the one place
// the check yields, and it yields to absence of evidence, never to a flag: no
// configuration, role, or argument can switch the rule off when a baseline does
// exist.
func enforceDiffScope(root, slug, id string, task core.TaskRow) error {
	baseline := missionBaseline(root, slug, id)
	var session core.DriverSession
	if baseline == "" {
		var err error
		session, err = core.LoadDriverSession(core.DriverSessionPath(root, slug))
		if err != nil {
			return err
		}
		baseline = session.BaselineHead
	}
	if baseline == "" {
		// The production profile has always refused an unpinned task, and R4.5
		// asks for the check to run everywhere — not for production to start
		// accepting what it previously rejected. The graduated behaviour below
		// is for the default profile only.
		if loadSpecConfig(root).ProductionTaskAuthorityRequired() {
			return core.Refusef("BASELINE_UNPINNED", "task %s has no pinned baseline; dispatch a fresh mission or open a driver session", id).
				WithRecovery(core.RefusalActorAgent, "specd session open "+slug+" --driver <host>")
		}
		return nil
	}

	// The full diff, deliberately not core.DeriveDiff: that helper strips
	// .specd/ paths, which is exactly the class R4.3 must reject.
	diff, err := corescope.Derive(root, baseline)
	if err != nil {
		return core.Refusef("BASELINE_UNRESOLVABLE", "cannot derive the diff for task %s against baseline %s: %v", id, baseline, err).
			WithRecovery(core.RefusalActorAgent, "specd session open "+slug+" --driver <host>").Wrapping(err)
	}
	changes := diff.Changes
	if session.BaselineHead == baseline {
		changes = slices.DeleteFunc(changes, func(change corescope.Change) bool {
			return change.Kind == "untracked" && slices.Contains(session.PreexistingUntracked, change.Path)
		})
	}
	if attributable, err := attributableTasksMarker(root, slug, baseline); err != nil {
		return err
	} else if attributable {
		changes = slices.DeleteFunc(changes, func(change corescope.Change) bool {
			return change.Path == filepath.Join(".specd", "specs", slug, "tasks.md")
		})
	}

	findings := gates.CheckDiffScope(gates.DiffScopeInput{
		TaskID:             id,
		Baseline:           baseline,
		Changes:            changes,
		DeclaredPaths:      task.DeclaredFiles,
		BaselineIsAncestor: isAncestor(root, baseline),
		BaselineResolvable: true,
		OtherLeaseScopes:   otherLeaseScopes(root, slug, id),
	})
	if len(findings) == 0 {
		return nil
	}
	messages := make([]string, 0, len(findings))
	for _, finding := range findings {
		messages = append(messages, finding.Message)
	}
	// No bypass is offered: the recovery is to declare the path or narrow the
	// change, which is a task edit, not a flag.
	return core.Refusef("OUTSIDE_SCOPE", "task %s changed files outside its declared scope:\n  %s", id, strings.Join(messages, "\n  ")).
		WithRecovery(core.RefusalActorAgent, "specd status "+slug+" --guide")
}

func attributableTasksMarker(root, slug, baseline string) (bool, error) {
	path := filepath.Join(".specd", "specs", slug, "tasks.md")
	baselineRaw, err := exec.Command("git", "-C", root, "show", baseline+":"+path).Output()
	if err != nil {
		return false, nil
	}
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	expected := baselineRaw
	for taskID, status := range state.TaskStatus {
		if status != core.TaskComplete {
			continue
		}
		expected, err = core.RewriteTaskStatusLine(expected, taskID, "✅")
		if err != nil {
			return false, err
		}
	}
	current, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return false, err
	}
	return slices.Equal(current, expected), nil
}

// missionBaseline returns the SubjectHead a brain mission pinned for this task,
// or "" when the brain is not driving it.
func missionBaseline(root, slug, id string) string {
	session, err := orchestration.LoadSession(filepath.Join(core.SpecdDir(root), "specs", slug, "session.json"))
	if err != nil {
		return ""
	}
	baseline := ""
	for _, mission := range append(session.Missions, session.PendingMissions...) {
		if mission.TaskID == id {
			baseline = mission.SubjectHead
		}
	}
	return baseline
}

// otherLeaseScopes maps every active lease except this task's to the paths it
// holds, so an overlapping write is refused rather than racing (R4.4).
func otherLeaseScopes(root, slug, id string) map[string][]string {
	session, err := orchestration.LoadSession(filepath.Join(core.SpecdDir(root), "specs", slug, "session.json"))
	if err != nil {
		return nil
	}
	scopes := map[string][]string{}
	for _, lease := range session.Leases {
		// Only an active lease can race this write. A revoked or expired one
		// holds nothing, and treating it as an overlap would refuse legitimate
		// work after every failed attempt.
		if lease.TaskID == id || lease.State != orchestration.LeaseActive {
			continue
		}
		for _, mission := range append(session.Missions, session.PendingMissions...) {
			if mission.TaskID == lease.TaskID && len(mission.DeclaredFiles) > 0 {
				scopes[lease.LeaseID] = mission.DeclaredFiles
			}
		}
	}
	if len(scopes) == 0 {
		return nil
	}
	return scopes
}

// isAncestor reports whether baseline is an ancestor of HEAD. A false answer
// means the worktree carries history predating the mission baseline (R4.2).
func isAncestor(root, baseline string) bool {
	return exec.Command("git", "-C", root, "merge-base", "--is-ancestor", baseline, "HEAD").Run() == nil
}

// enforceSessionBinding is the live enforcement of R2.2, R2.3, R2.4, and R3.2
// on a mutable operation.
//
// Graduated like the diff-scope check above, and for the same reason: an
// operator driving specd by hand has no session, and refusing them would make
// the harness unusable outside a governed host. But the moment a session IS
// open, the host has declared itself governed, and every binding the protocol
// defines is then required. There is no flag to turn this off — closing the
// session is the only way out, and that is a visible act rather than a hidden
// one.
//
// The nonce is spent here, inside the same spec lock the caller holds, so two
// concurrent operations cannot both observe it unspent.
func enforceSessionBinding(root, slug, id string, state core.State, flags map[string]string, now time.Time) error {
	binding, session, err := sessionBinding(root, slug, id, state, flags, now)
	if err != nil || session.ID == "" {
		return err
	}
	if _, err := core.SpendNonce(root, slug, binding, state.Revision, now); err != nil {
		recordConformanceForRefusal(root, slug, id, err)
		return err
	}
	return nil
}

func validateSessionBinding(root, slug, id string, state core.State, flags map[string]string, now time.Time) error {
	binding, session, err := sessionBinding(root, slug, id, state, flags, now)
	if err != nil || session.ID == "" {
		return err
	}
	return session.ValidateOperation(binding, state.Revision, now)
}

func sessionBinding(root, slug, id string, state core.State, flags map[string]string, now time.Time) (core.OperationBinding, core.DriverSession, error) {
	session, err := core.LoadDriverSession(core.DriverSessionPath(root, slug))
	if err != nil {
		return core.OperationBinding{}, session, err
	}
	if session.ID == "" || session.Expired(now) {
		return core.OperationBinding{}, session, nil
	}

	sessionID, nonce := flags["session"], flags["nonce"]
	if sessionID == "" || nonce == "" {
		recordConformance(root, slug, id, core.ConformanceWorkWithoutBootstrap,
			"mutable operation attempted without session bindings while session "+session.ID+" was open")
		return core.OperationBinding{}, session, core.Refusef("BINDING_MISSING", "driver session %s is open, so %s requires --session and --nonce", session.ID, id).
			WithRecovery(core.RefusalActorAgent, "specd session action "+slug+" --json")
	}

	// R3.2: authority does not activate until the required context lanes are
	// acknowledged. Checked before the nonce is spent, so a host that has not
	// acknowledged does not burn one discovering that.
	if session.ContextReceipt == nil {
		recordConformance(root, slug, id, core.ConformanceContextAckSkipped, "no context receipt recorded for session "+session.ID)
		return core.OperationBinding{}, session, core.Refusef("BINDING_MISSING", "session %s has acknowledged no context; mutable authority is withheld", session.ID).
			WithRecovery(core.RefusalActorAgent, "specd session ack "+slug+" "+id+" --tokens <n>")
	}
	if !session.ContextReceipt.Complete() {
		recordConformance(root, slug, id, core.ConformanceContextAckSkipped,
			fmt.Sprintf("%d required context lanes unacknowledged", len(session.ContextReceipt.MissingDigests)))
		return core.OperationBinding{}, session, core.Refusef("AUTHORITY_DENIED", "session %s is missing %d required context lane(s); mutable authority is withheld",
			session.ID, len(session.ContextReceipt.MissingDigests)).
			WithRecovery(core.RefusalActorAgent, "specd session ack "+slug+" "+id+" --tokens <n>")
	}

	binding := core.OperationBinding{
		SessionID:            sessionID,
		ExpectedRevision:     state.Revision,
		HandshakeDigest:      session.HandshakeDigest,
		AuthorityDigest:      session.AuthorityDigest,
		ContextReceiptDigest: session.ContextReceipt.ReceiptDigest,
		BaselineRevision:     session.BaselineRevision,
		Nonce:                nonce,
	}
	return binding, session, nil
}

// recordConformance observes a protocol violation (R7.1). The error is
// deliberately discarded: an observation must never change control flow, or the
// log becomes load-bearing (R7.2).
func recordConformance(root, slug, taskID, kind, detail string) {
	_ = core.RecordConformanceEvent(root, slug, core.ConformanceEvent{
		Kind: kind, TaskID: taskID, Operation: "complete-task", Detail: detail,
	})
}

// recordConformanceForRefusal maps a binding refusal to the protocol event it
// evidences, so the log distinguishes a replay from an expired session.
func recordConformanceForRefusal(root, slug, taskID string, err error) {
	refusal, ok := core.AsRefusal(err)
	if !ok {
		return
	}
	kind := ""
	switch refusal.Code {
	case "NONCE_REPLAYED":
		kind = core.ConformanceStaleActionReplayed
	case "REVISION_CONFLICT", "BASELINE_DRIFTED":
		kind = core.ConformanceStaleActionReplayed
	case "AUTHORITY_DENIED":
		kind = core.ConformanceActedWithoutAuthority
	case "SESSION_UNKNOWN", "SESSION_EXPIRED", "HANDSHAKE_MISMATCH":
		kind = core.ConformanceWorkWithoutBootstrap
	}
	if kind == "" {
		return
	}
	recordConformance(root, slug, taskID, kind, refusal.Code+": "+refusal.Blocker)
}
