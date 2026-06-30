package core

type PositionalMeta struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Repeatable  bool   `json:"repeatable,omitempty"`
	Description string `json:"description,omitempty"`
}

type FlagMeta struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Default     string   `json:"default,omitempty"`
}

type PhaseCompatibilityMeta struct {
	Statuses []string `json:"statuses,omitempty"`
	Phases   []string `json:"phases,omitempty"`
}

type ModeCompatibilityMeta struct {
	Modes                           []string `json:"modes,omitempty"`
	RequiresOrchestrationCapability bool     `json:"requiresOrchestrationCapability,omitempty"`
}

type ExitCodeMeta struct {
	Code    int    `json:"code"`
	Meaning string `json:"meaning"`
}

type CommandMeta struct {
	Command            string                  `json:"command"`
	Category           string                  `json:"category"`
	Description        string                  `json:"description"`
	Usage              string                  `json:"usage"`
	Synopsis           string                  `json:"synopsis"`
	LongDescription    string                  `json:"longDescription"`
	Flags              []FlagMeta              `json:"flags"`
	Positionals        []PositionalMeta        `json:"positionals,omitempty"`
	PhaseCompatibility *PhaseCompatibilityMeta `json:"phaseCompatibility,omitempty"`
	ModeCompatibility  *ModeCompatibilityMeta  `json:"modeCompatibility,omitempty"`
	ExitCodes          []ExitCodeMeta          `json:"exitCodes"`
	Examples           []string                `json:"examples"`
	Hidden             bool                    `json:"hidden,omitempty"`
	DeprecatedIn       string                  `json:"deprecatedIn,omitempty"`
	RemovedIn          string                  `json:"removedIn,omitempty"`
}

