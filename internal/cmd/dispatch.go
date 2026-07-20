package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

// ErrUsage is the sentinel for fail-closed rejections that must map to exit
// code 2 (usage / out-of-phase / invalid-enum), as opposed to gate/verify
// failures which exit 1. main.go inspects it. Wrap it, never return it bare,
// so the message names the specific violation (spec 03 R2, R3).
var ErrUsage = errors.New("usage")

// usageError builds an arity/usage rejection from the palette's own usage
// string, so a handler's error can never drift from what `specd help <verb>`
// and docs/command-reference.md print for the same verb.
func usageError(verb string) error {
	if cmd, ok := core.CommandByName(verb); ok {
		return fmt.Errorf("%w: %s", ErrUsage, cmd.Usage)
	}
	return fmt.Errorf("%w: specd %s", ErrUsage, verb)
}

// Run is the single dispatch choke point. It resolves the verb, enforces
// declared flag enums and lifecycle-phase compatibility *before* any handler
// side effect, then invokes the handler. Fail-closed rejections wrap ErrUsage
// (exit 2); unknown verbs wrap ErrUnknownCommand (exit 2). This is the one
// place the harness turns command metadata into enforcement (spec 03).
func Run(root, name string, args []string, flags map[string]string) error {
	return runDispatch(root, name, args, flags, nil, time.Time{}, nil)
}

func runDispatch(root, name string, args []string, flags map[string]string, authority *core.AuthorityV1, now time.Time, changedPaths []string) error {
	handler, ok := Registry[name]
	if !ok || handler == nil {
		return RefuseUnknownCommand(name)
	}
	meta, hasMeta := core.CommandByName(name)
	if hasMeta {
		// Additive help surface (spec R4.1): on a multi-operation verb, --help
		// or a missing subcommand prints the verb's palette operations and
		// exits 0 instead of failing closed. Unknown subcommands still fail
		// closed below (exit 2) — only the discoverability path changed.
		if wantsOperationPalette(name, args, flags) {
			printOperationPalette(meta)
			return nil
		}
		operation, ok := core.ResolveOperation(name, args, flags)
		if !ok {
			return core.Refusef("OPERATION_UNKNOWN", "unknown operation for command %q", name).
				WithRecovery(core.RefusalActorAgent, "specd help "+name).Wrapping(ErrUsage)
		}
		if name == "agents" && len(args) > 0 && args[0] == "inspect" {
			// `agents inspect` aliases bare `agents` (spec R4.2): the alias token
			// resolved to agents.inspect above; the handler expects it stripped.
			args = args[1:]
		}
		if err := checkFlagEnums(meta, flags); err != nil {
			return err
		}
		if err := checkPhase(root, meta, args); err != nil {
			return err
		}
		if err := checkFreshDispatch(root, name, args); err != nil {
			return err
		}
		cfg := loadSpecConfig(root)
		productionTask := cfg.ProductionTaskAuthorityRequired() && operation.TaskRequired
		if productionTask && authority == nil {
			// No packet was supplied, so none was consumed: the agent can retry
			// once it holds one.
			return core.Refuse("AUTHORITY_DENIED", "production task command requires AuthorityV1 packet")
		}
		if authority != nil {
			mutable := operation.Effect != core.EffectRead
			phase := authority.Phase
			if meta.SpecSlugArg != nil && *meta.SpecSlugArg < len(args) {
				if state, err := core.LoadState(core.StatePath(root, args[*meta.SpecSlugArg])); err == nil {
					phase = string(state.Phase)
				}
			}
			if productionTask {
				paths, err := productionMissionScope(root, operation, args, *authority)
				if err != nil {
					return core.Refusef("AUTHORITY_DENIED", "%v", err)
				}
				changedPaths = paths
			}
			if err := core.AuthorizeTool(*authority, name, changedPaths, now, phase, mutable); err != nil {
				_ = orchestration.RecordAuthorityDenial(root, *authority, name, "denied", now)
				// The denial is recorded against this packet, so it is spent: a
				// retry needs a freshly issued one.
				return core.Refusef("AUTHORITY_DENIED", "%v", err).Consumed()
			}
			if meta.SpecSlugArg != nil && *meta.SpecSlugArg < len(args) && authority.SpecID != args[*meta.SpecSlugArg] {
				return core.Refuse("AUTHORITY_DENIED", "spec mismatch")
			}
			taskArg := 1
			if name == "task" {
				taskArg = 2
			}
			if operation.TaskRequired && len(args) > taskArg && authority.TaskID != args[taskArg] {
				return core.Refuse("AUTHORITY_DENIED", "task mismatch")
			}
		}
	}
	return handler(root, args, flags)
}

