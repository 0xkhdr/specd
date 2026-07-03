package core

// PositionalMeta describes one positional argument of a CLI command for help
// text and schema generation.
type PositionalMeta struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Repeatable  bool   `json:"repeatable,omitempty"`
	Description string `json:"description,omitempty"`
}

// FlagMeta describes one flag of a CLI command: its name, type, allowed
// values, default, and whether it is required, for help text and schema
// generation.
type FlagMeta struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Default     string   `json:"default,omitempty"`
}

// PhaseCompatibilityMeta lists the spec statuses and phases under which a
// command is valid to run.
type PhaseCompatibilityMeta struct {
	Statuses []string `json:"statuses,omitempty"`
	Phases   []string `json:"phases,omitempty"`
}

// ModeCompatibilityMeta lists the execution modes a command supports and
// whether it requires project orchestration capability.
type ModeCompatibilityMeta struct {
	Modes                           []string `json:"modes,omitempty"`
	RequiresOrchestrationCapability bool     `json:"requiresOrchestrationCapability,omitempty"`
}

// ExitCodeMeta documents one possible process exit code for a command and
// what it means.
type ExitCodeMeta struct {
	Code    int    `json:"code"`
	Meaning string `json:"meaning"`
}

// CommandMeta is the full metadata record for one CLI command: its usage,
// description, flags, positionals, compatibility constraints, exit codes, and
// examples. Commands is the registry of every CommandMeta.
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
	RemovedIn          string                  `json:"removedIn,omitempty"`
}