var Commands = []CommandMeta{
	{
		Command: "init", Category: "lifecycle",
		Description: "Scaffold project assets and configure coding agents",
		Usage:       "specd init [--agent <auto|all|none|codex|claude-code|cursor|antigravity|vscode>] [--scope project|global] [--yes] [--non-interactive] [--verbose] [--dry-run] [--repair|--refresh|--force] [--migrate] [--orchestration [<policy>]] [--orchestration-workers <n>] [--orchestration-retries <n>] [--orchestration-timeout <minutes>] [--orchestration-cost-limit <usd>] [--orchestration-mode <inline|delegate>] [--orchestration-sandbox <none|bwrap|container>]", Synopsis: "specd init [--agent <name>] [--yes] [--dry-run]",
		LongDescription: "Scaffolds .specd/ and AGENTS.md, passively detects supported coding-agent hosts, optionally installs project-scoped MCP registration, verifies the in-process MCP server, and returns one next action. Non-interactive auto-detection never mutates host configuration unless --yes is supplied. Global scope requires explicit consent.",
		Flags:           []FlagMeta{{Name: "agent", Type: "string", Description: "Coding-agent selection: auto, all, none, codex, claude-code, cursor, antigravity, or vscode"}, {Name: "scope", Type: "string", Description: "Integration scope (default project)"}, {Name: "yes", Type: "boolean", Description: "Accept non-destructive project-scoped integration changes"}, {Name: "non-interactive", Type: "boolean", Description: "Disable prompts"}, {Name: "verbose", Type: "boolean", Description: "Include detailed path results"}, {Name: "json", Type: "boolean", Description: "Output one versioned InitResult document"}, {Name: "dry-run", Type: "boolean", Description: "Preview exact actions without writing"}, {Name: "repair", Type: "boolean", Description: "Restore missing managed assets only"}, {Name: "migrate", Type: "boolean", Description: "Migrate legacy config.json to config.yml"}, {Name: "refresh", Type: "boolean", Description: "Refresh frozen managed assets and AGENTS.md markers"}, {Name: "force", Type: "boolean", Description: "Destructively overwrite all scaffold files and AGENTS.md"}, {Name: "list-packs", Type: "boolean", Description: "List the embedded spec packs and exit"}, {Name: "pack", Type: "string", Description: "Apply a spec pack by built-in name or http(s) URL"}, {Name: "sha256", Type: "string", Description: "Pinned SHA256 digest required for a remote --pack URL"}, {Name: "orchestration", Type: "string", Description: "Enable Brain/Pinky and set approval policy (manual, planning, session)"}, {Name: "orchestration-workers", Type: "string", Description: "Max concurrent Pinky workers (1..64, default 4)"}, {Name: "orchestration-retries", Type: "string", Description: "Retry budget for failed/reclaimed work (0..10, default 2)"}, {Name: "orchestration-timeout", Type: "string", Description: "Session wall-clock timeout in minutes (1..1440, default 120)"}, {Name: "orchestration-cost-limit", Type: "string", Description: "Host-reported cost brake in USD (default 0)"}, {Name: "orchestration-mode", Type: "string", Description: "Subagent coordination mode: inline or delegate (default delegate)"}, {Name: "orchestration-sandbox", Type: "string", Description: "Default verify sandbox: none, bwrap, or container (default none)"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Initialization or pack operation failed"}, {2, "Usage error"}},
		Examples:        []string{"specd init --agent auto --yes", "specd init --agent none --non-interactive", "specd init --agent all --dry-run --json", "specd init --repair"},
	},

	{
		Command: "doctor", Category: "inspection", Hidden: true, DeprecatedIn: "palette-merge",
		Description: "Diagnose scaffold, MCP, and coding-agent registration",
		Usage:       "specd doctor [--agent <name|all>] [--fix] [--json]", Synopsis: "specd doctor [--agent <name|all>] [--fix] [--json]",
		LongDescription: "Checks the current project binary, required scaffold assets, in-process MCP handshake and baseline tools, detected coding-agent hosts, and project registration state. --fix repairs only safe project-scoped state and refuses conflicting or unowned registrations.",
		Flags:           []FlagMeta{{Name: "agent", Type: "string", Description: "Inspect one host or all detected hosts"}, {Name: "fix", Type: "boolean", Description: "Repair safe project-scoped scaffold and registration state"}, {Name: "json", Type: "boolean", Description: "Output one deterministic JSON document"}},
		ExitCodes:       []ExitCodeMeta{{0, "All requested checks pass"}, {1, "One or more health checks failed"}, {2, "Usage error"}},
		Examples:        []string{"specd doctor", "specd doctor --agent codex --json", "specd doctor --fix"},
	},

	{
		Command: "migrate", Category: "lifecycle", Hidden: true, DeprecatedIn: "palette-merge",
		Description: "Migrate legacy config.json to config.yml",
		Usage:       "specd migrate config [--dry-run] [--global]", Synopsis: "specd migrate config [--dry-run] [--global]",
		LongDescription: "Converts legacy JSON configuration to deterministic YAML, validates the rendered config, then renames the JSON file to .bak. --dry-run prints YAML without mutating files. --global migrates the user-level legacy config and does not require a project root.",
		Flags:           []FlagMeta{{Name: "dry-run", Type: "boolean", Description: "Print YAML without writing or renaming"}, {Name: "global", Type: "boolean", Description: "Migrate the legacy global config"}, {Name: "json", Type: "boolean", Description: "Output migration result as JSON"}},
		Positionals:     []PositionalMeta{{Name: "subcommand", Required: true, Description: "config"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Migration failed validation or safety checks"}, {2, "Usage error"}, {3, ".specd/ or legacy config not found"}},
		Examples:        []string{"specd migrate config --dry-run", "specd migrate config", "specd migrate config --global"},
	},

	{
		Command: "fusion", Category: "inspection", Hidden: true,
		Description: "Emit startup bootstrap and binding policy oracles",
		Usage:       "specd fusion bootstrap [--include-schema] [--json] | specd fusion policy [<slug>] [--expect-config-digest <sha256>] [--json]", Synopsis: "specd fusion <bootstrap|policy> [slug] [--json]",
		LongDescription: "Read-only agent integration surface. bootstrap returns the first-turn load list, command schema digest, config digest, health, and active modes. policy summarizes binding config, optional spec execution mode, allowed loop family, diagnostics, and config digest drift.",
		Flags:           []FlagMeta{{Name: "include-schema", Type: "boolean", Description: "Inline full command schema in bootstrap output"}, {Name: "expect-config-digest", Type: "string", Description: "Fail if current config digest differs from this sha256"}, {Name: "json", Type: "boolean", Description: "Output JSON"}},
		Positionals:     []PositionalMeta{{Name: "subcommand", Required: true, Description: "bootstrap or policy"}, {Name: "slug", Required: false, Description: "Spec slug for policy mode"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Policy violation or digest mismatch"}, {2, "Usage error"}, {3, ".specd/ or spec not found"}},
		Examples:        []string{"specd fusion bootstrap --json", "specd fusion policy my-feature --json", "specd fusion policy --expect-config-digest <sha256> --json"},
	},

	{
		Command: "new", Category: "lifecycle",
		Description: "Create a spec with six artifacts",
		Usage:       "specd new <slug> [--title \"...\"] [--orchestrated]", Synopsis: "specd new <slug> [--title \"...\"] [--orchestrated]",
		LongDescription: "Creates a new spec directory under .specd/specs/<slug>/ with six artifact stubs. Specs default to Base execution mode; --orchestrated records executionMode=orchestrated (origin user) and requires project orchestration capability.",
		Flags:           []FlagMeta{{Name: "title", Type: "string", Description: "The title of the spec"}, {Name: "orchestrated", Type: "boolean", Description: "Create the spec in orchestrated (Brain/Pinky) mode; requires project orchestration capability"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Orchestration requested without project capability"}, {2, "Usage error"}, {3, ".specd/ not found or spec already exists"}},
		Examples:        []string{"specd new my-feature", "specd new my-feature --title \"My Feature\"", "specd new payments --title \"Billing\" --orchestrated"},
	},

	{
		Command: "approve", Category: "lifecycle",
		Description: "Clear approval gate / advance phase",
		Usage:       "specd approve <slug> [--json]", Synopsis: "specd approve <slug> [--json]",
		LongDescription: "Clears an awaiting-approval gate or advances the planning phase of the specified spec.",
		Flags:           []FlagMeta{{Name: "json", Type: "boolean", Description: "Output in JSON format"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Gate validation failed"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd approve my-feature", "specd approve my-feature --json"},
	},

	{
		Command: "decision", Category: "lifecycle",
		Description: "Record an architectural decision (ADR)",
		Usage:       "specd decision <slug> \"<text>\" [--supersedes <id>]", Synopsis: "specd decision <slug> \"<text>\" [--supersedes <id>]",
		LongDescription: "Appends an architectural decision record (ADR) to decisions.md.",
		Flags:           []FlagMeta{{Name: "supersedes", Type: "string", Description: "ID of the decision being superseded"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd decision my-feature \"Use SQLite instead of PostgreSQL\""},
	},

	{
		Command: "midreq", Category: "lifecycle",
		Description: "Log feedback mid-requirement",
		Usage:       "specd midreq <slug> \"<input>\" --impact <low|medium|high|critical>", Synopsis: "specd midreq <slug> \"<input>\" --impact <low|medium|high|critical>",
		LongDescription: "Logs user feedback or mid-requirement changes and triggers appropriate gates.",
		Flags:           []FlagMeta{{Name: "impact", Type: "string", Description: "Impact level (low/medium/high/critical)"}, {Name: "interpretation", Type: "string"}, {Name: "changes", Type: "string"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd midreq my-feature \"Add search bar\" --impact medium"},
	},

	{
		Command: "memory", Category: "lifecycle",
		Description: "Add or promote learning items",
		Usage:       "specd memory <slug> add|promote [flags]", Synopsis: "specd memory <slug> <add|promote> [flags]",
		LongDescription: "Manages persistent project learnings.",
		Flags:           []FlagMeta{{Name: "key", Type: "string"}, {Name: "pattern", Type: "string"}, {Name: "body", Type: "string"}, {Name: "source", Type: "string"}, {Name: "criticality", Type: "string"}, {Name: "related", Type: "string"}, {Name: "force", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd memory my-feature add --key db-lock --pattern \"parallel writes\" --body \"SQLite locks on concurrent write\" --source log --criticality important"},
	},

	{
		Command: "next", Category: "execution",
		Description: "Next runnable task",
		Usage:       "specd next <slug> [--all] [--dispatch] [--json]", Synopsis: "specd next <slug> [--all] [--dispatch] [--json]",
		LongDescription: "Finds and prints the next runnable task in the spec's task DAG.",
		Flags:           []FlagMeta{{Name: "all", Type: "boolean", Description: "Print all runnable tasks"}, {Name: "dispatch", Type: "boolean", Description: "Emit ready-to-run dispatch packets for the runnable frontier"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd next my-feature", "specd next my-feature --all --json"},
	},

	{
		Command: "dispatch", Category: "execution", Hidden: true, DeprecatedIn: "palette-merge",
		Description: "Ready-to-run packets for frontier",
		Usage:       "specd dispatch <slug> [--json]", Synopsis: "specd dispatch <slug> [--json]",
		LongDescription: "Produces ready-to-run packets for all tasks in the current runnable frontier.",
		Flags:           []FlagMeta{{Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd dispatch my-feature --json"},
	},

	{
		Command: "verify", Category: "execution",
		Description:     "Run task verify command / record proof",
		Usage:           "specd verify <slug> <id>  |  specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence \"...\"",
		Synopsis:        "specd verify <slug> [id] [flags]",
		LongDescription: "Runs a task's verification command and records the result, or records a per-criterion acceptance proof.",
		Flags:           []FlagMeta{{Name: "criterion", Type: "string"}, {Name: "status", Type: "string"}, {Name: "evidence", Type: "string"}, {Name: "revert-on-fail", Type: "boolean", Description: "On a failed verify, stash the working tree (recoverable) instead of leaving it dirty"}, {Name: "sandbox", Type: "string", Description: "Isolation backend for this run (none|bwrap|container); overrides verify.sandbox config"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Verification failed"}, {2, "Usage error"}, {3, "Spec or task not found"}},
		Examples:        []string{"specd verify my-feature T1", "specd verify my-feature --criterion 1.1 --status pass --evidence \"Tested manually\""},
	},

	{
		Command: "task", Category: "execution",
		Description:     "Evidence-gated status flip",
		Usage:           "specd task <slug> <id> --status <s> [--evidence \"...\"] [--reason \"...\"] [--force]",
		Synopsis:        "specd task <slug> <id> --status <status> [flags]",
		LongDescription: "Updates the status of a specific task. Completing requires a passing verify record.",
		Flags:           []FlagMeta{{Name: "status", Type: "string"}, {Name: "evidence", Type: "string"}, {Name: "reason", Type: "string"}, {Name: "force", Type: "boolean"}, {Name: "unverified", Type: "boolean"}, {Name: "tokens", Type: "string", Description: "Annotate task telemetry with a token count (stored, not computed)"}, {Name: "cost", Type: "string", Description: "Annotate task telemetry with a cost (stored, not computed)"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Gate verification failed"}, {2, "Usage error"}, {3, "Spec or task not found"}},
		Examples:        []string{"specd task my-feature T1 --status running", "specd task my-feature T1 --status complete --evidence \"Ran tests\"", "specd task my-feature T1 --status complete --evidence \"…\" --tokens 12000 --cost 0.42"},
	},

	{
		Command: "status", Category: "inspection",
		Description: "Render durable ledger / board",
		Usage:       "specd status [<slug>] [--all] [--program] [--set-mode base|orchestrated] [--recommend] [--json]", Synopsis: "specd status [<slug>] [--all] [--program] [--json]",
		LongDescription: "Renders the durable status board of a specific spec, lists all specs, or displays the cross-spec program frontier. With a slug, --set-mode records a new per-spec execution mode (orchestrated requires project capability; switching to base is refused while a Brain session is active) and --recommend emits a deterministic, advisory mode recommendation — the survivor home for the merged `mode` command's set/recommend paths.",
		Flags:           []FlagMeta{{Name: "all", Type: "boolean", Description: "List all specs (default when no slug is supplied)"}, {Name: "program", Type: "boolean", Description: "Show the cross-spec program frontier"}, {Name: "set-mode", Type: "string", Description: "Set the spec's execution mode: base or orchestrated"}, {Name: "recommend", Type: "boolean", Description: "Emit an advisory mode recommendation from countable spec facts"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Capability missing or session-active refusal"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd status", "specd status my-feature --json", "specd status my-feature --set-mode orchestrated", "specd status my-feature --recommend --json"},
	},

	{
		Command: "mode", Category: "inspection", Hidden: true, DeprecatedIn: "palette-merge",
		Description: "Show or set a spec's execution mode",
		Usage:       "specd mode <slug> [--set base|orchestrated] [--recommend] [--json]", Synopsis: "specd mode <slug> [--set base|orchestrated] [--recommend] [--json]",
		LongDescription: "Shows the effective execution mode (base | orchestrated), its origin, and project orchestration capability for a spec. --set records a new per-spec mode (orchestrated requires project capability; switching to base is refused while a Brain session is active). --recommend emits a deterministic, advisory recommendation computed from on-disk spec facts — the user decides.",
		Flags:           []FlagMeta{{Name: "set", Type: "string", Description: "Set the spec's execution mode: base or orchestrated"}, {Name: "recommend", Type: "boolean", Description: "Emit an advisory mode recommendation from countable spec facts"}, {Name: "json", Type: "boolean", Description: "Output in JSON format"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Capability missing or session-active refusal"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd mode my-feature", "specd mode my-feature --set orchestrated", "specd mode my-feature --recommend --json"},
	},

	{
		Command: "check", Category: "inspection",
		Description: "Run all validation gates",
		Usage:       "specd check <slug> [--schema-only] [--json] | specd check --schema", Synopsis: "specd check <slug> [--schema-only] [--json] | specd check --schema",
		LongDescription: "Runs all seven validation gates on the specified spec. --schema-only validates state.json against the embedded open spec schema; --schema emits that schema.",
		Flags:           []FlagMeta{{Name: "schema-only", Type: "boolean", Description: "Validate state.json against the embedded open spec schema only"}, {Name: "schema", Type: "boolean", Description: "Emit the embedded open spec format JSON Schema"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Validation failed"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd check my-feature", "specd check my-feature --json"},
	},

	{
		Command: "context", Category: "inspection",
		Description: "Phase-scoped briefing",
		Usage:       "specd context <slug> [--json]", Synopsis: "specd context <slug> [--json]",
		LongDescription: "Provides a minimal phase-scoped briefing for the current spec phase.",
		Flags:           []FlagMeta{{Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd context my-feature", "specd context my-feature --json"},
	},

	{
		Command: "validate", Category: "inspection", Hidden: true, DeprecatedIn: "palette-merge",
		Description:     "Check state.json against the open spec schema",
		Usage:           "specd validate <slug> --schema [--version <v>]",
		Synopsis:        "specd validate <slug> --schema [--version <v>]",
		LongDescription: "Validates a spec's on-disk state.json against the embedded open spec format JSON Schema. A structural/format conformance mode (shape, required keys, closed property sets, enums) — independent of the seven semantic gates in `specd check`. Read-only. Text by default; JSON under SPECD_JSON.",
		Flags:           []FlagMeta{{Name: "schema", Type: "boolean", Description: "Validate against the embedded JSON Schema (required)"}, {Name: "version", Type: "string", Description: "Schema version to validate against (default: current)"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Conformant"}, {1, "Conformance violations found"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd validate my-feature --schema", "SPECD_JSON=1 specd validate my-feature --schema"},
	},

	{
		Command: "schema", Category: "inspection", Hidden: true, DeprecatedIn: "palette-merge",
		Description:     "Emit the embedded open spec format JSON Schema",
		Usage:           "specd schema [--version <v>]",
		Synopsis:        "specd schema [--version <v>]",
		LongDescription: "Writes the embedded, versioned JSON Schema for the specd on-disk artifacts to stdout. The schema is the published, machine-readable format contract; the Go types in internal/core are its single source of truth (a conformance test fails CI on drift). Requires no .specd/ root.",
		Flags:           []FlagMeta{{Name: "version", Type: "string", Description: "Schema version to emit (default: current)"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Unknown schema version"}, {2, "Usage error"}},
		Examples:        []string{"specd schema", "specd schema --version 1 > spec-schema.json"},
	},

	{
		Command: "report", Category: "inspection",
		Description: "Generate markdown, HTML, or metrics report",
		Usage:       "specd report <slug> [--format md|html|prometheus] [--out <path>] [--pr-summary] [--serve|--watch|--history|--diff]", Synopsis: "specd report <slug> [--format md|html|prometheus] [--out <path>] [--pr-summary]",
		LongDescription: "Compiles a comprehensive HTML or Markdown progress report, or an opt-in Prometheus textfile metrics view. With --pr-summary, emits a deterministic, network-free pull-request summary (Markdown, or JSON under SPECD_JSON): wave/task progress, gate status, and the commit↔task link map.",
		Flags:           []FlagMeta{{Name: "format", Type: "string", Description: "Output format: md, html, or prometheus"}, {Name: "out", Type: "string"}, {Name: "pr-summary", Type: "boolean", Description: "Emit a deterministic PR summary instead of the full report"}, {Name: "serve", Type: "boolean", Description: "Serve the live dashboard"}, {Name: "watch", Type: "boolean", Description: "Stream runnable-frontier changes"}, {Name: "history", Type: "boolean", Description: "Replay the spec audit timeline"}, {Name: "diff", Type: "boolean", Description: "Diff spec artifacts between git refs"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd report my-feature --format html --out ./report.html", "specd report my-feature --format prometheus"},
	},

	{
		Command: "serve", Category: "inspection", Hidden: true, DeprecatedIn: "palette-merge",
		Description: "Serve a read-only live dashboard",
		Usage:       "specd serve <slug> [--addr 127.0.0.1:8765]", Synopsis: "specd serve <slug> [--addr 127.0.0.1:8765]",
		LongDescription: "Starts a read-only HTTP server (loopback by default) that renders the same HTML as `specd report --format html` at GET /, and the JSON ReportData at GET /api/report. No mutating routes; data is rebuilt from disk per request.",
		Flags:           []FlagMeta{{Name: "addr", Type: "string", Description: "Bind address (default 127.0.0.1:8765)"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd serve my-feature", "specd serve my-feature --addr 127.0.0.1:9000"},
	},

	{
		Command: "replay", Category: "inspection", Hidden: true, DeprecatedIn: "palette-merge",
		Description: "Replay a spec's audit timeline",
		Usage:       "specd replay <slug> [--acp-session <id>]", Synopsis: "specd replay <slug> [--acp-session <id>]",
		LongDescription: "Reconstructs a deterministic, read-only event timeline (task start/finish/verify/block + acceptance records) from the spec's on-disk audit data. Text by default; a typed JSON array under SPECD_JSON.",
		Flags:           []FlagMeta{{Name: "acp-session", Type: "string", Description: "Replay events for a specific orchestration session ID"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd replay my-feature", "specd replay my-feature --acp-session 11111111111111111111111111111111", "SPECD_JSON=1 specd replay my-feature"},
	},

	{
		Command: "diff", Category: "inspection", Hidden: true, DeprecatedIn: "palette-merge",
		Description:     "Diff a spec's artifacts between two git refs",
		Usage:           "specd diff <slug> --from <ref> [--to <ref>]",
		Synopsis:        "specd diff <slug> --from <ref> [--to <ref>]",
		LongDescription: "Shows how a spec's on-disk artifacts changed between two git refs — a deterministic, read-only `git diff --name-status` scoped to the spec directory. --from is required; --to defaults to the working tree. Text by default; a typed JSON object under SPECD_JSON.",
		Flags:           []FlagMeta{{Name: "from", Type: "string", Description: "Base git ref (required)"}, {Name: "to", Type: "string", Description: "Target git ref (default: working tree)"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "git diff failed (bad ref / not a repo)"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd diff my-feature --from HEAD~5", "specd diff my-feature --from v0.1.0 --to HEAD"},
	},

	{
		Command: "watch", Category: "inspection", Hidden: true, DeprecatedIn: "palette-merge",
		Description: "Stream runnable-frontier changes (NDJSON / SSE / webhook)",
		Usage:       "specd watch [--once] [--spec <slug>] [--sse <addr>] [--webhook <url>]", Synopsis: "specd watch [--once] [--spec <slug>] [--sse <addr>] [--webhook <url>]",
		LongDescription: "Watches spec state and emits a FrontierEvent whenever a spec's runnable task set changes. Read-only: it never writes state. Transports: by default NDJSON on stdout; --sse <addr> serves Server-Sent Events at GET /events over net/http; --webhook <url> POSTs each event on a non-blocking background worker with bounded retry/backoff. --once does a single pass and exits; long-running modes shut down cleanly on SIGINT/SIGTERM. Polls at SPECD_WATCH_INTERVAL_MS (default 1000ms). Stdlib-only.",
		Flags:           []FlagMeta{{Name: "once", Type: "boolean", Description: "Emit current frontiers once and exit"}, {Name: "spec", Type: "string", Description: "Limit the stream to a single spec slug"}, {Name: "sse", Type: "string", Description: "Serve Server-Sent Events at this bind address (e.g. 127.0.0.1:8766)"}, {Name: "webhook", Type: "string", Description: "POST each frontier event to this URL"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, ".specd/ not found"}},
		Examples:        []string{"specd watch --once", "specd watch --spec my-feature", "specd watch --sse 127.0.0.1:8766", "specd watch --webhook https://ci.example/hook"},
	},

	{
		Command: "waves", Category: "inspection",
		Description: "Show task wave DAG",
		Usage:       "specd waves <slug> [--json]", Synopsis: "specd waves <slug> [--json]",
		LongDescription: "Renders the task wave dependency graph in ASCII or JSON.",
		Flags:           []FlagMeta{{Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd waves my-feature"},
	},

	{
		Command:     "brain",
		Category:    "orchestration",
		Description: "Drive deterministic Brain orchestration sessions.",
		Usage:       "specd brain <start|status|step|pause|resume|cancel|checkpoint> ... [--program] [--auto-step|--verbose|--ledger|--directive|--compact]",
		Synopsis:    "specd brain <start|status|step|pause|resume|cancel|checkpoint> ... [--program]",
		Flags: []FlagMeta{
			{Name: "program", Type: "boolean", Description: "operate on the cross-spec program session instead of one spec"},
			{Name: "auto-step", Type: "boolean", Description: "Start then drive the Brain loop (merged from brain run)"},
			{Name: "verbose", Type: "boolean", Description: "Explain scheduling state (merged from brain why)"},
			{Name: "ledger", Type: "boolean", Description: "Show the session context ledger"},
			{Name: "compact", Type: "boolean", Description: "Compact/checkpoint context before host clear"},
			{Name: "directive", Type: "boolean", Description: "Record a host directive on brain step"},
			{Name: "session", Type: "string", Description: "explicit orchestration session id"},
			{Name: "approval-policy", Type: "string", Description: "required approval policy for start/step"},
			{Name: "max-workers", Type: "string", Description: "required worker concurrency limit for start/step"},
			{Name: "max-retries", Type: "string", Description: "required retry limit for start/step"},
			{Name: "timeout-seconds", Type: "string", Description: "required session timeout for start/step"},
			{Name: "cost-limit", Type: "string", Description: "optional host-reported cost limit"},
			{Name: "worker-cmd", Type: "string", Description: "host shell command to spawn Pinky workers"},
			{Name: "bootstrap", Type: "boolean", Description: "automatically bootstrap missing specs during run"},
			{Name: "max-steps", Type: "string", Description: "max driver loop steps (default 100 for single, 200 for program)"},
			{Name: "title", Type: "string", Description: "title to use when bootstrapping a missing spec"},
			{Name: "worker", Type: "string", Description: "worker id for directive"},
			{Name: "spec", Type: "string", Description: "spec slug for directive"},
			{Name: "task", Type: "string", Description: "task id for directive"},
			{Name: "attempt", Type: "string", Description: "positive attempt number for directive"},
			{Name: "action", Type: "string", Description: "directive action: continue, retry, cancel, reassign, or escalate"},
			{Name: "reason", Type: "string", Description: "directive reason"},
			{Name: "in-reply-to", Type: "string", Description: "query message id this directive answers"},
			{Name: "json", Type: "boolean", Description: "emit JSON"},
		},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {1, "Gate or validation failure"}, {2, "Usage error"}, {3, "Workspace or session not found"}},
		Examples:  []string{"specd brain run my-spec --worker-cmd './worker.sh'", "specd brain resume --session 11111111111111111111111111111111", "specd brain start my-spec --approval-policy manual --max-workers 4 --max-retries 2 --timeout-seconds 7200 --json", "specd brain directive --session 11111111111111111111111111111111 --worker t1-a1 --spec my-spec --task T1 --attempt 1 --action continue --reason 'docs not required'", "specd brain step my-spec --session 11111111111111111111111111111111 --approval-policy manual --max-workers 4 --max-retries 2 --timeout-seconds 7200"},
	},

	{
		Command:     "pinky",
		Category:    "orchestration",
		Description: "Record deterministic Pinky worker claims, reports, and queries.",
		Usage:       "specd pinky <claim|status|update|report|block|release> ...",
		Synopsis:    "specd pinky <claim|status|update|report|block|release> ...",
		Flags: []FlagMeta{
			{Name: "mission", Type: "string", Description: "mission JSON path or - for claim"},
			{Name: "session", Type: "string", Description: "session id"},
			{Name: "worker", Type: "string", Description: "worker id"},
			{Name: "spec", Type: "string", Description: "spec slug"},
			{Name: "task", Type: "string", Description: "task id"},
			{Name: "attempt", Type: "string", Description: "positive attempt number"},
			{Name: "artifact", Type: "string", Description: "the artifact template name for brief (e.g. requirements.md)"},
			{Name: "percent", Type: "string", Description: "progress percent"},
			{Name: "message", Type: "string", Description: "progress message"},
			{Name: "reason", Type: "string", Description: "blocker reason"},
			{Name: "text", Type: "string", Description: "bounded Pinky query text"},
			{Name: "verification-ref", Type: "string", Description: "terminal verification reference"},
			{Name: "summary", Type: "string", Description: "terminal summary"},
			{Name: "changed-files", Type: "string", Description: "comma-separated changed files"},
			{Name: "git-head", Type: "string", Description: "git commit observed by the host worker"},
			{Name: "duration-ms", Type: "string", Description: "host-reported task duration in milliseconds"},
			{Name: "host-tokens", Type: "string", Description: "host-reported token count (stored, not computed)"},
			{Name: "host-cost", Type: "string", Description: "host-reported cost (stored, not computed)"},
			{Name: "json", Type: "boolean", Description: "emit JSON"},
		},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {1, "Gate or validation failure"}, {2, "Usage error"}, {3, "Workspace or session not found"}},
		Examples:  []string{"specd pinky claim --mission mission.json --json", "specd pinky update --session s --worker w --spec my-spec --task T1 --attempt 1 --message 'working' --percent 50", "specd pinky status --session s --worker w --json"},
	},

	{
		Command: "program", Category: "inspection", Hidden: true, DeprecatedIn: "palette-merge",
		Description:     "Cross-spec DAG runnable frontier",
		Usage:           "specd program [status] [--json]  |  specd program <link|unlink> <spec> --on <dep>",
		Synopsis:        "specd program [status|link|unlink] [args] [flags]",
		LongDescription: "Manages cross-spec dependencies and displays the cross-spec runnable frontier.",
		Flags:           []FlagMeta{{Name: "on", Type: "string"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}},
		Examples:        []string{"specd program status", "specd program link feat-b --on feat-a"},
	},

	{
		Command: "update", Category: "meta", Hidden: true, DeprecatedIn: "palette-merge",
		Description: "Update specd to latest version",
		Usage:       "specd update [--force]", Synopsis: "specd update [--force]",
		LongDescription: "Downloads the latest pre-built binary from GitHub Releases and replaces the running binary.",
		Flags:           []FlagMeta{{Name: "force", Type: "boolean", Description: "Force update even if already up to date"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Update failed"}, {2, "Usage error"}},
		Examples:        []string{"specd update", "specd update --force"},
	},

	{
		Command: "uninstall", Category: "meta", Hidden: true, DeprecatedIn: "palette-merge",
		Description:     "Remove specd from this machine",
		Usage:           "specd uninstall [--force] [--dry-run] [--json]",
		Synopsis:        "specd uninstall [--force] [--dry-run] [--json]",
		LongDescription: "Removes the specd install directory, the ~/.local/bin/specd link, and the '# specd' PATH lines install.sh added to your shell rc files (backed up to <rc>.specd.bak). A bare invocation only previews the plan; pass --force to actually remove. Project-local '.specd/' directories are always preserved.",
		Flags: []FlagMeta{
			{Name: "force", Type: "boolean", Description: "Perform the removal (without it, only the plan is shown)"},
			{Name: "dry-run", Type: "boolean", Description: "Show what would be removed and exit"},
			{Name: "json", Type: "boolean", Description: "Machine-readable output"},
		},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {1, "Uninstall failed"}, {2, "Usage error"}},
		Examples:  []string{"specd uninstall", "specd uninstall --dry-run", "specd uninstall --force"},
	},

	{
		Command: "version", Category: "meta", Hidden: true,
		Description: "Show version information",
		Usage:       "specd version [--json]", Synopsis: "specd version [--json]",
		LongDescription: "Prints the version of the installed specd binary. With --json, emits a machine-readable object for CI and release automation.",
		Flags:           []FlagMeta{{Name: "json", Type: "boolean", Description: "Emit machine-readable version JSON"}}, ExitCodes: []ExitCodeMeta{{0, "Success"}},
		Examples: []string{"specd version", "specd version --json"},
	},

	{
		Command: "mcp", Category: "meta", Hidden: true,
		Description:     "Run the MCP stdio server (or print a host config snippet)",
		Usage:           "specd mcp [--root <path>] [--spec <slug>] [--config <host>]",
		Synopsis:        "specd mcp [--root <path>] [--spec <slug>] [--config <host>]",
		LongDescription: "Starts a Model Context Protocol (MCP) JSON-RPC 2.0 server over stdio, exposing every read-safe and state-mutating specd command as an MCP tool. A thin transport over the existing handlers — stdlib-only, no network, no LLM calls.\n\n--spec <slug> pins active-spec resolution to one spec for the lifetime of the MCP process. --config <host> prints a ready-to-paste config snippet for the named MCP host and exits without starting the server. Supported hosts: antigravity, claude-code, claude-desktop, codex, cursor, vscode. Combine with --root to substitute your project path into the snippet.",
		Flags: []FlagMeta{
			{Name: "root", Type: "string", Description: "Resolve specs against this project root (also substituted into --config output)"},
			{Name: "spec", Type: "string", Description: "Pin MCP phase/status resolution to one spec slug"},
			{Name: "config", Type: "string", Description: "Print ready-to-paste MCP config for a host (antigravity | claude-code | claude-desktop | codex | cursor | vscode) and exit"},
		},
		ExitCodes: []ExitCodeMeta{{0, "Success (stream closed or config printed)"}, {1, "Server error"}, {2, "Usage error"}},
		Examples:  []string{"specd mcp", "specd mcp --root /path/to/project", "specd mcp --spec auth", "specd mcp --config cursor", "specd mcp --config codex --root /path/to/project"},
	},

	{
		Command: "help", Category: "meta", Hidden: true,
		Description: "Show detailed help for a command",
		Usage:       "specd help [command]", Synopsis: "specd help [command]",
		LongDescription: "Prints summary documentation for all commands, or detailed reference for a single command.",
		Flags:           []FlagMeta{{Name: "all", Type: "boolean", Description: "Include meta-hidden commands"}, {Name: "json", Type: "boolean", Description: "Output the command registry as JSON"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error (unknown command)"}},
		Examples:        []string{"specd help", "specd help init", "specd help --json"},
	},
}

func init() {
	positionals := map[string][]PositionalMeta{
		"new":      {{Name: "slug", Required: true, Description: "Spec slug"}},
		"approve":  {{Name: "slug", Required: true, Description: "Spec slug"}},
		"decision": {{Name: "slug", Required: true, Description: "Spec slug"}, {Name: "text", Required: true, Description: "Decision text"}},
		"midreq":   {{Name: "slug", Required: true, Description: "Spec slug"}, {Name: "input", Required: true, Description: "Feedback text"}},
		"memory":   {{Name: "slug", Required: true, Description: "Spec slug"}, {Name: "action", Required: true, Description: "add or promote"}},
		"next":     {{Name: "slug", Required: true, Description: "Spec slug"}},
		"dispatch": {{Name: "slug", Required: true, Description: "Spec slug"}},
		"verify":   {{Name: "slug", Required: true, Description: "Spec slug"}, {Name: "id", Required: false, Description: "Task id when running task verify"}},
		"task":     {{Name: "slug", Required: true, Description: "Spec slug"}, {Name: "id", Required: true, Description: "Task id"}},
		"status":   {{Name: "slug", Required: false, Description: "Optional spec slug"}},
		"mode":     {{Name: "slug", Required: true, Description: "Spec slug"}},
		"check":    {{Name: "slug", Required: true, Description: "Spec slug"}},
		"context":  {{Name: "slug", Required: true, Description: "Spec slug"}},
		"validate": {{Name: "slug", Required: true, Description: "Spec slug"}},
		"report":   {{Name: "slug", Required: true, Description: "Spec slug"}},
		"serve":    {{Name: "slug", Required: true, Description: "Spec slug"}},
		"replay":   {{Name: "slug", Required: true, Description: "Spec slug"}},
		"waves":    {{Name: "slug", Required: true, Description: "Spec slug"}},
		"brain":    {{Name: "action", Required: true, Description: "start, run, status, step, why, directive, pause, resume, or cancel"}, {Name: "slug", Required: false, Description: "Spec slug for spec-scoped actions"}},
		"pinky":    {{Name: "action", Required: true, Description: "claim, brief, heartbeat, progress, query, report, block, release, or inbox"}},
		"program":  {{Name: "action", Required: false, Description: "status, link, or unlink"}, {Name: "spec", Required: false, Description: "Spec slug for link/unlink"}},
		"mcp":      {{Name: "action", Required: false, Description: "Run server by default; config mode via --config"}},
		"help":     {{Name: "command", Required: false, Description: "Command name"}},
	}
	phaseCompat := map[string]*PhaseCompatibilityMeta{
		"approve":  planningAndVerificationCompat(),
		"decision": planningCompat(),
		"midreq":   anySpecCompat(),
		"memory":   executionReflectCompat(),
		"next":     executionCompat(),
		"dispatch": executionCompat(),
		"verify":   executionVerificationCompat(),
		"task":     executionCompat(),
		"mode":     anySpecCompat(),
		"check":    anySpecCompat(),
		"context":  anySpecCompat(),
		"validate": anySpecCompat(),
		"report":   completeReflectCompat(),
		"serve":    anySpecCompat(),
		"replay":   anySpecCompat(),
		"diff":     anySpecCompat(),
		"waves":    anySpecCompat(),
		"brain":    executionCompat(),
		"pinky":    executionCompat(),
		"program":  executionCompat(),
	}
	modeCompat := map[string]*ModeCompatibilityMeta{
		"new":     {Modes: []string{"base", "orchestrated"}},
		"mode":    {Modes: []string{"base", "orchestrated"}},
		"brain":   {Modes: []string{"orchestrated"}, RequiresOrchestrationCapability: true},
		"pinky":   {Modes: []string{"orchestrated"}, RequiresOrchestrationCapability: true},
		"program": {Modes: []string{"orchestrated"}, RequiresOrchestrationCapability: true},
	}
	for i := range Commands {
		cmd := &Commands[i]
		cmd.Positionals = positionals[cmd.Command]
		if compat, ok := phaseCompat[cmd.Command]; ok {
			cmd.PhaseCompatibility = compat
		}
		if compat, ok := modeCompat[cmd.Command]; ok {
			cmd.ModeCompatibility = compat
		} else if cmd.Category != "orchestration" {
			cmd.ModeCompatibility = &ModeCompatibilityMeta{Modes: []string{"any"}}
		}
		annotateFlagEnums(cmd)
	}
}

func planningCompat() *PhaseCompatibilityMeta {
	return &PhaseCompatibilityMeta{Statuses: []string{string(StatusRequirements), string(StatusDesign), string(StatusTasks)}, Phases: []string{string(PhaseAnalyze), string(PhasePlan)}}
}

func planningAndVerificationCompat() *PhaseCompatibilityMeta {
	return &PhaseCompatibilityMeta{Statuses: []string{string(StatusRequirements), string(StatusDesign), string(StatusTasks), string(StatusVerifying)}, Phases: []string{string(PhaseAnalyze), string(PhasePlan), string(PhaseVerify)}}
}

func executionCompat() *PhaseCompatibilityMeta {
	return &PhaseCompatibilityMeta{Statuses: []string{string(StatusExecuting), string(StatusBlocked)}, Phases: []string{string(PhaseExecute)}}
}

func executionVerificationCompat() *PhaseCompatibilityMeta {
	return &PhaseCompatibilityMeta{Statuses: []string{string(StatusExecuting), string(StatusVerifying)}, Phases: []string{string(PhaseExecute), string(PhaseVerify)}}
}

func executionReflectCompat() *PhaseCompatibilityMeta {
	return &PhaseCompatibilityMeta{Statuses: []string{string(StatusExecuting), string(StatusVerifying), string(StatusComplete)}, Phases: []string{string(PhaseExecute), string(PhaseVerify), string(PhaseReflect)}}
}

func completeReflectCompat() *PhaseCompatibilityMeta {
	return &PhaseCompatibilityMeta{Statuses: []string{string(StatusVerifying), string(StatusComplete)}, Phases: []string{string(PhaseVerify), string(PhaseReflect)}}
}

func anySpecCompat() *PhaseCompatibilityMeta {
	return &PhaseCompatibilityMeta{Statuses: []string{string(StatusRequirements), string(StatusDesign), string(StatusTasks), string(StatusExecuting), string(StatusVerifying), string(StatusComplete), string(StatusBlocked)}, Phases: []string{string(PhaseAnalyze), string(PhasePlan), string(PhaseExecute), string(PhaseVerify), string(PhaseReflect)}}
}

func annotateFlagEnums(cmd *CommandMeta) {
	for i := range cmd.Flags {
		flag := &cmd.Flags[i]
		switch flag.Name {
		case "agent":
			flag.Enum = []string{"auto", "all", "none", "codex", "claude-code", "cursor", "antigravity", "vscode"}
		case "scope":
			flag.Enum = []string{"project", "global"}
			flag.Default = "project"
		case "orchestration", "approval-policy":
			flag.Enum = []string{"manual", "planning", "session"}
		case "orchestration-mode":
			flag.Enum = []string{"inline", "delegate"}
		case "orchestration-sandbox", "sandbox":
			flag.Enum = []string{"none", "bwrap", "container"}
		case "impact":
			flag.Enum = []string{"low", "medium", "high", "critical"}
			flag.Required = true
		case "criticality":
			flag.Enum = []string{"note", "important", "critical"}
		case "status":
			if cmd.Command == "verify" {
				flag.Enum = []string{"pass", "fail"}
			} else if cmd.Command == "task" {
				flag.Enum = []string{string(TaskPending), string(TaskRunning), string(TaskComplete), string(TaskBlocked)}
				flag.Required = true
			}
		case "set":
			flag.Enum = []string{ModeBase, ModeOrchestrated}
		case "format":
			flag.Enum = []string{"md", "html", "prometheus"}
		case "action":
			flag.Enum = []string{"continue", "retry", "cancel", "reassign", "escalate"}
		case "config":
			flag.Enum = []string{"antigravity", "claude-code", "claude-desktop", "codex", "cursor", "vscode"}
		case "from", "schema":
			flag.Required = true
		}
	}
}
