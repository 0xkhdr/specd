package core

// HelpSchemaVersion versions the machine-readable help palette (`help --json`).
// Consumers (MCP, role prompts, external tools) pin against it and can detect a
// shape change; bump it whenever the Command/Flag JSON contract changes (spec
// 03 R4, pairs with the state-schema discipline of spec 02).
const HelpSchemaVersion = 1

// OperationSchemaVersion versions the canonical per-operation metadata. An
// operation is narrower than a command: mixed commands such as eval and task
// expose distinct read and write contracts instead of inheriting one verb-wide
// guess.
const OperationSchemaVersion = 1

type OperationActor string

const (
	ActorAgent    OperationActor = "agent"
	ActorHuman    OperationActor = "human"
	ActorOperator OperationActor = "operator"
)

type OperationEffect string

const (
	EffectRead           OperationEffect = "read"
	EffectWorkspaceWrite OperationEffect = "workspace-write"
	EffectStateWrite     OperationEffect = "state-write"
	EffectExternal       OperationEffect = "external"
)

// Operation is the single public-operation contract projected into dispatch,
// help, MCP, handshakes, manifests, and driver guidance (spec 11 R3/V13).
type Operation struct {
	ID                string          `json:"id"`
	Command           string          `json:"command"`
	Subcommand        string          `json:"subcommand,omitempty"`
	Usage             string          `json:"usage"`
	Actor             OperationActor  `json:"actor"`
	Effect            OperationEffect `json:"effect"`
	AllowedPhases     []Phase         `json:"phases"`
	AuthorityRequired bool            `json:"authority_required"`
	TaskRequired      bool            `json:"task_required"`
	ScopeSource       string          `json:"scope_source"`
	NetworkClass      string          `json:"network_class"`
	ExitCodes         []ExitCode      `json:"exit_codes"`
	Examples          []string        `json:"examples"`
}

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
	// Values is the human-readable value shape or provenance of a value flag
	// (spec R4.3): an enum rendering like "pass|fail", or a note on where the
	// value comes from (e.g. which command mints the id it expects). It is
	// surfaced verbatim by `help --json` and per-command text help; unlike
	// Enum it is documentation only — dispatch never validates against it.
	Values string `json:"values,omitempty"`
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
	// HumanOnly marks a verb that records human intent (approval, mid-course
	// decisions). Driving guidance lists these separately from machine-legal
	// commands so an agent never self-approves or fabricates a human decision
	// (spec 01 R6.1, R6.2 "not agent self-approval").
	HumanOnly bool `json:"human_only,omitempty"`
	// RequiresTask marks a verb that operates on an executable task (task
	// verify/context). Driving guidance suppresses these when a spec has no
	// executable task, so an agent is never told to verify or fetch context for
	// a task that does not exist (spec 01 R6.2).
	RequiresTask bool `json:"requires_task,omitempty"`
}

// anyPhase is the explicit unrestricted declaration.
func anyPhase() []Phase { return []Phase{PhaseAny} }

func securityExceptionFlags() []Flag {
	names := []string{"action", "reason", "ticket", "owner", "scope", "revision", "environment", "issued-at", "expires-at", "control", "approver"}
	flags := make([]Flag, 0, len(names))
	for _, name := range names {
		flags = append(flags, Flag{Name: name, TakesValue: true, Type: "string", Description: "Governed exception " + name + "."})
	}
	return flags
}

// postRequirementsPhases is the set for execution verbs (verify, next): every
// phase except perceive. A spec still in the requirements (perceive) phase has
// no approved design or task DAG to act on, so these verbs fail closed there
// (spec 03 R2 acceptance: "execution verb on a spec still in requirements phase
// exits 2"). The finer approval check (requireTaskGate) still applies inside
// the handler; this is the coarse metadata-driven guard.
func postRequirementsPhases() []Phase {
	return []Phase{PhaseAnalyze, PhasePlan, PhaseExecute, PhaseVerify, PhaseReflect}
}

