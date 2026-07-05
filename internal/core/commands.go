package core

// HelpSchemaVersion versions the machine-readable help palette (`help --json`).
// Consumers (MCP, role prompts, external tools) pin against it and can detect a
// shape change; bump it whenever the Command/Flag JSON contract changes (spec
// 03 R4, pairs with the state-schema discipline of spec 02).
const HelpSchemaVersion = 1

// Flag describes one command-line flag surfaced by help metadata. Enum and
// Default make the flag a machine-readable contract: dispatch validates values
// against Enum (spec 03 R3) and MCP maps Enum/Default into JSON Schema.
type Flag struct {
	Name        string   `json:"name"`
	TakesValue  bool     `json:"takes_value,omitempty"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"`    // "bool" | "string"; empty ⇒ bool
	Enum        []string `json:"enum,omitempty"`    // allowed values (value flags only)
	Default     string   `json:"default,omitempty"` // documented default
}

// ExitCode documents one exit status a command can return. The convention is
// 0 success, 1 gate/verify failure, 2 usage / fail-closed rejection; per-verb
// deviations are declared explicitly (spec 03 design notes).
type ExitCode struct {
	Code    int    `json:"code"`
	Meaning string `json:"meaning"`
}

// Command describes one supported top-level command. This metadata is the
// single source of truth for help, dispatch enforcement, MCP tool schemas, and
// role prompts — no surface hand-restates command semantics (spec 03 C.8).
type Command struct {
	Name        string `json:"name"`
	Usage       string `json:"usage"`
	Description string `json:"description"`
	Flags       []Flag `json:"flags,omitempty"`
	// AllowedPhases is the set of lifecycle phases the verb may run in. A verb
	// valid everywhere declares []Phase{PhaseAny} explicitly — nothing defaults
	// silently to unrestricted (spec 03 R1, R6).
	AllowedPhases []Phase `json:"allowed_phases,omitempty"`
	// ExitCodes documents every status the verb can return (spec 03 R1/B.3).
	ExitCodes []ExitCode `json:"exit_codes,omitempty"`
	// Examples is at least one runnable invocation (spec 03 R1).
	Examples []string `json:"examples,omitempty"`
	// SpecSlugArg is the positional-argument index (0-based) that carries the
	// spec slug for phase enforcement, or nil when the verb resolves no spec by
	// a fixed position. Dispatch only phase-checks verbs with a non-nil index
	// (spec 03 R2: "verbs that take no spec slug skip the check"). Not exported
	// to JSON — it is an internal dispatch hint, not part of the help contract.
	SpecSlugArg *int `json:"-"`
	// Deferred marks a registered verb whose implementation is intentionally
	// not wired yet. The dispatcher reports the deferral and exits 0; the
	// handler-parity test treats a deferred verb as satisfied.
	Deferred bool `json:"deferred,omitempty"`
}

// anyPhase is the explicit unrestricted declaration.
func anyPhase() []Phase { return []Phase{PhaseAny} }

// postRequirementsPhases is the set for execution verbs (verify, next): every
// phase except perceive. A spec still in the requirements (perceive) phase has
// no approved design or task DAG to act on, so these verbs fail closed there
// (spec 03 R2 acceptance: "execution verb on a spec still in requirements phase
// exits 2"). The finer approval check (requireTaskGate) still applies inside
// the handler; this is the coarse metadata-driven guard.
func postRequirementsPhases() []Phase {
	return []Phase{PhaseAnalyze, PhasePlan, PhaseExecute, PhaseVerify, PhaseReflect}
}

// stdCodes is the conventional exit-code table; every verb declares at least
// codes 0 and 2 (spec 03 design notes / test invariant).
func stdCodes() []ExitCode {
	return []ExitCode{
		{Code: 0, Meaning: "success"},
		{Code: 1, Meaning: "gate or verify failure"},
		{Code: 2, Meaning: "usage error or fail-closed rejection"},
	}
}

// argAt returns a pointer to i for the SpecSlugArg field.
func argAt(i int) *int { return &i }

