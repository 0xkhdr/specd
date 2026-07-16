package cmd

import (
	"errors"
	"fmt"
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
		return fmt.Errorf("%w: %q", ErrUnknownCommand, name)
	}
	meta, hasMeta := core.CommandByName(name)
	if hasMeta {
		operation, ok := core.ResolveOperation(name, args, flags)
		if !ok {
			return fmt.Errorf("%w: unknown operation for command %q", ErrUsage, name)
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
		if cfg.Security.Profile == "production" && operation.TaskRequired && authority == nil {
			return fmt.Errorf("authority denied: production task command requires AuthorityV1 packet")
		}
		if authority != nil {
			mutable := operation.Effect != core.EffectRead
			phase := string(authority.Phase)
			if err := core.AuthorizeTool(*authority, name, changedPaths, now, phase, mutable); err != nil {
				_ = orchestration.RecordAuthorityDenial(root, *authority, name, "denied", now)
				return fmt.Errorf("authority denied: %w", err)
			}
			if meta.SpecSlugArg != nil && *meta.SpecSlugArg < len(args) && authority.SpecID != args[*meta.SpecSlugArg] {
				return fmt.Errorf("authority denied: spec mismatch")
			}
			taskArg := 1
			if name == "task" {
				taskArg = 2
			}
			if operation.TaskRequired && len(args) > taskArg && authority.TaskID != args[taskArg] {
				return fmt.Errorf("authority denied: task mismatch")
			}
		}
	}
	return handler(root, args, flags)
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
	return fmt.Errorf("dispatch paused: stale records require re-approval: %s", strings.Join(report.Stale, ", "))
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
		if !contains(flag.Enum, value) {
			return fmt.Errorf("%w: flag --%s=%q not allowed; expected one of %v", ErrUsage, name, value, flag.Enum)
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
		return fmt.Errorf("%w: %v", ErrUsage, err)
	}
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return nil // no resolvable spec state ⇒ not our rejection to make
	}
	if meta.AllowsPhase(state.Phase) {
		return nil
	}
	return fmt.Errorf("%w: verb %q not allowed in phase %q; allowed phases: %s",
		ErrUsage, meta.Name, state.Phase, joinPhases(meta.AllowedPhases))
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
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
