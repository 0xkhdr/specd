package mcp

import "fmt"

// Intent-level MCP tools (GAP-5). The command-mirror tools (specd_*) expose the
// full flag surface as a generic args-array passthrough — powerful but high
// cognitive load: an MCP client must know every flag and run the whole loop
// itself. These intent tools wrap the same deterministic primitives with sane
// policy defaults so a model gets one clear affordance per intent:
//
//	brain_orchestrate — start/resume driving a spec to delivery (wraps `brain run`)
//	brain_status      — read a session's current decision/state (wraps `brain status`)
//	brain_approve     — clear an approval gate / advance a planning phase (wraps `approve`)
//	brain_pause       — pause an orchestration session (wraps `brain pause`)
//	brain_resume      — resume a paused session (wraps `brain resume`)
//	brain_cancel      — cancel a session cooperatively (wraps `brain cancel`)
//
// They add NO new core authority: each translates to a command + argv that the
// raw passthrough could already produce. The raw specd_brain/specd_pinky tools
// remain for power users.

// intentArg describes one named argument of an intent tool.
type intentArg struct {
	name        string
	typ         string // "string" or "boolean"
	description string
	required    bool
	// enum, when non-empty, advertises the closed set of allowed string values to
	// the host so a composite's view/action property is self-documenting.
	enum []string
}

// intentTool is a semantic tool plus the translation from its named arguments to
// a (command, argv) pair routed through the same dispatcher as every other tool.
type intentTool struct {
	name        string
	description string
	readOnly    bool
	args        []intentArg
	// translate maps validated arguments to the underlying command and its
	// argv (without --json; the dispatcher appends it). It returns an error for
	// a missing required argument or an otherwise un-runnable request.
	translate func(args map[string]any) (command string, argv []string, err error)
}

// def renders an intent tool's MCP schema. Unlike command-mirror tools it models
// named properties (no positional "args" array) so the client never plumbs flags.
func (t intentTool) def() toolDef {
	props := make(map[string]schemaProp, len(t.args))
	for _, a := range t.args {
		props[a.name] = schemaProp{Type: a.typ, Description: a.description, Enum: a.enum}
	}
	return toolDef{
		Name:        t.name,
		Description: t.description,
		InputSchema: jsonSchema{Type: "object", Properties: props, AdditionalProperties: false},
		Annotations: toolAnnotations{ReadOnlyHint: t.readOnly},
		intent:      true,
	}
}

// argString reads a required-or-optional string argument, coercing scalars and
// rejecting wrong-typed values so a translator never silently drops input.
func argString(args map[string]any, key string) (string, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return "", false, nil
	}
	switch v := raw.(type) {
	case string:
		return v, v != "", nil
	case bool, float64, int:
		return fmt.Sprint(v), true, nil
	default:
		return "", false, fmt.Errorf("argument %q must be a string", key)
	}
}

// argBool reads an optional boolean argument; a present non-bool is an error.
func argBool(args map[string]any, key string) (bool, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return false, false, nil
	}
	v, ok := raw.(bool)
	if !ok {
		return false, false, fmt.Errorf("argument %q must be a boolean", key)
	}
	return v, true, nil
}

// sessionControlArgs is the shared argument set for status/pause/resume/cancel:
// a required session id plus an optional program flag.
var sessionControlArgs = []intentArg{
	{name: "session", typ: "string", description: "Orchestration session id to act on.", required: true},
	{name: "program", typ: "boolean", description: "Target a program (cross-spec) session instead of a single spec."},
}

// sessionControlTranslate builds `brain <verb> --session <id> [--program]`.
func sessionControlTranslate(verb string) func(map[string]any) (string, []string, error) {
	return func(args map[string]any) (string, []string, error) {
		session, ok, err := argString(args, "session")
		if err != nil {
			return "", nil, err
		}
		if !ok {
			return "", nil, fmt.Errorf("%s requires a 'session' id", verb)
		}
		argv := []string{verb, "--session", session}
		program, _, err := argBool(args, "program")
		if err != nil {
			return "", nil, err
		}
		if program {
			argv = append(argv, "--program")
		}
		return "brain", argv, nil
	}
}