// Commands is the stable top-level command palette.
var Commands = []Command{
	{
		Name:          "help",
		Usage:         "specd help [command] [--json]",
		Description:   "Show command help.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd help", "specd help --json"},
		Flags: []Flag{
			{Name: "json", Type: "bool", Description: "Emit machine-readable help."},
		},
	},
	{
		Name:          "version",
		Usage:         "specd version [--json]",
		Description:   "Print build version metadata.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd version", "specd version --json"},
		Flags:         []Flag{{Name: "json", Type: "bool", Description: "Emit machine-readable JSON."}},
	},
	{
		Name:          "init",
		Usage:         "specd init [--agent=<name>]",
		Description:   "Initialize specd project state.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd init", "specd init --agent=codex"},
		Flags: []Flag{
			{Name: "agent", TakesValue: true, Type: "string", Description: "Select agent harness."},
		},
	},
	{
		Name:          "new",
		Usage:         "specd new <name> [--agent=<name>]",
		Description:   "Create a new spec workspace.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd new payments", "specd new payments --agent=codex"},
		Flags: []Flag{
			{Name: "agent", TakesValue: true, Type: "string", Description: "Select agent harness."},
		},
	},
	{
		Name:          "approve",
		Usage:         "specd approve <spec> <gate>",
		Description:   "Record human approval for a lifecycle gate.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd approve payments requirements", "specd approve payments design"},
	},
	{
		Name:          "midreq",
		Usage:         "specd midreq <spec> --text <change> [--scope <scope>]",
		Description:   "Capture a scoped mid-stream requirement change.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd midreq payments --text 'add refund path' --scope requirements"},
		Flags: []Flag{
			{Name: "text", TakesValue: true, Type: "string", Description: "Change description (required)."},
			{Name: "scope", TakesValue: true, Type: "string", Description: "Optional scope label."},
		},
	},
	{
		Name:          "decision",
		Usage:         "specd decision <spec> --text <rationale> [--scope <scope>]",
		Description:   "Record an explicit human decision.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd decision payments --text 'defer webhooks' --scope design"},
		Flags: []Flag{
			{Name: "text", TakesValue: true, Type: "string", Description: "Decision rationale (required)."},
			{Name: "scope", TakesValue: true, Type: "string", Description: "Optional scope label."},
		},
	},
	{
		Name:          "next",
		Usage:         "specd next <slug> [--json | --waves | --dispatch]",
		Description:   "Select the next eligible task or wave.",
		AllowedPhases: postRequirementsPhases(),
		SpecSlugArg:   argAt(0),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd next payments", "specd next payments --json"},
		Flags: []Flag{
			{Name: "waves", Type: "bool", Description: "Show all wave groups as JSON."},
			{Name: "dispatch", Type: "bool", Description: "Emit the context manifest for the first frontier task."},
			{Name: "json", Type: "bool", Description: "Emit machine-readable frontier list."},
		},
	},
	{
		Name:          "status",
		Usage:         "specd status [spec] [--json]",
		Description:   "Report current spec and task state.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd status payments", "specd status payments --json"},
		Flags: []Flag{
			{Name: "json", Type: "bool", Description: "Emit machine-readable status."},
		},
	},
	{
		Name:          "task",
		Usage:         "specd task <id> | specd task complete <spec> <id>",
		Description:   "Show task details or mark a task complete (requires passing evidence).",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd task T3 --json", "specd task complete payments T3"},
		Flags: []Flag{
			{Name: "json", Type: "bool", Description: "Emit machine-readable task row."},
		},
	},
	{
		Name:          "check",
		Usage:         "specd check <spec> [--security] [--json]",
		Description:   "Run the validation gate registry against a spec.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd check payments", "specd check payments --security --json"},
		Flags: []Flag{
			{Name: "security", Type: "bool", Description: "Run opt-in security gates."},
			{Name: "schema", Type: "bool", Description: "Validate state.json schema."},
			{Name: "schema-only", Type: "bool", Description: "Validate only state.json schema."},
			{Name: "json", Type: "bool", Description: "Emit machine-readable findings."},
		},
	},
	{
		Name:          "verify",
		Usage:         "specd verify <slug> <task-id> [--revert-on-fail] [--sandbox] [--sandbox-binary=<path>] | specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence <text>",
		Description:   "Run and record task verification (task mode), or record a per-acceptance-criterion evidence record (--criterion mode).",
		AllowedPhases: postRequirementsPhases(),
		SpecSlugArg:   argAt(0),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd verify payments T3", "specd verify payments T3 --revert-on-fail", "specd verify payments --criterion 1.2 --status pass --evidence 'covered by T3 integration test'"},
		Flags: []Flag{
			{Name: "revert-on-fail", Type: "bool", Description: "Restore working tree on verify failure."},
			{Name: "sandbox", Type: "bool", Description: "Run inside bwrap/container sandbox (config.verify.sandbox)."},
			{Name: "sandbox-binary", TakesValue: true, Type: "string", Description: "Path to sandbox binary (overrides auto-detect)."},
			{Name: "criterion", TakesValue: true, Type: "string", Description: "Record evidence for acceptance criterion <r>.<n> instead of running a task verify."},
			{Name: "status", TakesValue: true, Type: "string", Enum: []string{"pass", "fail"}, Description: "Criterion verdict (with --criterion): pass|fail."},
			{Name: "evidence", TakesValue: true, Type: "string", Description: "Evidence text or path backing the criterion verdict (with --criterion)."},
		},
	},
	{
		Name:          "context",
		Usage:         "specd context <slug> <task-id> [--json|--hud]",
		Description:   "Build the bounded context manifest for a task.",
		AllowedPhases: postRequirementsPhases(),
		SpecSlugArg:   argAt(0),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd context payments T3", "specd context payments T3 --hud"},
		Flags: []Flag{
			{Name: "json", Type: "bool", Description: "Emit machine-readable context."},
			{Name: "hud", Type: "bool", Description: "Render the operator HUD (files, bytes, tokens, mode)."},
		},
	},
	{
		Name:          "memory",
		Usage:         "specd memory <slug> <add|promote> [flags]",
		Description:   "Append or promote steering-memory patterns (learning flywheel).",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd memory payments add --key 'atomic writes' --pattern 'use AtomicWrite'"},
		Flags: []Flag{
			{Name: "key", TakesValue: true, Type: "string", Description: "Pattern key (H2 heading)."},
			{Name: "pattern", TakesValue: true, Type: "string", Description: "One-line pattern statement (add)."},
			{Name: "body", TakesValue: true, Type: "string", Description: "Detail of the pattern (add)."},
			{Name: "source", TakesValue: true, Type: "string", Description: "Where the pattern came from (add)."},
			{Name: "criticality", TakesValue: true, Type: "string", Enum: []string{"minor", "important", "critical"}, Description: "minor|important|critical (add)."},
			{Name: "related", TakesValue: true, Type: "string", Description: "Comma-separated related keys → wikilinks (add)."},
			{Name: "force", Type: "bool", Description: "Promote past the threshold (promote)."},
		},
	},
	{
		Name:          "mcp",
		Usage:         "specd mcp",
		Description:   "Serve the MCP integration surface over stdio.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd mcp"},
	},
	{
		Name:          "handshake",
		Usage:         "specd handshake [bootstrap|policy]",
		Description:   "Emit bootstrap or policy handshake material.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd handshake bootstrap", "specd handshake bootstrap --json"},
	},
	{
		Name:          "brain",
		Usage:         "specd brain <start|step|run|status> <spec> [--authority]",
		Description:   "Run the opt-in deterministic orchestration controller.",
		AllowedPhases: postRequirementsPhases(),
		SpecSlugArg:   argAt(1),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd brain start payments --authority", "specd brain status payments"},
		Flags: []Flag{
			{Name: "authority", Type: "bool", Description: "Grant dispatch authority (fail-closed by default)."},
		},
	},
	{
		Name:          "report",
		Usage:         "specd report <spec> [--pr|--metrics|--json]",
		Description:   "Render evidence-backed status and PR reports.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd report payments --pr", "specd report payments --metrics"},
		Flags: []Flag{
			{Name: "pr", Type: "bool", Description: "Emit PR-oriented report."},
			{Name: "metrics", Type: "bool", Description: "Emit metrics summary."},
			{Name: "json", Type: "bool", Description: "Emit machine-readable report."},
		},
	},
	{
		Name:          "triage",
		Usage:         "specd triage <spec>",
		Description:   "Run the opt-in extended-loop triage tier.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd triage payments"},
		Deferred:      true,
	},
}

