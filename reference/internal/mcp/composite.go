package mcp

import (
	"fmt"
	"sort"
	"strings"
)

// Composite MCP tools (spec A2/A4). The command-mirror tools map 1:1 to specd
// commands, which makes inspection alone ~10 tools and orchestration a confusing
// mix of raw + intent tools. Composites collapse that surface into a handful of
// view-/action-routed verbs:
//
//	specd_inspect   — view-routed read (status/waves/context/check/validate/replay/diff)
//	specd_read      — report rendering (serve/watch stay CLI-only over MCP)
//	specd_query     — frontier queries (next/dispatch)
//	specd_orchestrate — action-routed Brain control (start/step/status/why/pause/resume/cancel)
//	specd_worker      — action-routed Pinky control (claim/heartbeat/.../inbox)
//
// They are dispatch wrappers: each validates its view/action against a fixed
// allowlist, then translates to the same (command, argv) the atomic tool would
// produce — no new core authority, byte-identical output (spec R8).

// compositeOrchestration names the composites gated behind orchestration; like
// the brain_* intent tools they vanish when orchestration is excluded (spec R7).
var compositeOrchestration = map[string]bool{
	"specd_orchestrate": true,
	"specd_worker":      true,
}

// Allowed view/action sets. Kept as ordered slices so error messages and schema
// enums are deterministic.
var (
	inspectViews     = []string{"status", "waves", "context", "check", "validate", "replay", "diff"}
	readViews        = []string{"report"}
	queryViews       = []string{"next", "dispatch"}
	orchestrateVerbs = []string{"start", "step", "status", "why", "pause", "resume", "cancel"}
	workerVerbs      = []string{"claim", "heartbeat", "progress", "query", "report", "block", "release", "inbox"}
)

// compositeTools is the registered composite set in stable order.
var compositeTools = []intentTool{
	{
		name:        "specd_inspect",
		description: "Read-only inspection of a spec, routed by `view`: status (state summary), waves (wave readiness), context (phase context), check (gate results), validate (schema validation), replay (decision history), diff (state delta from a ref). Returns the selected view's output unchanged.",
		readOnly:    true,
		args: []intentArg{
			{name: "view", typ: "string", description: "Which inspection to run.", required: true, enum: inspectViews},
			{name: "slug", typ: "string", description: "Spec slug to inspect."},
			{name: "from", typ: "string", description: "diff: baseline git ref (required for view=diff)."},
			{name: "to", typ: "string", description: "diff: target git ref (defaults to the working tree)."},
		},
		translate: func(args map[string]any) (string, []string, error) {
			view, err := requireEnum(args, "view", inspectViews)
			if err != nil {
				return "", nil, err
			}
			argv, err := positionalSlug(args)
			if err != nil {
				return "", nil, err
			}
			if view == "diff" {
				if _, ok, _ := argString(args, "from"); !ok {
					return "", nil, fmt.Errorf("specd_inspect view=diff requires a 'from' ref")
				}
			}
			argv = append(argv, forwardFlags(args, "view", "slug")...)
			if view == "validate" && !hasFlag(args, "schema") {
				argv = append(argv, "--schema")
			}
			return view, argv, nil
		},
	},
	{
		name:        "specd_read",
		description: "Render a spec's report (Markdown or HTML). Streaming transports (serve/watch) stay CLI-only over MCP, so this returns the report body for a single request.",
		readOnly:    true,
		args: []intentArg{
			{name: "view", typ: "string", description: "Read view; currently only report.", required: true, enum: readViews},
			{name: "slug", typ: "string", description: "Spec slug to report on."},
			{name: "format", typ: "string", description: "Output format: md (default) or html.", enum: []string{"md", "html"}},
		},
		translate: func(args map[string]any) (string, []string, error) {
			if _, err := requireEnum(args, "view", readViews); err != nil {
				return "", nil, err
			}
			argv, err := positionalSlug(args)
			if err != nil {
				return "", nil, err
			}
			argv = append(argv, forwardFlags(args, "view", "slug")...)
			return "report", argv, nil
		},
	},
	{
		name:        "specd_query",
		description: "Query a spec's work frontier, routed by `view`: next (the next runnable task/decision) or dispatch (the next dispatchable mission). Read-only.",
		readOnly:    true,
		args: []intentArg{
			{name: "view", typ: "string", description: "Which query to run.", required: true, enum: queryViews},
			{name: "slug", typ: "string", description: "Spec slug to query."},
			{name: "all", typ: "boolean", description: "next: include the full frontier, not just the first item."},
		},
		translate: func(args map[string]any) (string, []string, error) {
			view, err := requireEnum(args, "view", queryViews)
			if err != nil {
				return "", nil, err
			}
			argv, err := positionalSlug(args)
			if err != nil {
				return "", nil, err
			}
			argv = append(argv, forwardFlags(args, "view", "slug")...)
			return view, argv, nil
		},
	},
	{
		name:        "specd_orchestrate",
		description: "Control a Brain orchestration session, routed by `action`: start (begin driving a spec), step (advance one decision), status (read current decision/state), why (explain the last decision), pause/resume (suspend/continue), cancel (terminate cooperatively). Translates to the matching `brain` sub-action.",
		args: []intentArg{
			{name: "action", typ: "string", description: "Brain action to perform.", required: true, enum: orchestrateVerbs},
			{name: "spec", typ: "string", description: "Spec slug to start (action=start)."},
			{name: "session", typ: "string", description: "Session id to act on (status/step/why/pause/resume/cancel)."},
			{name: "program", typ: "boolean", description: "Target a program (cross-spec) session."},
			{name: "approval_policy", typ: "string", description: "start: override approval policy (manual|planning|session)."},
			{name: "worker_cmd", typ: "string", description: "start: shell command run per Pinky dispatch."},
			{name: "max_steps", typ: "string", description: "start: maximum Brain steps before stopping."},
		},
		translate: func(args map[string]any) (string, []string, error) {
			action, err := requireEnum(args, "action", orchestrateVerbs)
			if err != nil {
				return "", nil, err
			}
			argv := []string{action}
			if spec, ok, err := argString(args, "spec"); err != nil {
				return "", nil, err
			} else if ok {
				argv = append(argv, spec)
			}
			argv = append(argv, forwardFlags(args, "action", "spec")...)
			return "brain", argv, nil
		},
	},
	{
		name:        "specd_worker",
		description: "Control a Pinky worker, routed by `action`: claim/heartbeat/progress/report/block/release manage a mission lease and its evidence; query/inbox read worker state. Translates to the matching `pinky` sub-action.",
		args: []intentArg{
			{name: "action", typ: "string", description: "Pinky action to perform.", required: true, enum: workerVerbs},
			{name: "session", typ: "string", description: "Session id the worker belongs to."},
			{name: "task", typ: "string", description: "Task id the mission targets."},
			{name: "text", typ: "string", description: "Free-text payload (progress note, block reason, report body)."},
			{name: "severity", typ: "string", description: "block: severity of the blocker."},
			{name: "evidence", typ: "string", description: "report: evidence reference."},
		},
		translate: func(args map[string]any) (string, []string, error) {
			action, err := requireEnum(args, "action", workerVerbs)
			if err != nil {
				return "", nil, err
			}
			argv := append([]string{action}, forwardFlags(args, "action")...)
			return "pinky", argv, nil
		},
	},
}