// wantsOperationPalette reports whether this invocation asks for the verb's
// operation palette instead of an operation: --help on any multi-operation
// verb, or an empty subcommand on one that has no bare (subcommand-less)
// operation and would otherwise fail closed with no guidance (spec R4.1).
func wantsOperationPalette(name string, args []string, flags map[string]string) bool {
	operations := core.OperationsForCommand(name)
	hasSubcommand, hasBare := false, false
	for _, operation := range operations {
		if operation.Subcommand != "" {
			hasSubcommand = true
		} else {
			hasBare = true
		}
	}
	if len(operations) < 2 || !hasSubcommand {
		return false
	}
	if _, ok := flags["help"]; ok {
		return true
	}
	return len(args) == 0 && !hasBare
}

// printOperationPalette renders the palette operations already declared in
// core/commands.go — usage, examples, and the verb's flags — the same metadata
// help --json and MCP serve. Nothing is restated by hand.
func printOperationPalette(meta core.Command) {
	fmt.Fprintf(os.Stdout, "usage: %s\n%s\n\noperations:\n", meta.Usage, meta.Description)
	for _, operation := range core.OperationsForCommand(meta.Name) {
		fmt.Fprintf(os.Stdout, "  %-18s %s\n", operation.ID, operation.Usage)
		for _, example := range operation.Examples {
			fmt.Fprintf(os.Stdout, "%20s example: %s\n", "", example)
		}
	}
	if len(meta.Flags) > 0 {
		fmt.Fprintln(os.Stdout, "\nflags:")
		for _, flag := range meta.Flags {
			fmt.Fprintf(os.Stdout, "  --%-16s %s\n", flag.Name, flag.Description)
		}
	}
}

func productionMissionScope(root string, operation core.Operation, args []string, authority core.AuthorityV1) ([]string, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("task identity missing")
	}
	slug, taskID := args[0], args[1]
	if operation.Command == "task" {
		if len(args) < 3 {
			return nil, fmt.Errorf("task identity missing")
		}
		slug, taskID = args[1], args[2]
	}
	if authority.SpecID != slug {
		return nil, fmt.Errorf("spec mismatch")
	}
	if authority.TaskID != taskID {
		return nil, fmt.Errorf("task mismatch")
	}
	spec, err := loadSpec(root, slug)
	if err != nil {
		return nil, err
	}
	var task *core.TaskRow
	for i := range spec.Tasks {
		if spec.Tasks[i].ID == taskID {
			task = &spec.Tasks[i]
			break
		}
	}
	if task == nil {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	if authority.Role != task.Role {
		return nil, fmt.Errorf("role mismatch")
	}
	session, err := orchestration.LoadSession(filepath.Join(core.SpecdDir(root), "specs", slug, "session.json"))
	if err != nil {
		return nil, err
	}
	missionID := ""
	for _, lease := range session.Leases {
		if lease.TaskID == taskID && lease.Authority.Digest == authority.Digest {
			missionID = lease.MissionID
		}
	}
	if missionID == "" {
		return nil, fmt.Errorf("fresh authority lease not found")
	}
	var mission *orchestration.MissionV1
	for i := range session.Missions {
		if session.Missions[i].MissionID == missionID {
			mission = &session.Missions[i]
		}
	}
	if mission == nil {
		return nil, fmt.Errorf("fresh mission not found")
	}
	if mission.Role != task.Role || mission.SubjectHead != authority.BaselineRevision {
		return nil, fmt.Errorf("mission binding mismatch")
	}
	if !samePaths(mission.DeclaredFiles, task.DeclaredFiles) || !samePaths(authority.DeclaredWritePaths, mission.DeclaredFiles) {
		return nil, fmt.Errorf("mission scope binding mismatch")
	}
	diff, err := core.DeriveDiff(root, mission.SubjectHead)
	if err != nil {
		return nil, err
	}
	return diff.Paths, nil
}