// CommandNames returns command names in help order.
func CommandNames() []string {
	names := make([]string, len(Commands))
	for i, command := range Commands {
		names[i] = command.Name
	}
	return names
}

// CommandByName returns the command metadata for name, and whether it exists.
func CommandByName(name string) (Command, bool) {
	for _, command := range Commands {
		if command.Name == name {
			return command, true
		}
	}
	return Command{}, false
}

// AllowsPhase reports whether the command may run in phase. A command that
// declares PhaseAny is unrestricted.
func (c Command) AllowsPhase(phase Phase) bool {
	for _, allowed := range c.AllowedPhases {
		if allowed == PhaseAny || allowed == phase {
			return true
		}
	}
	return false
}

// FlagByName returns the flag metadata for name, or nil if the command has no
// such flag.
func (c Command) FlagByName(name string) *Flag {
	for i := range c.Flags {
		if c.Flags[i].Name == name {
			return &c.Flags[i]
		}
	}
	return nil
}

// HelpPayload is the stable machine-readable help contract emitted by
// `help --json`. SchemaVersion lets consumers detect shape changes.
type HelpPayload struct {
	SchemaVersion int       `json:"schema_version"`
	Commands      []Command `json:"commands"`
}

// BuildHelpPayload assembles the full palette for `help --json`.
func BuildHelpPayload() HelpPayload {
	return HelpPayload{SchemaVersion: HelpSchemaVersion, Commands: Commands}
}