// intentTools is the registered intent-level tool set, kept in a stable order so
// the tool list and golden schema are deterministic.
var intentTools = []intentTool{
	{
		name:        "brain_orchestrate",
		description: "Drive a spec from its current phase toward delivery: bootstraps a missing spec, then runs the deterministic Brain loop under the planning approval policy until it completes, escalates, or awaits approval. Supply a worker command to execute Pinky dispatches; without one the loop stops at the first dispatch for the host to run. Returns the final session id and decision.",
		args: []intentArg{
			{name: "spec", typ: "string", description: "Spec slug to orchestrate (created if absent).", required: true},
			{name: "goal", typ: "string", description: "One-line goal/title used when bootstrapping a new spec."},
			{name: "worker_cmd", typ: "string", description: "Shell command run per Pinky dispatch; receives the mission via SPECD_* env. Omit to stop at the first dispatch."},
			{name: "approval_policy", typ: "string", description: "Override the approval policy (default: planning). One of manual|planning|session."},
			{name: "max_steps", typ: "string", description: "Maximum Brain steps before the driver stops (safety bound)."},
			{name: "session", typ: "string", description: "Resume or name a specific session id instead of the active one."},
			{name: "no_bootstrap", typ: "boolean", description: "Do not auto-create a missing spec; fail closed instead."},
		},
		translate: func(args map[string]any) (string, []string, error) {
			spec, ok, err := argString(args, "spec")
			if err != nil {
				return "", nil, err
			}
			if !ok {
				return "", nil, fmt.Errorf("brain_orchestrate requires a 'spec' slug")
			}
			argv := []string{"run", spec}

			noBootstrap, _, err := argBool(args, "no_bootstrap")
			if err != nil {
				return "", nil, err
			}
			if !noBootstrap {
				argv = append(argv, "--bootstrap")
			}
			// goal seeds the spec title only on a bootstrap; harmless otherwise.
			if goal, ok, err := argString(args, "goal"); err != nil {
				return "", nil, err
			} else if ok {
				argv = append(argv, "--title", goal)
			}
			for _, m := range []struct{ key, flag string }{
				{"approval_policy", "--approval-policy"},
				{"worker_cmd", "--worker-cmd"},
				{"max_steps", "--max-steps"},
				{"session", "--session"},
			} {
				v, ok, err := argString(args, m.key)
				if err != nil {
					return "", nil, err
				}
				if ok {
					argv = append(argv, m.flag, v)
				}
			}
			return "brain", argv, nil
		},
	},
	{
		name:        "brain_status",
		description: "Report the current state of an orchestration session: its latest deterministic decision, phase, and whether it is running, awaiting approval, escalated, or terminal. Read-only.",
		readOnly:    true,
		args:        sessionControlArgs,
		translate: func(args map[string]any) (string, []string, error) {
			session, ok, err := argString(args, "session")
			if err != nil {
				return "", nil, err
			}
			if !ok {
				return "", nil, fmt.Errorf("brain_status requires a 'session' id")
			}
			argv := []string{"status", "--session", session}
			program, _, err := argBool(args, "program")
			if err != nil {
				return "", nil, err
			}
			if program {
				argv = append(argv, "--program")
			}
			return "brain", argv, nil
		},
	},
	{
		name:        "brain_approve",
		description: "Clear an awaiting-approval gate or advance the planning phase of a spec. Subject to the same human-only gate rules as `specd approve`: high/critical mid-requirement gates can never be auto-cleared.",
		args: []intentArg{
			{name: "spec", typ: "string", description: "Spec slug whose gate to clear / phase to advance.", required: true},
		},
		translate: func(args map[string]any) (string, []string, error) {
			spec, ok, err := argString(args, "spec")
			if err != nil {
				return "", nil, err
			}
			if !ok {
				return "", nil, fmt.Errorf("brain_approve requires a 'spec' slug")
			}
			return "approve", []string{spec}, nil
		},
	},
	{
		name:        "brain_pause",
		description: "Pause an orchestration session. The driver stops issuing new dispatches; in-flight workers are unaffected. Resume with brain_resume.",
		args:        sessionControlArgs,
		translate:   sessionControlTranslate("pause"),
	},
	{
		name:        "brain_resume",
		description: "Resume a paused orchestration session so the Brain loop can step again.",
		args:        sessionControlArgs,
		translate:   sessionControlTranslate("resume"),
	},
	{
		name:        "brain_cancel",
		description: "Cancel an orchestration session cooperatively. The session reaches a terminal state; evidence and lease invariants are preserved.",
		args:        sessionControlArgs,
		translate:   sessionControlTranslate("cancel"),
	},
	{
		name:        "mode_get",
		description: "Show a spec's effective execution mode (base | orchestrated), how it was chosen (origin), and whether the project has orchestration capability. Capability only permits orchestration; the spec's mode selects it.",
		readOnly:    true,
		args: []intentArg{
			{name: "spec", typ: "string", description: "Spec slug to inspect.", required: true},
		},
		translate: func(args map[string]any) (string, []string, error) {
			spec, ok, err := argString(args, "spec")
			if err != nil {
				return "", nil, err
			}
			if !ok {
				return "", nil, fmt.Errorf("mode_get requires a 'spec' slug")
			}
			return "mode", []string{spec}, nil
		},
	},
	{
		name:        "mode_set",
		description: "Set a spec's execution mode. 'orchestrated' opts the spec into Brain/Pinky and requires project orchestration capability (fails closed otherwise, emitting the enabling command); 'base' opts back out (refused while a Brain session is active). Never escalate without an explicit user request.",
		args: []intentArg{
			{name: "spec", typ: "string", description: "Spec slug to change.", required: true},
			{name: "mode", typ: "string", description: "Target execution mode.", required: true, enum: []string{"base", "orchestrated"}},
		},
		translate: func(args map[string]any) (string, []string, error) {
			spec, ok, err := argString(args, "spec")
			if err != nil {
				return "", nil, err
			}
			if !ok {
				return "", nil, fmt.Errorf("mode_set requires a 'spec' slug")
			}
			mode, ok, err := argString(args, "mode")
			if err != nil {
				return "", nil, err
			}
			if !ok {
				return "", nil, fmt.Errorf("mode_set requires a 'mode' (base|orchestrated)")
			}
			return "mode", []string{spec, "--set", mode}, nil
		},
	},
	{
		name:        "mode_recommend",
		description: "Compute a deterministic, advisory execution-mode recommendation for a spec from on-disk countable facts (task count, wave width, distinct roles, cross-spec edges, token estimate). The verdict is userDecides:true — surface it as a suggestion and let the user choose; never switch automatically.",
		readOnly:    true,
		args: []intentArg{
			{name: "spec", typ: "string", description: "Spec slug to evaluate.", required: true},
		},
		translate: func(args map[string]any) (string, []string, error) {
			spec, ok, err := argString(args, "spec")
			if err != nil {
				return "", nil, err
			}
			if !ok {
				return "", nil, fmt.Errorf("mode_recommend requires a 'spec' slug")
			}
			return "mode", []string{spec, "--recommend"}, nil
		},
	},
}

// IntentToolCount is the number of intent-level tools exposed alongside the
// command-mirror tools. Exported so external tests can assert tools/list parity.
var IntentToolCount = len(intentTools)

// intentByName indexes intentTools for O(1) routing in callTool.
var intentByName = func() map[string]intentTool {
	m := make(map[string]intentTool, len(intentTools))
	for _, t := range intentTools {
		m[t.name] = t
	}
	return m
}()