func samePaths(a, b []string) bool {
	left, right := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(left)
	sort.Strings(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func checkFreshDispatch(root, name string, args []string) error {
	idx := -1
	switch name {
	case "next":
		idx = 0
	case "brain":
		idx = 1
	}
	if idx < 0 || idx >= len(args) {
		return nil
	}
	state, err := core.LoadState(core.StatePath(root, args[idx]))
	if err != nil {
		return nil
	}
	report, err := state.StateFreshness()
	if err != nil || len(report.Stale) == 0 {
		return nil
	}
	// Only a human can re-approve, so this refusal is not agent-retryable.
	return core.Refusef("APPROVAL_REQUIRED", "dispatch paused: stale records require re-approval: %s", strings.Join(report.Stale, ", ")).
		WithRecovery(core.RefusalActorHuman, "specd approve "+args[idx])
}

// RunAuthorized enforces a mission authority packet before normal dispatch.
func RunAuthorized(root, name string, args []string, flags map[string]string, authority core.AuthorityV1, changedPaths []string, now time.Time) error {
	return runDispatch(root, name, args, flags, &authority, now, changedPaths)
}

// checkFlagEnums fails closed (exit 2) when a flag carrying a declared enum is
// given a value outside that enum (spec 03 R3). Flags absent from metadata are
// left to the handler; boolean flags (no Enum) are never enum-checked.
func checkFlagEnums(meta core.Command, flags map[string]string) error {
	for name, value := range flags {
		flag := meta.FlagByName(name)
		if flag == nil || len(flag.Enum) == 0 {
			continue
		}
		if !slices.Contains(flag.Enum, value) {
			return core.Refusef("FLAG_VALUE_INVALID", "flag --%s=%q not allowed; expected one of %v", name, value, flag.Enum).
				WithRecovery(core.RefusalActorAgent, "specd help "+meta.Name).Wrapping(ErrUsage)
		}
	}
	return nil
}

// checkPhase fails closed (exit 2) when a verb is invoked against a spec whose
// current lifecycle phase is not in the verb's allowed set (spec 03 R2). Only
// verbs that resolve a spec by a fixed positional index are checked; every
// other verb declares PhaseAny and is skipped by construction. A spec whose
// state cannot be loaded (absent/new) is left to the handler — the phase gate
// never invents a rejection from a missing file.
func checkPhase(root string, meta core.Command, args []string) error {
	if meta.SpecSlugArg == nil || meta.AllowsPhase(core.PhaseAny) {
		return nil
	}
	idx := *meta.SpecSlugArg
	if idx >= len(args) {
		return nil // arity error surfaces in the handler's own usage message
	}
	slug := args[idx]
	// Reject a traversal slug at the boundary, before any handler resolves it
	// into a filesystem path (a phase-enforced verb reads/writes state.json for
	// its slug; "../../x" would escape .specd/specs/). Fail closed (exit 2).
	if err := core.ValidateSlug(slug); err != nil {
		return core.Refusef("SPEC_INVALID", "%v", err).Wrapping(ErrUsage)
	}
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return nil // no resolvable spec state ⇒ not our rejection to make
	}
	if meta.AllowsPhase(state.Phase) {
		return nil
	}
	return core.Refusef("PHASE_INVALID", "verb %q not allowed in phase %q; allowed phases: %s",
		meta.Name, state.Phase, joinPhases(meta.AllowedPhases)).
		WithRecovery(core.RefusalActorAgent, "specd status "+slug+" --guide").Wrapping(ErrUsage)
}

func joinPhases(phases []core.Phase) string {
	out := ""
	for i, p := range phases {
		if i > 0 {
			out += ", "
		}
		out += string(p)
	}
	return out
}
