# Design — agent-driver-protocol

> The primitives already exist — handshake, guidance, context manifests, authority,
> leases, evidence, completion. What is missing is that they are separately callable,
> so an agent can enter anywhere and skip anything. This spec joins them into one
> session-scoped protocol and states the host contract that makes authority real.

- references: R1, R1.1, R1.2, R1.3, R2, R2.1, R2.2, R2.3, R2.4, R3, R3.1, R3.2, R4, R4.1, R4.2, R4.3, R4.4, R4.5, R5, R5.1, R5.2, R5.3, R5.4, R6, R6.1, R6.2, R6.3, R7, R7.1, R7.2
- disposition: accepted
- owner: maintainer

## Boundaries

- Owned: a `drive` verb and `session` operations in `internal/core/commands.go` and
  `internal/cmd/`, session state under `.specd/`, a diff-scope gate in
  `internal/core/gates/`, session/lease binding in `internal/orchestration/`, the host
  contract document, and one reference adapter.
- Excluded: the reasoning an agent performs, model prompting, and enforcement inside a
  host that declines the contract. specd states requirements and labels assurance; it
  cannot mediate a tool it does not sit in front of.
- Excluded: evidence and completion semantics, which are already correct and stay untouched.

## Interfaces

- `specd drive <slug> --json` — the single envelope of R1.1; a projection over existing
  handshake, guide, frontier, and authority code, not a second source of truth (R1.3).
- `specd session open <slug> --driver <host> --json`, `specd session action <session-id> --json`,
  `specd session close <session-id>` — session lifecycle (R2.1).
- Mutable operation preconditions: session id, expected revision, handshake digest, authority
  digest, context receipt, baseline revision, single-use nonce (R2.2). Any mismatch returns the
  typed refusal defined by agent-protocol-clarity (R2.3).
- Context receipt: manifest digest, supplied items, missing items, host token count, host and
  driver identity (R3.1). Missing required lane withholds mutable authority (R3.2).
- Diff-scope gate: `(baseline, worktree, declared paths, active leases) -> findings`, run before
  verify and complete on every transport (R4.5).
- Host contract document: the normative list of R5.1 through R5.4, with a declared capability
  set the host asserts at bootstrap and specd records alongside the assurance level.

## Invariants

- Determinism: session validation, nonce checks, diff-scope comparison, and overlap detection
  are pure functions of on-disk state and the git worktree. No LLM in any of them.
- Evidence integrity is untouched: completion still consumes a passing verify record pinned to a
  resolvable HEAD, and this spec adds no bypass. Diff-scope only ever refuses; it never satisfies.
- Fail closed: an unknown session, stale revision, reused nonce, absent receipt, or unresolvable
  baseline refuses the operation and leaves trusted state unmodified.
- Session state is written through `core.AtomicWrite`, mutations use CAS on the revision counter,
  and per-spec work stays serialized by the reentrant spec lock.
- Zero runtime dependencies; sessions are files under `.specd/`, not a server.
- Conformance events are observational only; no gate consults them (R7.2).

## Failure

- Host crash without close: sessions and leases carry an expiry, so state self-heals without
  manual repair; a resumed host opens a new session against the current revision.
- Baseline unresolvable (rewritten history, shallow clone): refuse with a typed refusal naming
  the recovery command, never silently rebase the mission.
- Diff-scope false positive on a generated file: refuse and name the declaration change needed.
  A bypass flag is explicitly not offered; the task declares the path or the task is wrong.
- Session churn from a poorly written adapter: the protocol is stateless per action apart from
  the nonce ledger, so repeated opens cost a file write, not a leak.
- Overlapping leases racing dispatch: detection runs under the spec lock before dispatch, so the
  loser is refused rather than both proceeding.

## Integration

- `internal/orchestration/` already owns leases, decisions, and the ACP ledger; sessions bind to
  leases there rather than growing a parallel scheduler (R6.1).
- MCP already enforces authority under the production profile; the diff-scope gate moves from
  that transport into the core gate registry so every transport inherits it (R4.5).
- `docs/command-reference.md` is generated from the palette; adding `drive` and `session` requires
  `go run ./tools/gendocs` or `scripts/docs-lint.sh` fails CI.
- `scripts/regress-domains.sh` gains a session/diff-scope domain invariant check.
- Depends on agent-protocol-clarity for assurance levels, typed refusals, and capability contracts.

## Alternatives

- Make `drive` a new decision engine that picks work itself — rejected: specd must stay the
  deterministic control plane around reasoning, not become an AI planner.
- Enforce scope by sandboxing inside specd — rejected: specd is a CLI, not a supervisor. The host
  owns the process boundary; specd owns the declaration and the post-hoc diff check.
- Solve cooperation gaps with longer role prose — rejected explicitly by the analysis; prose is
  not authority and does not survive a host that never reads it.
- Ship adapters for every host — deferred: one reference adapter proves the contract; more
  adapters are a follow-on once the contract is stable.

## Verification

- R1.1/R1.2/R1.3: golden-envelope test over `drive --json`, a refusal test for a non-driveable
  spec, and existing granular-command tests kept green.
- R2.1 to R2.4: table test over each binding — stale revision, reused nonce, wrong handshake
  digest, wrong authority digest, absent receipt, drifted baseline — asserting refusal and an
  unchanged `state.json` revision.
- R3.1/R3.2: test that authority stays read-only when a required lane is absent from the receipt.
- R4.1 to R4.5: diff-scope table test covering undeclared modify, create, delete, rename,
  pre-baseline change, direct task-marker edit, direct `.specd` mutation, and lease overlap; plus a
  test asserting the gate runs on the default profile.
- R5.1 to R5.4: contract conformance test in `internal/integration/` driving the reference adapter,
  and a test asserting a host declaring no sandbox support yields an advisory session.
- R6.1 to R6.3: orchestration tests for session/lease binding, pre-dispatch overlap detection, and
  stale mission report rejection.
- R7.1/R7.2: test asserting each listed event is recorded and that removing the event log changes
  no gate outcome.
- Whole suite: `go test ./... -race -count=1`, plus `-count=2` for iteration-order flakiness.

## Deployment

- Staged: sessions and `drive` ship first as opt-in alongside granular commands; the diff-scope
  gate ships next as a core gate; the reference adapter ships last.
- Existing projects keep working because granular commands remain (R1.3) and sessions are additive
  files under `.specd/`.
- Observation: conformance events (R7.1) show whether hosts actually enter the protocol.
- Ownership: maintainer.

## Rollback

- Trigger: the diff-scope gate refuses legitimate work at a rate that stalls delivery, or session
  preconditions break a shipped adapter.
- Path: the gate is registry-entry scoped and can be reverted independently of sessions; sessions
  can be reverted independently of `drive`, which is a projection over primitives that already
  exist. Session files are inert once the verbs are gone. No state migration is required in either
  direction, and no evidence or completion record is affected by any rollback step.
