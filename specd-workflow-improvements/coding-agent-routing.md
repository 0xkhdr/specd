# Coding-agent routing

## Domain definition

Decides whether a user request is ordinary coding, read-only Specd consultation, or governed
Specd-managed work, and whether repository enforcement requires managed handling.

## Current behavior

Generated `AGENTS.md` presents the bootstrap/task loop without an activation rule. Agent guidance
assumes a slug. Presence of `.specd` and repository instructions can therefore pull unrelated work
through the lifecycle. Current `ModeDefault|ModeAgent|ModeOrchestrated` describes spec execution,
not request routing.

## Evidence from feedback

The prompt identifies unrelated coding requests being interpreted through Specd as a major concern.
Feedback also shows repeated bootstrap/config/approval overhead before real work and an agent-facing
palette that disagreed with human-only actions.

## Main problems

- Repository installation is mistaken for per-request consent.
- Execution mode and request mode are conflated.
- Natural-language classification is nondeterministic and can surprise users.
- No read-only consultation mode exists.
- “Bypass” is framed as weakening Specd, when ordinary work should never have activated it.

## Root-cause analysis

Integration starts from a workflow recipe, not an intent router. Agent hosts load repository text
globally, while Specd authority is scoped per spec; missing glue defaults to the more intrusive path.

## Desired behavior

General mode is default. Managed mode is explicit or enforceably required. Consultation reads Specd
without attaching work. Router decisions are deterministic, disclosed, and independent of LLM
classification.

## Recommended design

Two dimensions:

```text
request_mode: general | consult | managed
enforcement: optional | required
```

Resolution precedence:

1. Explicit per-request directive: `/general`, `/specd-consult`, `/specd <slug>` or host equivalent.
2. Active session binding established by user.
3. Explicit repository enforcement rule matching path/branch, only when host enforces it.
4. Project `agent.routing_default`.
5. Compiled `general`.

Classifier may emit `recommend_managed` with reason/confidence, but user confirmation is needed and
no state mutates meanwhile. In required enforcement, general directive refuses before edit and names
policy/path; it is not a bypass.

General mode exposes no mutable Specd operations and need not run handshake. Consult exposes status,
check preview, report, config show, and docs only. Managed binds slug/intake and then uses normal
authority loop.

## Workflow implications

Normal bug fixes and questions lose all lifecycle overhead. Users can inspect a spec without joining
it. Managed work gains clearer entry and exit. Temporary suspension is `/general` only when no
enforcement rule or active uncommitted managed authority forbids it.

## Data-model implications

Store session binding separately from spec lifecycle: mode, source, selected slug, enforcement
rule id, actor, created/expiry time. Do not write a Specd event for general requests.

## CLI implications

CLI can expose `specd agent-mode show|general|consult|managed <slug>` for hosts, but generated guide
directives may be enough initially. Handshake/drive envelopes include mode/source/enforcement.

## Coding-agent implications

Agent discloses mode once before mutable work and on change. It never invokes Specd in general mode.
It may suggest managed mode only when requirements, approvals, or audit value justify cost.

## Compatibility implications

Existing spec execution `state.mode` remains separate and should be renamed/projected as
`execution_mode` to avoid collision. Existing sessions with explicit slug map managed.

## Failure scenarios

Conflicting explicit directive and required policy refuses; stale session falls back only after
disclosure; host lacking path enforcement cannot claim required mode and reports advisory; ambiguous
classifier recommendation asks once, then continues general if unanswered.

## Edge cases

General request touches file governed by policy after exploration; host must re-route before write.
One request may consult multiple specs but managed mode binds one mutable spec. Switching slugs
invalidates old authority.

## Testing strategy

Precedence matrix, generated-guide conformance, host capability cases, no-Specd-command assertion for
general mode, read-only enforcement for consult, authority invalidation on switch.

## Implementation recommendations

Start with guide contract and pure resolver. Avoid automatic classifier implementation until explicit
modes prove insufficient.

## Trade-offs

Explicit directives add one small ceremony when governance is desired, removing large ceremony from
every unrelated request. Required path policy depends on host support.

## Risks

Users may think general mode bypasses policy. UI must distinguish “not activated” from “required but
refused.” Hosts may ignore mode; assurance stays advisory.

## Acceptance criteria

- `.specd` presence alone resolves general.
- Consult cannot mutate.
- Managed requires slug/intake.
- Route source and enforcement are machine-readable.
- Classifier cannot silently activate managed.
- Required enforcement cannot be dismissed by general directive.

## Open questions

- Standard directive syntax across Codex, Claude Code, and MCP.
- Whether path enforcement ships before a reference host implements pre-write interception.