// postExecutionPhases is the set for terminal verbs (submit): only once a spec
// is executing or past it. A spec still in analyze/plan has no completed work to
// submit, so `submit` fails closed there (spec 08 R6).
func postExecutionPhases() []Phase {
	return []Phase{PhaseExecute, PhaseVerify, PhaseReflect}
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
		Usage:         "specd init [--agent=<name>] [--repair|--refresh] [--dry-run]",
		Description:   "Initialize or re-sync specd project state and managed assets.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd init", "specd init --agent=pinky", "specd init --repair --dry-run", "specd init --refresh"},
		Flags: []Flag{
			{Name: "agent", TakesValue: true, Type: "string", Description: "Select agent harness."},
			{Name: "repair", Type: "bool", Description: "Restore drifted managed regions from the current templates."},
			{Name: "refresh", Type: "bool", Description: "Update managed regions to the current binary's template version."},
			{Name: "dry-run", Type: "bool", Description: "Print the managed-region changes and write nothing."},
		},
	},
	{
		Name:          "agents",
		Usage:         "specd agents [doctor | guide <slug>] [--json]",
		Description:   "Inspect agent artifacts, diagnose prerequisites, or emit deterministic driver guidance without writing.",
		AllowedPhases: anyPhase(),
		Examples:      []string{"specd agents", "specd agents doctor --json", "specd agents guide payments --json"},
		Flags:         []Flag{{Name: "json", TakesValue: false, Type: "bool", Description: "Emit JSON."}},
		ExitCodes:     stdCodes(),
	},
	{
		Name:          "adapters",
		Usage:         "specd adapters [--json]",
		Description:   "Inspect configured interoperability adapters read-only, distinguishing configured, missing, incompatible, and disabled without loading secrets or running anything.",
		AllowedPhases: anyPhase(),
		Examples:      []string{"specd adapters", "specd adapters --json"},
		Flags:         []Flag{{Name: "json", Type: "bool", Description: "Emit machine-readable JSON."}},
		ExitCodes:     stdCodes(),
	},
	{
		Name:          "eval",
		Usage:         "specd eval <import|status> <spec> [artifact]",
		Description:   "Import validated local eval evidence or inspect stored eval evidence.",
		AllowedPhases: anyPhase(),
		Examples:      []string{"specd eval import payments adapter.jsonl --task T1", "specd eval status payments --json"},
		Flags: []Flag{
			{Name: "json", Type: "bool", Description: "Emit machine-readable JSON."},
			{Name: "task", TakesValue: true, Type: "string", Values: "task id from the spec's tasks.md `id` column", Description: "Expected task identity for import."},
			{Name: "check", TakesValue: true, Type: "string", Values: "check-id from the task's declared evidence cell (class/check-id; classes: test|output_eval|trajectory_eval|review)", Description: "Expected check identity for import."},
		},
		ExitCodes: stdCodes(),
	},
	{
		Name:          "new",
		Usage:         "specd new <name> [--agent=<name>]",
		Description:   "Create a new spec workspace.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd new payments", "specd new payments --agent=codex", "specd new payments --agent=pinky"},
		Flags: []Flag{
			{Name: "agent", TakesValue: true, Type: "string", Description: "Select agent harness."},
		},
	},
	{
		Name:          "incident",
		Usage:         "specd incident seed <new-spec> --source-spec <spec> --release <id> --deployment <id> --criterion <id> --evidence-ref <ref[,ref]>",
		Description:   "Seed a new spec from bounded delivery observation references without loading raw payloads.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd incident seed checkout-recovery --source-spec checkout --release rel-7 --deployment dep-4 --criterion availability --evidence-ref obs://health/42"},
		Flags: []Flag{
			{Name: "source-spec", TakesValue: true, Type: "string", Description: "Source spec owning the immutable delivery ledger."},
			{Name: "release", TakesValue: true, Type: "string", Description: "Source release identity."},
			{Name: "deployment", TakesValue: true, Type: "string", Description: "Source deployment identity."},
			{Name: "criterion", TakesValue: true, Type: "string", Description: "Failed or observed health criterion."},
			{Name: "evidence-ref", TakesValue: true, Type: "string", Description: "Comma-separated bounded external references; queries, fragments, and raw payloads are refused."},
		},
	},
	{
		Name:          "archive",
		Usage:         "specd archive <spec> --successor <spec> --owner <owner> --evidence <ref>",
		Description:   "Retire a spec from active context while preserving content hashes and successor provenance.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd archive payments-v1 --successor payments-v2 --owner platform --evidence release:rel-7"},
		Flags: []Flag{
			{Name: "successor", TakesValue: true, Type: "string", Description: "Active successor spec receiving a supersedes link."},
			{Name: "owner", TakesValue: true, Type: "string", Description: "Accountable archive owner."},
			{Name: "evidence", TakesValue: true, Type: "string", Description: "Audit evidence reference authorizing retirement."},
		},
	},
	{
		Name:          "approve",
		Usage:         "specd approve <spec>",
		Description:   "Advance a spec exactly one lifecycle step after human approval and passing readiness gates.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd approve payments"},
		HumanOnly:     true,
		SpecSlugArg:   argAt(0),
	},
	{
		Name:          "mode",
		Usage:         "specd mode <spec> orchestrated",
		Description:   "Record human approval for the separate opt-in orchestration mode transition.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd mode payments orchestrated"},
		HumanOnly:     true,
		SpecSlugArg:   argAt(0),
	},
	{
		Name:          "exception",
		Usage:         "specd exception <approve|revoke> <finding> [governed exception fields]",
		Description:   "Record or revoke a governed human security exception without changing lifecycle status.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd exception approve scanner-false-positive --reason 'reviewed false positive'"},
		HumanOnly:     true,
		Flags:         securityExceptionFlags(),
	},
	{
		Name:          "midreq",
		Usage:         "specd midreq <spec> --text <change> [--scope <scope>]",
		Description:   "Capture a scoped mid-stream requirement change.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd midreq payments --text 'add refund path' --scope requirements"},
		HumanOnly:     true,
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
		HumanOnly:     true,
		Flags: []Flag{
			{Name: "text", TakesValue: true, Type: "string", Description: "Decision rationale (required)."},
			{Name: "scope", TakesValue: true, Type: "string", Description: "Optional scope label."},
		},
	},
	{
		// The agent-legal counterpart to `decision`. An agent that hits a
		// deviation needs a route it is permitted to take; without one it either
		// invents authority or drops the deviation on the floor (R1.1). This
		// records the request only — it advances no phase, resolves nothing, and
		// writes no evidence. A human still answers with `specd decision`.
		Name:          "request-decision",
		Usage:         "specd request-decision <spec> --text <deviation> [--scope <scope>]",
		Description:   "Record an agent's request for a human decision. Records the request only; it advances no phase and writes no evidence.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd request-decision payments --text 'webhook retry needs a backoff not in the design' --scope design"},
		Flags: []Flag{
			{Name: "text", TakesValue: true, Type: "string", Description: "Deviation the agent needs decided (required)."},
			{Name: "scope", TakesValue: true, Type: "string", Description: "Optional scope label."},
		},
	},
	{
		Name:          "drift",
		Usage:         "specd drift <spec> [--json]",
		Description:   "Project declared invariants and active decisions against local verify evidence without writing.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd drift payments", "specd drift payments --json"},
		Flags:         []Flag{{Name: "json", Type: "bool", Description: "Emit stable JSON Lines."}},
	},
	{
		Name: "recurring", Usage: "specd recurring record <spec> --check <id> --head <sha> --release <id> --config <id> --verdict pass|fail --observed-at <RFC3339>",
		Description: "Validate and append an externally executed recurring-check result.", AllowedPhases: anyPhase(), ExitCodes: stdCodes(),
		Examples: []string{"specd recurring record payments --check api-health --head 0123456789012345678901234567890123456789 --release rel-7 --config prod-v3 --verdict pass --observed-at 2026-01-01T00:00:00Z"},
		Flags:    []Flag{{Name: "check", TakesValue: true, Type: "string", Description: "Recurring check identity."}, {Name: "head", TakesValue: true, Type: "string", Description: "Tested git HEAD."}, {Name: "release", TakesValue: true, Type: "string", Description: "Tested release identity."}, {Name: "config", TakesValue: true, Type: "string", Description: "Tested configuration identity."}, {Name: "verdict", TakesValue: true, Type: "string", Enum: []string{"pass", "fail"}, Values: "pass|fail", Description: "Check verdict."}, {Name: "observed-at", TakesValue: true, Type: "string", Description: "Explicit RFC3339 observation time."}},
	},
	{
		Name:          "spike",
		Usage:         "specd spike <spec> --question <q> --scope <s> --expiry <RFC3339> [--output <ref>]",
		Description:   "Record a bounded exploratory spike (learning without a completion or approval bypass).",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd spike payments --question 'is webhook retry idempotent?' --scope 'payments/webhook' --expiry 2026-07-19T00:00:00Z"},
		Flags: []Flag{
			{Name: "question", TakesValue: true, Type: "string", Description: "Bounded question the spike explores (required)."},
			{Name: "scope", TakesValue: true, Type: "string", Description: "Bounded scope of the exploration (required)."},
			{Name: "expiry", TakesValue: true, Type: "string", Description: "RFC3339 instant after which the spike is stale (required, must be in the future)."},
			{Name: "output", TakesValue: true, Type: "string", Description: "Optional reference to the spike's output (attaches to a decision; never satisfies task evidence)."},
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
		Usage:         "specd status [spec] [--json] | specd status <spec> --guide [--json] | specd status --program",
		Description:   "Report current spec and task state, machine driving guidance, or the cross-spec program view.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd status payments", "specd status payments --json", "specd status payments --guide --json", "specd status --program"},
		Flags: []Flag{
			{Name: "json", Type: "bool", Description: "Emit machine-readable status."},
			{Name: "guide", Type: "bool", Description: "Emit machine driving guidance: phase, required artifact, legal commands, human-only actions, and blockers."},
			{Name: "program", Type: "bool", Description: "Show the cross-spec program view: specs, links, phases, and frontier."},
		},
	},
	{
		Name:          "task",
		Usage:         "specd task <id> [--override --reason <text>]",
		Description:   "Show task details or clear an escalated task with a human override.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd task T3 --json", "specd task T3 --override --reason 'flaky infra, verified manually'"},
		Flags: []Flag{
			{Name: "json", Type: "bool", Description: "Emit machine-readable task row."},
			{Name: "override", Type: "bool", Description: "Clear an escalated task (resets the verify-failure ratchet; does not complete it). Requires --reason."},
			{Name: "reason", TakesValue: true, Type: "string", Description: "Human justification for --override (required, non-empty)."},
		},
	},
	{
		Name:          "complete-task",
		Usage:         "specd complete-task <spec> <id>",
		Description:   "Complete one task by consuming current passing evidence through the gated completion transaction.",
		AllowedPhases: postRequirementsPhases(),
		SpecSlugArg:   argAt(0),
		RequiresTask:  true,
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd complete-task payments T3"},
		Flags: []Flag{
			{Name: "session", TakesValue: true, Type: "string", Description: "Driver session id. Required while a session is open; mint the accompanying nonce with `specd session action`.", Values: "id minted by `specd session open`"},
			{Name: "nonce", TakesValue: true, Type: "string", Description: "Single-use operation nonce. Spent on use; a replay is refused.", Values: "minted by `specd session action`"},
			{Name: "tokens", TakesValue: true, Type: "string", Description: "Optional worker-reported token count, stored verbatim."},
			{Name: "cost", TakesValue: true, Type: "string", Description: "Optional worker-reported cost as a decimal string, stored verbatim."},
			{Name: "duration-ms", TakesValue: true, Type: "string", Description: "Optional worker-reported wall-clock milliseconds, stored verbatim."},
			{Name: "input-tokens", TakesValue: true, Type: "string", Description: "Optional provider-neutral input token count."},
			{Name: "output-tokens", TakesValue: true, Type: "string", Description: "Optional provider-neutral output token count."},
			{Name: "cached-tokens", TakesValue: true, Type: "string", Description: "Optional provider-neutral cached token count."},
			{Name: "provider", TakesValue: true, Type: "string", Description: "Optional bounded provider identifier."},
			{Name: "model", TakesValue: true, Type: "string", Description: "Optional bounded model identifier."},
			{Name: "currency", TakesValue: true, Type: "string", Description: "Currency unit required with canonical cost."},
			{Name: "pricing-ref", TakesValue: true, Type: "string", Description: "Pricing reference required with canonical cost."},
			{Name: "telemetry-source", TakesValue: true, Type: "string", Enum: []string{"worker", "provider_adapter", "operator"}, Description: "Telemetry provenance."},
			{Name: "attestation-ref", TakesValue: true, Type: "string", Description: "Optional external attestation reference."},
		},
	},
	{
		Name:          "check",
		Usage:         "specd check <spec> [--security] [--schema] [--schema-only] [--json]",
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
		RequiresTask:  true,
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd verify payments T3", "specd verify payments T3 --revert-on-fail", "specd verify payments --criterion 1.2 --status pass --evidence 'covered by T3 integration test'"},
		Flags: []Flag{
			{Name: "revert-on-fail", Type: "bool", Description: "Restore working tree on verify failure."},
			{Name: "sandbox", Type: "bool", Description: "Run the verify line inside a bwrap sandbox (fail-closed if the binary is absent)."},
			{Name: "sandbox-binary", TakesValue: true, Type: "string", Description: "Path to sandbox binary (overrides auto-detect)."},
			{Name: "criterion", TakesValue: true, Type: "string", Description: "Record evidence for acceptance criterion <r>.<n> instead of running a task verify."},
			{Name: "status", TakesValue: true, Type: "string", Enum: []string{"pass", "fail"}, Values: "pass|fail", Description: "Criterion verdict (with --criterion): pass|fail."},
			{Name: "evidence", TakesValue: true, Type: "string", Description: "Evidence text or path backing the criterion verdict (with --criterion)."},
			{Name: "session", TakesValue: true, Type: "string", Description: "Driver session id. Required while a session is open; mint the accompanying nonce with `specd session action`.", Values: "id minted by `specd session open`"},
			{Name: "nonce", TakesValue: true, Type: "string", Description: "Single-use operation nonce. Spent on use; a replay is refused.", Values: "minted by `specd session action`"},
			{Name: "tokens", TakesValue: true, Type: "string", Description: "Optional worker-reported token count, stored verbatim."},
			{Name: "cost", TakesValue: true, Type: "string", Description: "Optional worker-reported cost as a decimal string, stored verbatim."},
			{Name: "duration-ms", TakesValue: true, Type: "string", Description: "Optional worker-reported wall-clock milliseconds, stored verbatim."},
			{Name: "input-tokens", TakesValue: true, Type: "string", Description: "Optional provider-neutral input token count."},
			{Name: "output-tokens", TakesValue: true, Type: "string", Description: "Optional provider-neutral output token count."},
			{Name: "cached-tokens", TakesValue: true, Type: "string", Description: "Optional provider-neutral cached token count."},
			{Name: "provider", TakesValue: true, Type: "string", Description: "Optional bounded provider identifier."},
			{Name: "model", TakesValue: true, Type: "string", Description: "Optional bounded model identifier."},
			{Name: "currency", TakesValue: true, Type: "string", Description: "Currency unit required with canonical cost."},
			{Name: "pricing-ref", TakesValue: true, Type: "string", Description: "Pricing reference required with canonical cost."},
			{Name: "telemetry-source", TakesValue: true, Type: "string", Enum: []string{"worker", "provider_adapter", "operator"}, Description: "Telemetry provenance."},
			{Name: "attestation-ref", TakesValue: true, Type: "string", Description: "Optional external attestation reference."},
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
		Usage:         "specd mcp | specd mcp --config <host> [--root <path>] [--spec <slug>]",
		Description:   "Serve the MCP integration surface over stdio, or print a host config snippet.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd mcp", "specd mcp --config claude-code --spec demo"},
		Flags: []Flag{
			{Name: "config", TakesValue: true, Type: "string", Description: "Print a paste-ready MCP config snippet for a host (e.g. claude-code)."},
			{Name: "root", TakesValue: true, Type: "string", Description: "Pin the server working directory in the snippet."},
			{Name: "spec", TakesValue: true, Type: "string", Description: "Pin the active spec in the snippet."},
		},
	},
	{
		Name:          "handshake",
		Usage:         "specd handshake bootstrap [<spec>] [--json] [--expect-<identity> <value>]",
		Description:   "Emit a complete, drift-safe bootstrap identity packet.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd handshake bootstrap", "specd handshake bootstrap --json"},
		Flags: []Flag{
			{Name: "json", Type: "bool", Description: "Emit machine-readable handshake."},
			{Name: "expect-palette-digest", TakesValue: true, Type: "string", Description: "Fail (exit 1) if the command-palette digest differs."},
			{Name: "expect-config-digest", TakesValue: true, Type: "string", Description: "Fail (exit 1) if the effective-config digest differs."},
			{Name: "expect-managed-digest", TakesValue: true, Type: "string", Description: "Fail if managed guidance differs."},
			{Name: "expect-binary-version", TakesValue: true, Type: "string", Description: "Fail if binary version differs."},
			{Name: "expect-binary-commit", TakesValue: true, Type: "string", Description: "Fail if binary commit differs."},
			{Name: "expect-state-schema", TakesValue: true, Type: "string", Description: "Fail if state schema differs."},
			{Name: "expect-context-schema", TakesValue: true, Type: "string", Description: "Fail if context schema differs."},
			{Name: "expect-template-schema", TakesValue: true, Type: "string", Description: "Fail if template schema differs."},
			{Name: "expect-root", TakesValue: true, Type: "string", Description: "Fail if workspace root differs."},
			{Name: "expect-spec", TakesValue: true, Type: "string", Description: "Fail if active spec differs."},
			{Name: "expect-revision", TakesValue: true, Type: "string", Description: "Fail if state revision differs."},
		},
	},
	{
		Name:          "drive",
		Usage:         "specd drive <spec> [--json] [--sandbox]",
		Description:   "Emit the single next-action envelope: session, revision, assurance, permitted actor, legal operations, selected task, authority, context digest, blockers, and the exact next command. A projection over the granular commands, which keep working unchanged.",
		AllowedPhases: postRequirementsPhases(),
		SpecSlugArg:   argAt(0),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd drive payments --json", "specd drive payments --sandbox --json"},
		Flags: []Flag{
			{Name: "json", Type: "bool", Description: "Emit the machine-readable drive envelope."},
			{Name: "sandbox", Type: "bool", Description: "Declare that the invoking host isolates execution. Raises the reported assurance ceiling; absent, the session is advisory."},
		},
	},
	{
		Name:          "session",
		Usage:         "specd session <open|show|action|ack|close> <spec> [<task>] [--driver <host>] [--tokens <n>] [--json]",
		Description:   "Manage the driver session that binds a host to one spec's mutable work. `action` mints the single-use nonce and bindings a mutable operation must carry; `ack` records the host's context receipt, without which mutable authority stays withheld.",
		AllowedPhases: postRequirementsPhases(),
		SpecSlugArg:   argAt(1),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd session open payments --driver claude-code --json", "specd session ack payments T1 --tokens 4200", "specd session action payments --json", "specd session close payments"},
		Flags: []Flag{
			{Name: "json", Type: "bool", Description: "Emit machine-readable session packet."},
			{Name: "driver", TakesValue: true, Type: "string", Description: "Host identity opening the session (required by `open`)."},
			{Name: "tokens", TakesValue: true, Type: "string", Description: "Host-reported context token count recorded by `ack`. Recorded, never trusted as the harness estimate."},
			{Name: "partial", Type: "bool", Description: "Acknowledge no required context lane, proving the withholding path (`ack`)."},
		},
	},
	{
		Name:          "brain",
		Usage:         "specd brain <start|step|run|status|cancel|resume|claim|heartbeat|report> <spec> [args] [--authority]",
		Description:   "Run the opt-in deterministic orchestration controller. Mission ids (the `claim` argument) are minted by brain dispatch and listed by `specd brain status` — never invented by a worker.",
		AllowedPhases: postRequirementsPhases(),
		SpecSlugArg:   argAt(1),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd brain start payments --authority", "specd brain claim payments payments.s1.T1 worker-1 craftsman", "specd brain heartbeat payments <lease-id> worker-1", "specd brain report payments <lease-id> worker-1"},
		Flags: []Flag{
			{Name: "authority", Type: "bool", Description: "Grant dispatch authority (fail-closed by default)."},
		},
	},
	{
		Name:          "report",
		Usage:         "specd report <spec> [--pr|--metrics|--efficiency|--rollup|--delivery|--outcome-review|--json|--history|--trace|--format prometheus|event] | specd report --portfolio",
		Description:   "Render evidence-backed status, PR, history, trace, and metrics reports.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd report payments --pr", "specd report payments --metrics", "specd report payments --history", "specd report payments --trace", "specd report payments --format prometheus", "specd report payments --format event"},
		Flags: []Flag{
			{Name: "pr", Type: "bool", Description: "Emit PR-oriented report."},
			{Name: "metrics", Type: "bool", Description: "Emit metrics summary."},
			{Name: "efficiency", Type: "bool", Description: "Emit deterministic context-efficiency report with explicit unknown values."},
			{Name: "rollup", Type: "bool", Description: "Emit exact cross-spec economic roll-up with explicit missing telemetry."},
			{Name: "delivery", Type: "bool", Description: "Emit deterministic deployment status with adapter and trust source labeled separately."},
			{Name: "portfolio", Type: "bool", Description: "Emit deterministic cross-spec release/environment status and blockers from local ledgers."},
			{Name: "outcome-review", Type: "bool", Description: "Join local change evidence to release and incident references, preserving missing outcomes as unknown."},
			{Name: "json", Type: "bool", Description: "Emit machine-readable report (JSON Lines with --history)."},
			{Name: "history", Type: "bool", Description: "Replay the spec's audit trail from existing records in timestamp order."},
			{Name: "trace", Type: "bool", Description: "Export the metadata-only run trace as stable JSON Lines."},
			{Name: "format", TakesValue: true, Type: "string", Enum: []string{"prometheus", "event"}, Description: "Alternate output format; event emits neutral local JSONL, prometheus emits metrics."},
		},
	},
	{
		Name:          "link",
		Usage:         "specd link <from-slug> <to-slug> [--kind <kind>] [--reason <text>]",
		Description:   "Record a typed, traceable cross-spec dependency link.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd link api auth", "specd link api-v2 api --kind supersedes --reason 'replace obsolete contract'"},
		Flags: []Flag{
			{Name: "kind", TakesValue: true, Type: "string", Enum: []string{"follows", "regresses", "maintains", "supersedes"}, Description: "Link kind (default: follows)."},
			{Name: "reason", TakesValue: true, Type: "string", Description: "Optional human-authored reason stored with the link."},
		},
	},
	{
		Name:          "unlink",
		Usage:         "specd unlink <from-slug> <to-slug>",
		Description:   "Remove a cross-spec dependency link.",
		AllowedPhases: anyPhase(),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd unlink api auth"},
	},
	{
		Name:          "review",
		Usage:         "specd review <spec> [--force]",
		Description:   "Scaffold the review report the auditor fills before completion.",
		AllowedPhases: postExecutionPhases(),
		SpecSlugArg:   argAt(0),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd review payments", "specd review payments --force"},
		Flags: []Flag{
			{Name: "force", Type: "bool", Description: "Overwrite an existing report for the current git HEAD."},
		},
	},
	{
		Name:          "submit",
		Usage:         "specd submit <spec> [--resubmit]",
		Description:   "Run every gate, then stream the PR summary to the operator-configured submit command.",
		AllowedPhases: postExecutionPhases(),
		SpecSlugArg:   argAt(0),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd submit payments", "specd submit payments --resubmit"},
		Flags: []Flag{
			{Name: "resubmit", Type: "bool", Description: "Allow resubmitting a spec already submitted at the current git HEAD."},
		},
	},
	{
		Name:          "release",
		Usage:         "specd release candidate <spec> --artifact-digest <d> --sbom-ref <r> --provenance-ref <r>",
		Description:   "Freeze an immutable, reproducible release candidate identity into releases.jsonl. Builds and uploads nothing.",
		AllowedPhases: anyPhase(),
		SpecSlugArg:   argAt(1),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd release candidate payments --artifact-digest sha256:abc --sbom-ref sbom://payments --provenance-ref prov://payments"},
		Flags: []Flag{
			{Name: "artifact-digest", TakesValue: true, Type: "string", Description: "Content digest of the already-built artifact (a reference; release never builds)."},
			{Name: "sbom-ref", TakesValue: true, Type: "string", Description: "Reference to the artifact's SBOM."},
			{Name: "provenance-ref", TakesValue: true, Type: "string", Description: "Reference to the artifact's provenance attestation."},
		},
	},
	{
		Name:          "deploy",
		Usage:         "specd deploy <spec> --release <id> --environment <env> --adapter <a> --authority <auth> [--strategy <s>] [--population <p>] [--window <w>] [--idempotency-key <k>]",
		Description:   "Append a monotonic deployment attempt to deployments.jsonl under the spec lock. Drives no external system.",
		AllowedPhases: anyPhase(),
		SpecSlugArg:   argAt(0),
		ExitCodes:     stdCodes(),
		Examples:      []string{"specd deploy payments --release a1b2c3 --environment staging --adapter shell --authority ci"},
		Flags: []Flag{
			{Name: "release", TakesValue: true, Type: "string", Description: "Frozen release-candidate id to deploy."},
			{Name: "environment", TakesValue: true, Type: "string", Description: "Target environment (development|staging|production)."},
			{Name: "adapter", TakesValue: true, Type: "string", Description: "Deployment adapter name (a reference; core drives nothing)."},
			{Name: "authority", TakesValue: true, Type: "string", Description: "Authority under which the attempt is recorded."},
			{Name: "strategy", TakesValue: true, Type: "string", Description: "Rollout strategy label."},
			{Name: "population", TakesValue: true, Type: "string", Description: "Target population label."},
			{Name: "window", TakesValue: true, Type: "string", Description: "Observation window label."},
			{Name: "idempotency-key", TakesValue: true, Type: "string", Description: "Caller-supplied idempotency key for the attempt."},
		},
	},
}