// Commands is the complete registry of specd CLI commands, used to drive help
// text, JSON schema generation, and `specd help`. Positionals, phase/mode
// compatibility, and flag enums are filled in by the init function below.
var Commands = []CommandMeta{
	{
		Command: "init", Category: "lifecycle",
		Description: "Scaffold project assets and configure coding agents",
		Usage:       "specd init [--agent <auto|all|none|codex|claude-code|cursor|antigravity|vscode>] [--scope project|global] [--yes] [--non-interactive] [--verbose] [--dry-run] [--guardrails] [--repair|--refresh|--force] [--orchestration [<policy>]] [--orchestration-workers <n>] [--orchestration-retries <n>] [--orchestration-timeout <minutes>] [--orchestration-cost-limit <usd>] [--orchestration-mode <inline|delegate>] [--orchestration-sandbox <none|bwrap|container>]", Synopsis: "specd init [--agent <name>] [--yes] [--dry-run]",
		LongDescription: "Scaffolds .specd/ and AGENTS.md, passively detects supported coding-agent hosts, optionally installs project-scoped MCP registration, verifies the in-process MCP server, and returns one next action. Non-interactive auto-detection never mutates host configuration unless --yes is supplied. Global scope requires explicit consent.",
		Flags:           []FlagMeta{{Name: "agent", Type: "string", Description: "Coding-agent selection: auto, all, none, codex, claude-code, cursor, antigravity, or vscode"}, {Name: "scope", Type: "string", Description: "Integration scope (default project)"}, {Name: "yes", Type: "boolean", Description: "Accept non-destructive project-scoped integration changes"}, {Name: "non-interactive", Type: "boolean", Description: "Disable prompts"}, {Name: "verbose", Type: "boolean", Description: "Include detailed path results"}, {Name: "json", Type: "boolean", Description: "Output one versioned InitResult document"}, {Name: "dry-run", Type: "boolean", Description: "Preview exact actions without writing"}, {Name: "repair", Type: "boolean", Description: "Restore missing managed assets only"}, {Name: "refresh", Type: "boolean", Description: "Refresh frozen managed assets and AGENTS.md markers"}, {Name: "force", Type: "boolean", Description: "Destructively overwrite all scaffold files and AGENTS.md"}, {Name: "list-packs", Type: "boolean", Description: "List the embedded spec packs and exit"}, {Name: "pack", Type: "string", Description: "Apply a spec pack by built-in name, registry name, or http(s) URL"}, {Name: "sha256", Type: "string", Description: "Pinned SHA256 digest required for a remote --pack URL"}, {Name: "registry", Type: "string", Description: "Git URL of a pack registry index; resolves a named --pack and pins it in .specd/pack.lock"}, {Name: "orchestration", Type: "string", Description: "Enable Brain/Pinky and set approval policy (manual, planning, session)"}, {Name: "orchestration-workers", Type: "string", Description: "Max concurrent Pinky workers (1..64, default 4)"}, {Name: "orchestration-retries", Type: "string", Description: "Retry budget for failed/reclaimed work (0..10, default 2)"}, {Name: "orchestration-timeout", Type: "string", Description: "Session wall-clock timeout in minutes (1..1440, default 120)"}, {Name: "orchestration-cost-limit", Type: "string", Description: "Host-reported cost brake in USD (default 0)"}, {Name: "orchestration-mode", Type: "string", Description: "Subagent coordination mode: inline or delegate (default delegate)"}, {Name: "orchestration-sandbox", Type: "string", Description: "Default verify sandbox: none, bwrap, or container (default none)"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Initialization or pack operation failed"}, {2, "Usage error"}},
		Examples:        []string{"specd init --agent auto --yes", "specd init --agent none --non-interactive", "specd init --agent all --dry-run --json", "specd init --repair"},
	},

	{
		Command: "handshake", Category: "inspection", Hidden: true,
		Description: "Emit startup bootstrap and binding policy oracles",
		Usage:       "specd handshake bootstrap [--include-schema] [--json] | specd handshake policy [<slug>] [--expect-config-digest <sha256>] [--json]", Synopsis: "specd handshake <bootstrap|policy> [slug] [--json]",
		LongDescription: "Read-only agent integration surface. bootstrap returns the first-turn load list, command schema digest, config digest, health, and active modes. policy summarizes binding config, optional spec execution mode, allowed loop family, diagnostics, and config digest drift.",
		Flags:           []FlagMeta{{Name: "include-schema", Type: "boolean", Description: "Inline full command schema in bootstrap output"}, {Name: "expect-config-digest", Type: "string", Description: "Fail if current config digest differs from this sha256"}, {Name: "json", Type: "boolean", Description: "Output JSON"}},
		Positionals:     []PositionalMeta{{Name: "subcommand", Required: true, Description: "bootstrap or policy"}, {Name: "slug", Required: false, Description: "Spec slug for policy mode"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Policy violation or digest mismatch"}, {2, "Usage error"}, {3, ".specd/ or spec not found"}},
		Examples:        []string{"specd handshake bootstrap --json", "specd handshake policy my-feature --json", "specd handshake policy --expect-config-digest <sha256> --json"},
	},

	{
		Command: "new", Category: "lifecycle",
		Description: "Create a spec with six artifacts",
		Usage:       "specd new <slug> [--title \"...\"] [--orchestrated] [--prototype]", Synopsis: "specd new <slug> [--title \"...\"] [--orchestrated] [--prototype]",
		LongDescription: "Creates a new spec directory under .specd/specs/<slug>/ with six artifact stubs. Specs default to Base execution mode; --orchestrated records executionMode=orchestrated (origin user) and requires project orchestration capability. --prototype creates a prototype spec that skips the design/tasks planning gates but can never reach complete — run `specd promote` to convert it to a full spec.",
		Flags:           []FlagMeta{{Name: "title", Type: "string", Description: "The title of the spec"}, {Name: "orchestrated", Type: "boolean", Description: "Create the spec in orchestrated (Brain/Pinky) mode; requires project orchestration capability"}, {Name: "prototype", Type: "boolean", Description: "Create a prototype spec (planning gates relaxed; cannot complete until promoted)"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Orchestration requested without project capability"}, {2, "Usage error"}, {3, ".specd/ not found or spec already exists"}},
		Examples:        []string{"specd new my-feature", "specd new my-feature --title \"My Feature\"", "specd new payments --title \"Billing\" --orchestrated", "specd new spike-idea --prototype"},
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
		Usage:       "specd status [<slug>] [--all] [--program] [--set-mode simple|orchestrated] [--recommend] [--json]", Synopsis: "specd status [<slug>] [--all] [--program] [--json]",
		LongDescription: "Renders the durable status board of a specific spec, lists all specs, or displays the cross-spec program frontier. With a slug, --set-mode records a new per-spec execution mode (orchestrated requires project capability; switching to simple is refused while a Brain session is active) and --recommend emits a deterministic, advisory mode recommendation — the survivor home for the merged `mode` command's set/recommend paths.",
		Flags:           []FlagMeta{{Name: "all", Type: "boolean", Description: "List all specs (default when no slug is supplied)"}, {Name: "program", Type: "boolean", Description: "Show the cross-spec program frontier"}, {Name: "set-mode", Type: "string", Description: "Set the spec's execution mode: simple or orchestrated"}, {Name: "recommend", Type: "boolean", Description: "Emit an advisory mode recommendation from countable spec facts"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Capability missing or session-active refusal"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd status", "specd status my-feature --json", "specd status my-feature --set-mode orchestrated", "specd status my-feature --recommend --json"},
	},

	{
		Command: "check", Category: "inspection",
		Description: "Run all validation gates",
		Usage:       "specd check <slug> [--schema-only] [--security] [--json] | specd check --schema", Synopsis: "specd check <slug> [--schema-only] [--security] [--json] | specd check --schema",
		LongDescription: "Runs all seven validation gates on the specified spec. --schema-only validates state.json against the embedded open spec schema; --schema emits that schema. --security runs the deterministic security suite (secrets, injection, slopsquatting) over the working-tree changed files and records a summary in state; advisory scanners never fail the command, only blocking (error-severity) findings do.",
		Flags:           []FlagMeta{{Name: "schema-only", Type: "boolean", Description: "Validate state.json against the embedded open spec schema only"}, {Name: "schema", Type: "boolean", Description: "Emit the embedded open spec format JSON Schema"}, {Name: "security", Type: "boolean", Description: "Run the deterministic security suite over changed files"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Validation failed"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd check my-feature", "specd check my-feature --json"},
	},

	{
		Command: "context", Category: "inspection",
		Description: "Phase-scoped briefing",
		Usage:       "specd context <slug> [--hud] [--json]", Synopsis: "specd context <slug> [--hud] [--json]",
		LongDescription: "Provides a minimal phase-scoped briefing for the current spec phase. --hud renders the deterministic context heads-up display instead: the steering/skill load files with their on-disk byte and approximate token cost, plus the active mode and routing tier (all measured from disk and recorded state — no interpretation).",
		Flags:           []FlagMeta{{Name: "hud", Type: "boolean", Description: "Render the context HUD (load files, byte/token cost, mode/tier) instead of the phase briefing"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd context my-feature", "specd context my-feature --hud", "specd context my-feature --json"},
	},

	{
		Command: "eval", Category: "inspection",
		Description: "Score a spec against its eval rubric",
		Usage:       "specd eval <slug> [init|trend] [--suite <name>] [--force] [--json]", Synopsis: "specd eval <slug> [init|trend] [--suite <name>] [--json]",
		LongDescription: "Runs a spec's eval rubric and records the score to state.json and a result file. `eval init` compiles approved requirements into a rubric skeleton (one stub per acceptance criterion); `eval trend` reports score deltas and failure clustering over the result history. Scoring is deterministic; command checks run through the shared sandboxed exec path.",
		Flags:           []FlagMeta{{Name: "suite", Type: "string", Description: "Rubric suite name (default reads eval-rubric.json)"}, {Name: "force", Type: "boolean", Description: "Overwrite an existing rubric on eval init"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Score below minScore"}, {2, "Usage error"}, {3, "Spec or rubric not found"}},
		Examples:        []string{"specd eval my-feature", "specd eval my-feature init", "specd eval my-feature trend --json"},
	},

	{
		Command: "promote", Category: "lifecycle", Hidden: true,
		Description: "Promote a prototype spec after a passing eval",
		Usage:       "specd promote <slug> --evidence \"...\" [--suite <name>] [--json]", Synopsis: "specd promote <slug> --evidence \"...\"",
		LongDescription: "Converts a prototype spec into a full spec once its eval rubric passes. The evidence string is mandatory — promotion never bypasses the evidence discipline. The normal approve ratchet applies to the promoted spec.",
		Flags:           []FlagMeta{{Name: "evidence", Type: "string", Description: "Required promotion evidence", Required: true}, {Name: "suite", Type: "string", Description: "Rubric suite name (default reads eval-rubric.json)"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Not a prototype, eval failed, or missing evidence"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd promote my-feature --evidence \"eval green, owner sign-off\""},
	},

	{
		Command: "conductor", Category: "execution",
		Description: "Drive the interactive micro-task conductor session",
		Usage:       "specd conductor <slug> <start|step|accept|reject|stop|replay|switch|status> [micro] [--reason \"...\"] [--json]", Synopsis: "specd conductor <slug> <start|step|accept|reject|stop|status>",
		LongDescription: "Runs the hands-on conductor mode over a task's micro-tasks with an append-only ledger (conductor.jsonl). start opens a session under a spec lock; step briefs the next micro-task; accept/reject record the outcome (reject requires --reason — it is the training signal); stop closes the session. Acceptance never substitutes for verify evidence.",
		Flags:           []FlagMeta{{Name: "reason", Type: "string", Description: "Mandatory rejection reason (reject) or transition note (switch/stop)"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Gate failure (no session, missing reason, lock contention)"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd conductor my-feature start", "specd conductor my-feature reject --reason \"wrong file touched\"", "specd conductor my-feature status --json"},
	},

	{
		Command: "review", Category: "inspection",
		Description: "Scaffold a review report or extract a review checklist",
		Usage:       "specd review <slug> [checklist] [--force] [--json]", Synopsis: "specd review <slug> [checklist]",
		LongDescription: "Scaffolds review_report.md with the mandatory sections (Summary, Bugs, Security, Hallucinated Dependencies, Style, Verdict) and prints the read-only adversarial reviewer brief. `review checklist` deterministically extracts a human checklist from design.md sections and tasks.md contracts (extraction only). When config.review.required is on, `approve` blocks verifying→complete until a fresh, valid report with an `approve` verdict exists — human approval stays final.",
		Flags:           []FlagMeta{{Name: "force", Type: "boolean", Description: "Overwrite an existing review_report.md scaffold"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Gate failure"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd review my-feature", "specd review my-feature checklist --json"},
	},

	{
		Command: "orchestrate", Category: "execution",
		Description: "Inspect and resolve auto-escalations",
		Usage:       "specd orchestrate <slug> <status|resume> [--override] [--json]", Synopsis: "specd orchestrate <slug> <status|resume --override>",
		LongDescription: "Surfaces and resolves deterministic auto-escalations (V7). `status` prints the active escalation record (task, rule, facts) and the advisory conductor-handoff recommendation; `resume --override` is the human override that clears the escalation so orchestration may proceed. The binary never auto-clears an escalation and never auto-switches mode — resolution is always an explicit human action.",
		Flags:           []FlagMeta{{Name: "override", Type: "boolean", Description: "Clear the active escalation (required by resume)"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Gate failure (no escalation, missing --override)"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd orchestrate my-feature status", "specd orchestrate my-feature resume --override"},
	},

	{
		Command: "submit", Category: "execution",
		Description: "Validate all gates and run the configured PR submit command",
		Usage:       "specd submit <slug> [--waves w1,w2] [--dry-run] [--json]", Synopsis: "specd submit <slug> [--dry-run]",
		LongDescription: "Batch PR submission (V7). Validates that every configured gate is green for the spec, generates the deterministic, network-free PR summary, and streams it on stdin to the operator-configured config.submit.command (e.g. `gh pr create --body-file -`) run through the shared sandboxed exec path with a scrubbed env. No git/GitHub logic is embedded. A gate violation or a non-zero command exit is a failure with no partial state; --dry-run prints the summary without executing.",
		Flags:           []FlagMeta{{Name: "waves", Type: "string", Description: "Restrict the summary to a comma-separated wave bundle"}, {Name: "dry-run", Type: "boolean", Description: "Print the PR summary without running the submit command"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Gate violation or submit command failure"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd submit my-feature --dry-run", "specd submit my-feature"},
	},

	{
		Command: "deploy", Category: "execution",
		Description: "Run the evidence-gated deploy driver or its rollback",
		Usage:       "specd deploy <slug> --env <env> [--dry-run] [--json]  |  specd deploy rollback <slug> --env <env> [--json]", Synopsis: "specd deploy <slug> --env <env> [--dry-run]",
		LongDescription: "Evidence-gated deploy driver runner (V9). Refuses unless the spec is complete, every gate named in the env's `.specd/deploy/<env>.json` requiresGates is recorded green, and — for a production env or an approval-required plan — a human deploy approval exists (`specd approve <slug> --deploy --env <env>`). Runs the plan's steps sequenced through the shared sandboxed exec path with a scrubbed env, appending every result to deploy.jsonl. `deploy rollback` replays the recorded inverse chain (successful steps, reverse order); a failing rollback step halts and exits 3. No CD logic is embedded — steps are operator-authored commands.",
		Flags:           []FlagMeta{{Name: "env", Type: "string", Description: "Target environment (matches .specd/deploy/<env>.json)"}, {Name: "dry-run", Type: "boolean", Description: "Show the plan and precondition status without executing"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Precondition/gate failure or step failure"}, {2, "Usage error"}, {3, "Spec/config not found or rollback halted"}},
		Examples:        []string{"specd deploy my-feature --env staging --dry-run", "specd approve my-feature --deploy --env production", "specd deploy my-feature --env production", "specd deploy rollback my-feature --env staging"},
	},

	{
		Command: "observe", Category: "inspection",
		Description: "Correlate a production error into a mid-requirement",
		Usage:       "specd observe correlate <payload.json> [--spec <slug>] [--json]  |  specd observe --listen [--spec <slug>]", Synopsis: "specd observe correlate <payload.json> [--spec <slug>]",
		LongDescription: "Inbound production-error correlation (V9). `correlate` reads a schema-validated, size-capped error payload (a CI-piped Sentry export), deterministically attributes it to a spec by matching stack-frame files against task `files:` contracts (falling back to the recent deploy ledger), and appends an evidenced entry to that spec's mid-requirements.md — gating high/critical impact for human approval, exactly like `specd midreq`. `--listen` starts an optional loopback-only, token-authed HTTP receiver that applies the same transform per payload. The transform is the feature; the listener is optional.",
		Flags:           []FlagMeta{{Name: "listen", Type: "boolean", Description: "Start the loopback token-authed HTTP receiver"}, {Name: "spec", Type: "string", Description: "Force attribution to this spec"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Invalid payload or no correlation"}, {2, "Usage error"}, {3, "Payload/root not found"}},
		Examples:        []string{"specd observe correlate error.json", "specd observe correlate error.json --spec my-feature", "specd observe --listen"},
	},

	{
		Command: "ingest", Category: "lifecycle",
		Description: "Inventory a legacy codebase into an ingestion spec",
		Usage:       "specd ingest new <slug> --path <dir> [--include-ignored] [--json]", Synopsis: "specd ingest new <slug> --path <dir>",
		LongDescription: "Legacy ingestion (V10). `ingest new` validates the path (no traversal outside the repo), writes a deterministic inventory.json (sorted file list, sizes, and manifest-derived module names via stdlib — countable facts only, the binary never reads legacy semantics), and scaffolds an ingestion-flavored spec. File scoping respects `.gitignore` via `git ls-files` when in a git repo (`--include-ignored` forces a bounded walk with default excludes). The `specd-ingest` skill teaches the agent to reverse-engineer requirements/design/tasks; the opt-in `ingest` gate then enforces that every inventoried file is referenced by ≥1 requirement or waived with a reason.",
		Flags:           []FlagMeta{{Name: "path", Type: "string", Description: "Directory to inventory (inside the repo)"}, {Name: "include-ignored", Type: "boolean", Description: "Walk all files instead of git-tracked only"}, {Name: "title", Type: "string", Description: "Spec title (default derived from slug)"}, {Name: "json", Type: "boolean"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {1, "Invalid path or existing spec"}, {2, "Usage error"}, {3, "Path/root not found"}},
		Examples:        []string{"specd ingest new legacy-billing --path ./billing", "specd ingest new legacy-billing --path ./billing --include-ignored"},
	},

	{
		Command: "report", Category: "inspection",
		Description: "Generate markdown, HTML, or metrics report",
		Usage:       "specd report <slug> [--format md|html|prometheus] [--out <path>] [--pr-summary] [--conductor] [--serve|--watch|--history|--diff]", Synopsis: "specd report <slug> [--format md|html|prometheus] [--out <path>] [--pr-summary] [--conductor]",
		LongDescription: "Compiles a comprehensive HTML or Markdown progress report, or an opt-in Prometheus textfile metrics view. With --pr-summary, emits a deterministic, network-free pull-request summary (Markdown, or JSON under SPECD_JSON): wave/task progress, gate status, and the commit↔task link map. With --conductor, clusters the conductor ledger's rejection reasons (exact string + count) — the deterministic rejection-analytics view.",
		Flags:           []FlagMeta{{Name: "format", Type: "string", Description: "Output format: md, html, or prometheus"}, {Name: "out", Type: "string"}, {Name: "pr-summary", Type: "boolean", Description: "Emit a deterministic PR summary instead of the full report"}, {Name: "conductor", Type: "boolean", Description: "Cluster conductor rejection reasons (exact string + count)"}, {Name: "serve", Type: "boolean", Description: "Serve the live dashboard"}, {Name: "watch", Type: "boolean", Description: "Stream runnable-frontier changes"}, {Name: "history", Type: "boolean", Description: "Replay the spec audit timeline"}, {Name: "diff", Type: "boolean", Description: "Diff spec artifacts between git refs"}},
		ExitCodes:       []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:        []string{"specd report my-feature --format html --out ./report.html", "specd report my-feature --format prometheus", "specd report my-feature --conductor"},
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
		Command:         "harness",
		Category:        "platform",
		Hidden:          true,
		Description:     "Share the configured harness (guardrails, deploy, roles, routing) as a versioned team asset.",
		Usage:           "specd harness <push|pull|list|enable> ... [--name <n>] [--force] [--json]",
		Synopsis:        "specd harness <push|pull|list|enable> ... [--force]",
		LongDescription: "Bundles the project's declarative policy — guardrails, deploy templates, roles, routing — under .specd/harness/ with a SHA256-pinned harness.json manifest, and shares it over stdlib-exec git (scrubbed env, transport allowlist, remote-URL validation).\n\npush builds the current bundle (version advances monotonically) and pushes it to a git URL. pull clones a remote bundle, verifies every pinned checksum, refuses a version downgrade or a locally-modified overwrite without --force, and quarantines every imported executable `command` artifact — copied to .specd/harness/quarantine/, listed, never installed — until an operator runs enable, which is recorded in the harness decision log. list shows the bundle and the quarantine; enable installs one quarantined artifact.",
		Flags: []FlagMeta{
			{Name: "name", Type: "string", Description: "Bundle name on push (defaults to the prior name or project dir)"},
			{Name: "force", Type: "boolean", Description: "Override a version downgrade or a locally-modified overwrite"},
			{Name: "json", Type: "boolean", Description: "Emit JSON"},
		},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {1, "Gate failure (refused overwrite, checksum mismatch, downgrade)"}, {2, "Usage error"}, {3, "No bundle or quarantined item not found"}},
		Examples:  []string{"specd harness push git@example.com:team/harness.git --name platform", "specd harness pull https://example.com/team/harness.git", "specd harness list --json", "specd harness enable .specd/deploy/prod.json"},
	},

	{
		Command:         "migrate",
		Category:        "lifecycle",
		Hidden:          true,
		Description:     "Migrate a v0.1.x project onto v0.2.0 state schema and report available config blocks.",
		Usage:           "specd migrate [--json]",
		Synopsis:        "specd migrate [--json]",
		LongDescription: "Idempotent one-shot upgrade. Rewrites every spec's state.json at the current schema version (the v5→v6 migration is otherwise silent on first load) and reports which additive v0.2.0 policy blocks — guardrails, routing, eval/review gates — are available to adopt. It never writes policy content, so a migrated repo keeps the new gates default-off (backward-compat invariant). Running it a second time is a no-op.",
		Flags: []FlagMeta{
			{Name: "json", Type: "boolean", Description: "Emit the migration report as JSON"},
		},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {1, "Migration failed (concurrent write or corrupt state)"}, {2, "Usage error"}, {3, ".specd/ not found"}},
		Examples:  []string{"specd migrate", "specd migrate --json"},
	},

	{
		Command:         "dashboard",
		Category:        "inspection",
		Hidden:          true,
		Description:     "Serve the unified, read-only project dashboard (waves, cost, escalations, evals, harness).",
		Usage:           "specd dashboard [<slug>] [--addr 127.0.0.1:8765] [--mode <all|conductor|orchestrator|cost|eval>]",
		Synopsis:        "specd dashboard [<slug>] [--addr 127.0.0.1:8765] [--mode all]",
		LongDescription: "Starts the read-only, browser-native unified dashboard bound to loopback. Renders project-wide state from local state and ledgers only — conductor sessions, orchestrator waves, eval trends, cost attribution, escalations, and the shared harness bundle — with zero outbound network. Reuses the existing SSE stream for live updates. --mode filters the rendered panels; the default lists every panel. A read-only alias over `specd report --serve` with a project-wide home page.",
		Flags: []FlagMeta{
			{Name: "addr", Type: "string", Description: "Loopback bind address (default 127.0.0.1:8765)"},
			{Name: "mode", Type: "string", Description: "Panel filter: all, conductor, orchestrator, cost, or eval (default all)"},
		},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {1, "Server error"}, {2, "Usage error"}},
		Examples:  []string{"specd dashboard", "specd dashboard --mode cost", "specd dashboard my-feature --addr 127.0.0.1:9000"},
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
		"verify":   {{Name: "slug", Required: true, Description: "Spec slug"}, {Name: "id", Required: false, Description: "Task id when running task verify"}},
		"task":     {{Name: "slug", Required: true, Description: "Spec slug"}, {Name: "id", Required: true, Description: "Task id"}},
		"status":   {{Name: "slug", Required: false, Description: "Optional spec slug"}},
		"check":    {{Name: "slug", Required: true, Description: "Spec slug"}},
		"context":  {{Name: "slug", Required: true, Description: "Spec slug"}},
		"report":   {{Name: "slug", Required: true, Description: "Spec slug"}},
		"waves":    {{Name: "slug", Required: true, Description: "Spec slug"}},
		"brain":    {{Name: "action", Required: true, Description: "start, run, status, step, why, directive, pause, resume, or cancel"}, {Name: "slug", Required: false, Description: "Spec slug for spec-scoped actions"}},
		"pinky":    {{Name: "action", Required: true, Description: "claim, brief, heartbeat, progress, query, report, block, release, or inbox"}},
		"mcp":      {{Name: "action", Required: false, Description: "Run server by default; config mode via --config"}},
		"help":     {{Name: "command", Required: false, Description: "Command name"}},
	}
	phaseCompat := map[string]*PhaseCompatibilityMeta{
		"approve":  planningAndVerificationCompat(),
		"decision": planningCompat(),
		"midreq":   anySpecCompat(),
		"memory":   executionReflectCompat(),
		"next":     executionCompat(),
		"verify":   executionVerificationCompat(),
		"task":     executionCompat(),
		"check":    anySpecCompat(),
		"context":  anySpecCompat(),
		"report":   completeReflectCompat(),
		"waves":    anySpecCompat(),
		"brain":    executionCompat(),
		"pinky":    executionCompat(),
	}
	modeCompat := map[string]*ModeCompatibilityMeta{
		"new":   {Modes: []string{"simple", "orchestrated"}},
		"brain": {Modes: []string{"orchestrated"}, RequiresOrchestrationCapability: true},
		"pinky": {Modes: []string{"orchestrated"}, RequiresOrchestrationCapability: true},
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
			flag.Enum = []string{ModeSimple, ModeOrchestrated, ModeConductor}
		case "format":
			flag.Enum = []string{"md", "html", "prometheus"}
		case "action":
			flag.Enum = []string{"continue", "retry", "cancel", "reassign", "escalate"}
		case "config":
			flag.Enum = []string{"antigravity", "claude-code", "claude-desktop", "codex", "cursor", "vscode"}
		case "from", "schema":
			flag.Required = true
		case "mode":
			if cmd.Command == "dashboard" {
				flag.Enum = []string{"all", "conductor", "orchestrator", "cost", "eval"}
				flag.Default = "all"
			}
		}
	}
}
