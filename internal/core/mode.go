package core

// mode.go holds the per-spec execution-mode resolution and the project-level
// orchestration capability check. The two are deliberately separate concepts:
// capability *permits* orchestration for the whole project; mode *selects* it
// for one spec. Never conflate them — a project may be orchestration-capable
// while a given spec still runs Base.

// ResolveMode returns the effective execution mode for an invocation and the
// origin that should be recorded for it, enforcing the precedence:
//
//	explicit command flag  >  spec.ExecutionMode  >  base
//
// flag is the one-shot override ("" when none was given). When a flag is
// present it wins and the choice is attributed to the user (the caller is
// expected to persist it so state stays the single source of truth — no hidden
// drift). Otherwise the spec's recorded mode/origin carries through, defaulting
// to Base/default for specs that never opted in.
func ResolveMode(flag string, s *State) (mode, origin string) {
	if flag != "" {
		return flag, OriginUser
	}
	if s == nil || s.ExecutionMode == "" {
		return ModeBase, OriginDefault
	}
	origin = s.ModeOrigin
	if origin == "" {
		origin = OriginDefault
	}
	return s.ExecutionMode, origin
}

// ProjectOrchestrationEnabled reports whether the project has orchestration
// capability — i.e. `.specd/config.json` has `orchestration.enabled: true`.
// This is the capability gate consulted before a spec may be set to
// orchestrated; it never selects a mode on its own.
func ProjectOrchestrationEnabled(root string) bool {
	return LoadConfig(root).Orchestration.Enabled
}