type operationDefinition struct {
	id, subcommand       string
	usage                string
	actor                OperationActor
	effect               OperationEffect
	authorityRequired    bool
	taskRequired         bool
	scopeSource, network string
	examples             []string
}

// operationDefinitions contains only commands whose public operations need a
// narrower identity than their top-level verb. Single-operation commands are
// materialized from their Command declaration by buildOperations.
var operationDefinitions = map[string][]operationDefinition{
	"complete-task": {{id: "complete-task", effect: EffectStateWrite, authorityRequired: true, taskRequired: true, scopeSource: "task"}},
	"agents": {
		{id: "agents.inspect", effect: EffectRead, scopeSource: "workspace"},
		{id: "agents.doctor", subcommand: "doctor", effect: EffectRead, scopeSource: "workspace"},
		{id: "agents.guide", subcommand: "guide", effect: EffectRead, scopeSource: "spec"},
	},
	"eval": {
		{id: "eval.import", subcommand: "import", effect: EffectStateWrite, scopeSource: "spec"},
		{id: "eval.status", subcommand: "status", effect: EffectRead, scopeSource: "spec"},
	},
	"incident": {{id: "incident.seed", subcommand: "seed", actor: ActorOperator, effect: EffectWorkspaceWrite, authorityRequired: true, scopeSource: "spec"}},
	"exception": {
		{id: "exception.approve", subcommand: "approve", actor: ActorHuman, effect: EffectStateWrite, authorityRequired: true, scopeSource: "governed-exception"},
		{id: "exception.revoke", subcommand: "revoke", actor: ActorHuman, effect: EffectStateWrite, authorityRequired: true, scopeSource: "governed-exception"},
	},
	"recurring": {{id: "recurring.record", subcommand: "record", actor: ActorOperator, effect: EffectStateWrite, authorityRequired: true, scopeSource: "spec"}},
	"report":    {{id: "report.render", effect: EffectRead, scopeSource: "arguments"}},
	"task": {
		{id: "task.show", effect: EffectRead, scopeSource: "task"},
		{id: "task.override", actor: ActorHuman, effect: EffectStateWrite, authorityRequired: true, taskRequired: true, scopeSource: "task"},
	},
	"verify": {
		{id: "verify.task", effect: EffectStateWrite, authorityRequired: true, taskRequired: true, scopeSource: "task"},
		{id: "verify.criterion", effect: EffectStateWrite, authorityRequired: true, scopeSource: "criterion"},
	},
	"brain": {
		{id: "brain.start", subcommand: "start", actor: ActorOperator, effect: EffectStateWrite, authorityRequired: true, scopeSource: "authority"},
		{id: "brain.step", subcommand: "step", actor: ActorOperator, effect: EffectStateWrite, authorityRequired: true, scopeSource: "authority"},
		{id: "brain.run", subcommand: "run", actor: ActorOperator, effect: EffectStateWrite, authorityRequired: true, scopeSource: "authority"},
		{id: "brain.status", subcommand: "status", actor: ActorOperator, effect: EffectRead, scopeSource: "spec"},
		{id: "brain.cancel", subcommand: "cancel", actor: ActorOperator, effect: EffectStateWrite, authorityRequired: true, scopeSource: "authority"},
		{id: "brain.resume", subcommand: "resume", actor: ActorOperator, effect: EffectStateWrite, authorityRequired: true, scopeSource: "authority"},
		{id: "brain.claim", subcommand: "claim", actor: ActorOperator, effect: EffectStateWrite, authorityRequired: true, taskRequired: true, scopeSource: "authority"},
		{id: "brain.heartbeat", subcommand: "heartbeat", actor: ActorOperator, effect: EffectStateWrite, authorityRequired: true, taskRequired: true, scopeSource: "authority"},
		{id: "brain.report", subcommand: "report", actor: ActorOperator, effect: EffectStateWrite, authorityRequired: true, taskRequired: true, scopeSource: "authority"},
	},
}

