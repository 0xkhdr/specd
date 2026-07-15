package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
	"github.com/0xkhdr/specd/internal/orchestration"
)

// runNew creates a spec workspace: requirements.md, design.md, tasks.md, and state.json at
// revision 0 (R13.3). Creation is a fresh write under the per-spec lock, not a
// compare-and-swap; SaveStateCAS with expected revision 0 would ratchet to 1.
func runNew(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: specd new <name>")
	}
	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return err
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
		if err := core.AtomicWrite(filepath.Join(specDir, "requirements.md"), requirementsStub(slug)); err != nil {
			return struct{}{}, err
		}
		if err := core.AtomicWrite(filepath.Join(specDir, "design.md"), designStub(slug)); err != nil {
			return struct{}{}, err
		}
		if err := core.AtomicWrite(filepath.Join(specDir, "tasks.md"), tasksStub(slug)); err != nil {
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
	if len(args) < 1 || len(args) > 2 {
		return errors.New("usage: specd approve <spec>")
	}
	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return err
	}
	var approvedTarget core.Status
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		statePath := core.StatePath(root, slug)
		state, err := core.LoadState(statePath)
		if err != nil {
			return struct{}{}, err
		}
		target := core.NextStatus(state.Status)
		if len(args) == 2 {
			target = core.Status(args[1])
		}
		phase, err := core.AdvanceStatus(state.Status, target)
		if err != nil {
			return struct{}{}, err
		}
		approvedTarget = target
		gate := string(target)
		spec, err := loadSpec(root, slug)
		if err != nil {
			return struct{}{}, err
		}
		registry, _, err := requiredRegistry(root)
		if err != nil {
			return struct{}{}, err
		}
		findings := registry.Run(buildCheckCtx(root, slug, spec, gate))
		if gates.HasErrors(findings) {
			for _, finding := range findings {
				if finding.Severity == gates.Error {
					fmt.Fprintf(os.Stderr, "%s %s: %s\n", finding.Severity, finding.Gate, finding.Message)
				}
			}
			return struct{}{}, errors.New("approve refused: readiness gates failing")
		}
		// Cross-spec links are enforcement, not annotation (spec 12 R5): a spec
		// may plan while a dependency is unfinished, but may not advance into the
		// execution phase until every spec it depends on is complete. Planning
		// phases stay unblocked; only the transition into StatusExecuting gates.
		if target == core.StatusExecuting {
			program, err := core.LoadProgram(core.ProgramPath(root))
			if err != nil {
				return struct{}{}, err
			}
			if blocking := program.IncompleteDeps(slug, func(dep string) bool { return specComplete(root, dep) }); len(blocking) > 0 {
				return struct{}{}, fmt.Errorf("approve refused: %s blocked by incomplete dependencies: %s", slug, strings.Join(blocking, ", "))
			}
		}
		state.Status = target
		state.Phase = phase
		rec := core.Record{Kind: "approval", Gate: gate, ApprovedRevision: state.Revision}
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
		return struct{}{}, core.SaveStateCAS(statePath, state.Revision, state)
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "approved %s → %s\n", slug, approvedTarget)
	return nil
}

func runMode(root string, args []string, flags map[string]string) error {
	if len(args) != 2 || args[1] != string(core.ModeOrchestrated) {
		return errors.New("usage: specd mode <spec> orchestrated")
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

// runTaskComplete marks a task complete: it requires a passing evidence record
// pinned to a real commit (core.CompleteTask), then writes the ✅ marker to
// tasks.md and the machine-truth status to state.json under one lock+CAS so the
// two never drift (the Sync gate enforces that agreement).
func runTaskComplete(root string, args []string, flags map[string]string) error {
	if len(args) != 2 {
		return errors.New("usage: specd task complete <spec> <id>")
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
		cfg := loadSpecConfig(root)
		if cfg.Security.Profile == "production" {
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
			sessionPath := filepath.Join(core.SpecdDir(root), "specs", slug, "session.json")
			session, err := orchestration.LoadSession(sessionPath)
			if err != nil {
				return struct{}{}, err
			}
			baseline := ""
			for _, m := range append(session.Missions, session.PendingMissions...) {
				if m.TaskID == id {
					baseline = m.SubjectHead
				}
			}
			if baseline == "" {
				return struct{}{}, fmt.Errorf("outside_scope: task %s has no pinned baseline; dispatch a fresh mission", id)
			}
			diff, err := core.DeriveDiff(root, baseline)
			if err != nil {
				return struct{}{}, err
			}
			if err := gates.CheckScope(diff.Paths, task.DeclaredFiles); err != nil {
				return struct{}{}, err
			}
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
		updated, err := core.CompleteTask(raw, id, spec.Evidence)
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
		if err := core.AtomicWrite(tasksPath, string(updated)); err != nil {
			return struct{}{}, err
		}
		// Optional telemetry (spec 10 R1): completion carries the worker's
		// verbatim cost as a supplementary evidence record. CompleteTask already
		// required a passing verify record above, so this record only annotates —
		// it never manufactures passing evidence.
		if annotations != nil {
			rec := core.EvidenceRecord{TaskID: id, Command: "task complete", ExitCode: 0, GitHead: gitHead(root), Telemetry: annotations}
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

// runSpike records a bounded exploratory-learning spike (spec 01 R7.3). It
// appends a spike record under the per-spec lock+CAS and touches no lifecycle
// status, task status, or approval record. Completion still demands a passing
// verify record (CompleteTask) and architecture still demands a human design
// approval — a spike is a distinct record kind neither path reads, so recording
// one can never complete a task or approve architecture. Required-field and
// bound enforcement lives in core.Spike.Validate (via AppendSpike).
func runSpike(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: specd spike <spec> --question <q> --scope <s> --expiry <RFC3339> [--output <ref>]")
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
			fmt.Fprintf(os.Stdout, "  --%s  %s\n", flag.Name, flag.Description)
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
	if len(args) >= 1 && args[0] == "complete" {
		return runTaskComplete(root, args[1:], flags)
	}
	if len(args) != 1 {
		return errors.New("usage: specd task <id> | specd task complete <spec> <id>")
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
	return fmt.Sprintf("# Requirements — %s\n\n"+
		"> Author EARS-shaped requirements. Each is testable and unambiguous.\n\n"+
		"- **R1** When <trigger>, the system shall <response>.\n", slug)
}

func designStub(slug string) string {
	return fmt.Sprintf("# Design — %s\n\n"+
		"> Name module boundaries, on-disk contracts, and preserved invariants.\n"+
		"> The design gate reads this file before tasks execute.\n\n"+
		"## Modules\n\n## On-disk contracts\n\n## Invariants\n", slug)
}

func tasksStub(slug string) string {
	return fmt.Sprintf(`# Tasks — %s

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | scout | requirements.md | - | printf ok | scaffolded read-only placeholder — replace with real tasks |
`, slug)
}