// compositeByName indexes compositeTools for O(1) routing in callTool.
var compositeByName = func() map[string]intentTool {
	m := make(map[string]intentTool, len(compositeTools))
	for _, t := range compositeTools {
		m[t.name] = t
	}
	return m
}()

// requireEnum reads a required string argument and validates it against a closed
// allowlist, returning an error that names the valid values (spec R6).
func requireEnum(args map[string]any, key string, allowed []string) (string, error) {
	v, ok, err := argString(args, key)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%s is required; one of: %s", key, strings.Join(allowed, ", "))
	}
	for _, a := range allowed {
		if a == v {
			return v, nil
		}
	}
	return "", fmt.Errorf("unknown %s %q; valid values: %s", key, v, strings.Join(allowed, ", "))
}

// positionalSlug returns the leading argv carrying an optional spec slug.
func positionalSlug(args map[string]any) ([]string, error) {
	slug, ok, err := argString(args, "slug")
	if err != nil {
		return nil, err
	}
	if ok {
		return []string{slug}, nil
	}
	return nil, nil
}

// hasFlag reports whether a non-nil argument was supplied for key.
func hasFlag(args map[string]any, key string) bool {
	v, ok := args[key]
	return ok && v != nil
}

// forwardFlags renders the remaining named arguments as CLI flags, mirroring
// buildArgv's flag emission so a composite round-trips to its atomic equivalent
// (spec R8): booleans become bare --flag, others --flag value, keys are sorted
// for determinism, and underscores map to hyphens (approval_policy → --approval-policy).
// The excluded keys are the routing/positional arguments handled separately.
func forwardFlags(args map[string]any, exclude ...string) []string {
	skip := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		skip[e] = true
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		if !skip[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var argv []string
	for _, k := range keys {
		flag := "--" + strings.ReplaceAll(k, "_", "-")
		switch v := args[k].(type) {
		case bool:
			if v {
				argv = append(argv, flag)
			}
		case nil:
			// omitted flag
		default:
			argv = append(argv, flag, fmt.Sprint(v))
		}
	}
	return argv
}