// Operations is stable in command order, then declaration order.
var Operations = buildOperations()

func buildOperations() []Operation {
	var operations []Operation
	for _, command := range Commands {
		definitions := operationDefinitions[command.Name]
		if len(definitions) == 0 {
			definitions = []operationDefinition{{id: command.Name, actor: defaultOperationActor(command), effect: defaultOperationEffect(command.Name), authorityRequired: defaultAuthorityRequired(command.Name), taskRequired: command.RequiresTask, scopeSource: defaultScopeSource(command.Name), network: defaultNetworkClass(command.Name)}}
		}
		for _, definition := range definitions {
			actor := definition.actor
			if actor == "" {
				actor = defaultOperationActor(command)
			}
			usage := definition.usage
			if usage == "" {
				usage = command.Usage
			}
			examples := definition.examples
			if len(examples) == 0 {
				examples = command.Examples
			}
			network := definition.network
			if network == "" {
				network = defaultNetworkClass(command.Name)
			}
			operations = append(operations, Operation{
				ID: definition.id, Command: command.Name, Subcommand: definition.subcommand,
				Usage: usage, Actor: actor, Effect: definition.effect,
				AllowedPhases:     append([]Phase(nil), command.AllowedPhases...),
				AuthorityRequired: definition.authorityRequired, TaskRequired: definition.taskRequired,
				ScopeSource: definition.scopeSource, NetworkClass: network,
				ExitCodes: append([]ExitCode(nil), command.ExitCodes...), Examples: append([]string(nil), examples...),
			})
		}
	}
	return operations
}

func defaultOperationActor(command Command) OperationActor {
	if command.HumanOnly {
		return ActorHuman
	}
	switch command.Name {
	case "deploy", "recurring", "release", "submit":
		return ActorOperator
	default:
		return ActorAgent
	}
}

func defaultOperationEffect(command string) OperationEffect {
	switch command {
	case "help", "version", "agents", "adapters", "drift", "next", "status", "context", "handshake", "report", "mcp":
		return EffectRead
	case "init", "new", "incident", "review":
		return EffectWorkspaceWrite
	case "submit":
		return EffectExternal
	default:
		return EffectStateWrite
	}
}

func defaultAuthorityRequired(command string) bool {
	switch command {
	case "approve", "mode", "midreq", "decision", "deploy", "release", "recurring", "submit":
		return true
	default:
		return false
	}
}

func defaultScopeSource(command string) string {
	switch command {
	case "help", "version", "mcp":
		return "none"
	case "init", "agents", "adapters", "new":
		return "workspace"
	case "link", "unlink":
		return "spec-pair"
	case "status", "report":
		return "arguments"
	default:
		return "spec"
	}
}

func defaultNetworkClass(command string) string {
	switch command {
	case "mcp":
		return "stdio"
	case "submit":
		return "operator-configured"
	default:
		return "none"
	}
}

func OperationByID(id string) (Operation, bool) {
	for _, operation := range Operations {
		if operation.ID == id {
			return operation, true
		}
	}
	return Operation{}, false
}

func OperationsForCommand(command string) []Operation {
	var operations []Operation
	for _, operation := range Operations {
		if operation.Command == command {
			operations = append(operations, operation)
		}
	}
	return operations
}

// ResolveOperation maps one concrete CLI invocation to its canonical public
// operation. Mixed-command unknown subcommands fail closed before dispatch.
func ResolveOperation(command string, args []string, flags map[string]string) (Operation, bool) {
	id := command
	first := ""
	if len(args) > 0 {
		first = args[0]
	}
	switch command {
	case "agents":
		switch first {
		case "", "inspect":
			// `specd agents inspect` aliases bare `specd agents`, matching the
			// palette id agents.inspect (spec R4.2). Dispatch strips the alias
			// token before the handler runs.
			id = "agents.inspect"
		case "doctor", "guide":
			id = "agents." + first
		default:
			return Operation{}, false
		}
	case "eval", "exception", "brain":
		if first == "" {
			return Operation{}, false
		}
		id = command + "." + first
	case "incident":
		if first != "seed" {
			return Operation{}, false
		}
		id = "incident.seed"
	case "recurring":
		if first != "record" {
			return Operation{}, false
		}
		id = "recurring.record"
	case "task":
		switch {
		case first == "complete":
			return Operation{}, false
		case flagSet(flags, "override"):
			id = "task.override"
		default:
			id = "task.show"
		}
	case "verify":
		if flagSet(flags, "criterion") {
			id = "verify.criterion"
		} else {
			id = "verify.task"
		}
	case "report":
		id = "report.render"
	}
	return OperationByID(id)
}

func flagSet(flags map[string]string, name string) bool {
	_, ok := flags[name]
	return ok
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

// Guidance is the machine-readable driving guidance for one lifecycle phase
// (spec 01 R6.1). It separates what a machine actor may legally do
// (LegalCommands) from what only a human may do (HumanOnly, e.g. approval), so
// an agent never treats approval as a self-serve action; it also names the
// artifact the phase must produce (RequiredArtifact) and any Blockers the caller
// resolved from state. NextGate is the gate a human must clear to advance.
type Guidance struct {
	Status           Status   `json:"status"`
	Phase            Phase    `json:"phase"`
	RequiredArtifact string   `json:"required_artifact,omitempty"`
	NextGate         Status   `json:"next_gate,omitempty"`
	LegalCommands    []string `json:"legal_commands"`
	HumanOnly        []string `json:"human_only"`
	Blockers         []string `json:"blockers,omitempty"`
}

// GuidanceForPhase builds the driving guidance for status. Blockers are supplied
// by the caller (the deterministic gate failures for the next transition); this
// function is pure over the command palette. Deferred verbs and verbs not legal
// in the phase are omitted; human-only verbs are listed separately so an agent
// cannot mistake approval for a machine action (spec 01 R6).
func GuidanceForPhase(status Status, blockers []string) Guidance {
	phase := PhaseForStatus(status)
	g := Guidance{
		Status:           status,
		Phase:            phase,
		RequiredArtifact: RequiredArtifact(status),
		NextGate:         NextStatus(status),
		Blockers:         blockers,
	}
	for _, cmd := range Commands {
		if cmd.Deferred || !cmd.AllowsPhase(phase) {
			continue
		}
		if cmd.HumanOnly {
			g.HumanOnly = append(g.HumanOnly, cmd.Name)
		} else {
			g.LegalCommands = append(g.LegalCommands, cmd.Name)
		}
	}
	return g
}

// RequiredArtifact is the source artifact a lifecycle status is producing, or ""
// once the planning artifacts are authored (execution phases produce evidence,
// not a single artifact).
func RequiredArtifact(status Status) string {
	switch status {
	case StatusRequirements:
		return "requirements.md"
	case StatusDesign:
		return "design.md"
	case StatusTasks:
		return "tasks.md"
	}
	return ""
}

// NextStatus returns the status immediately after status in the lifecycle order,
// or "" when status is already final. It names the gate an actor must clear next
// (spec 01 R6.1 driving guidance).
func NextStatus(status Status) Status {
	for i, s := range statusOrder {
		if s == status && i+1 < len(statusOrder) {
			return statusOrder[i+1]
		}
	}
	return ""
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
