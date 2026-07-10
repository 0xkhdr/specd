# Requirements — Security, permissions, and governance

## Scope

Add deterministic security enforcement around existing gates/verify/orchestration. Preserve
atomic writes, state CAS, reentrant spec lock, append-only evidence, no-bypass verify, human
approval, byte-stable task parser, offline/stdlib-only core, `go:embed` templates, and
compatibility migration. No LLM in any trust decision. No new runtime dependency.

### R1 — Operating profile and required gates

- R1.1: A versioned `security_profile` (`prototype` | `production`) shall resolve before the agent
  receives a mission. `prototype` may warn/require confirmation; `production` makes security,
  scope, sandbox, evidence, and review gates mandatory. Invalid/absent profile fails closed.
- R1.2: `CoreRegistry` completion and `submit` shall include required security gates under
  `production`. Submit shall refuse when security was not run or is stale for the subject revision,
  emitting a structured `security_evidence_stale` with one deterministic next command.
- R1.3: Every resolved policy shall produce a stable `policy_version`/`policy_digest`. Same inputs
  yield identical findings and digest. The digest is pinned to dispatch, evidence, and report.
- R1.4: Profile change shall be explicit with migration diagnostics, never a silent downgrade of an
  existing project's enforcement.

### R2 — Declared-file scope from harness diff

- R2.1: Declared paths shall be normalized repo-relative paths/globs. `../`, absolute paths, and
  symlink escapes fail closed. Test paths shall be declared explicitly; no implicit "plus tests".
- R2.2: At completion the harness shall derive the change set from a pinned task baseline: tracked,
  staged, untracked, deletion, rename, mode, and symlink changes. Worker-reported paths are audit
  hints and cannot override the derived set.
- R2.3: Any derived change outside declared scope shall fail completion with a `outside_scope`
  finding, even when verify passes and the worker reports only in-scope files.
- R2.4: The byte-stable tasks parser shall keep round-tripping after scope metadata is added.

### R3 — Role authority and tool policy

- R3.1: The role gate shall accept only the four documented roles (scout, craftsman, validator,
  auditor). Unknown role fails; no craftsman fallback for unknown/auditor mode resolution.
- R3.2: A machine-readable authority packet shall bind actor/worker, spec, task, phase, role,
  allowed/denied tool ids with argument constraints, declared read/write paths, network policy,
  sandbox profile, baseline revision, expiry, and policy digest.
- R3.3: At the dispatch and MCP boundaries, scout/validator/auditor write calls shall be denied and
  a sanitized denial event recorded. Craftsman writes outside declared paths shall be denied.
  Unknown tools default-deny under production.
- R3.4: Stale, expired, or wrong-phase authority packets shall fail closed before granting work.

### R4 — Context and change-boundary scanning

- R4.1: The context scan shall include the runtime `.specd/` tree (specs, steering, roles, memory)
  and the pending untracked/generated change set — not only git-tracked non-`.specd` files.
- R4.2: Failed git enumeration or an unreadable in-scope file shall yield an error finding, never an
  empty successful scan. Exclusions shall be explicit per scanner, not blanket trust.
- R4.3: Each context item shall carry a trust label (trusted instruction vs untrusted data) and a
  digest. Only embedded/versioned role and policy material renders as instruction. Untrusted text
  cannot alter gate behavior even when it contains an injection marker.
- R4.4: Findings shall reference exact locations and safe bounded excerpts; a secret value or full
  malicious payload shall never be inlined to explain a hit.

### R5 — Sandbox and secret isolation

- R5.1: Production verify shall run in a required sandbox: network off, synthetic/empty HOME,
  minimal PATH/env, private temp, controlled writable paths (repo + temp only). A missing sandbox
  binary/adapter fails closed before shell starts; no silent fallback.
- R5.2: Host credential paths (e.g. `$HOME/.aws/credentials`) shall be unavailable inside the
  sandbox; read-only root is not sufficient — secrets/sensitive paths shall be hidden.
- R5.3: Verify stdout/stderr, evidence, reports, and context shall pass central redaction; a secret
  fixture shall never appear in full in any of them.
- R5.4: Sandbox arguments shall express resource limits (CPU, memory, process, output size, wall
  timeout, filesystem growth). A breach records a failure record, not a silent kill.

### R6 — Dependency and dangerous-change governance

- R6.1: Dependency policy shall inspect the manifest diff (not just Go names) with declared
  reason/source for new dependencies; unknown registry/checksum/provenance fails per profile.
  Lockfile-only changes shall be inspected, not globally excluded.
- R6.2: External vulnerability/provenance evidence shall arrive as pinned offline adapter artifacts;
  malformed or stale artifact fails. The gate stays offline and stdlib-only.
- R6.3: Deterministic policies over normalized diffs/traces shall detect destructive shell,
  world-writable/executable mode changes, auth/permission policy changes, generated secret files,
  and path/symlink escapes, with documented false-positive controls.

### R7 — Governed exceptions and mission audit

- R7.1: An exception shall require exact finding/action, reason, ticket, owner, scope,
  revision/environment, issue/expiry, and compensating control. Missing any required field fails.
- R7.2: Exceptions are append-only; an edit changes the digest. Expired, revoked, or wrong-revision
  exceptions suppress nothing and re-surface the finding. Exceptions cannot waive evidence integrity
  or broaden worker authority. Reports show active and historical exceptions.
- R7.3: A unified sanitized audit view keyed by run/mission/task and policy digest shall correlate
  authority, tools, diff, scans, verify, review, exceptions, and submit in order. Duplicate or
  out-of-order identifiers fail import. No secrets, raw sensitive args, or hidden reasoning.

### R8 — Cross-platform adapters and regression governance

- R8.1: Sandbox capability negotiation shall let Linux/macOS/CI adapters declare capabilities;
  production refuses an adapter missing a required capability. Conformance fixtures produce
  equivalent policy outcomes across adapters. No runtime library dependency.
- R8.2: Promoted security incidents shall become deterministic regression fixtures with redacted
  provenance and an expected finding. Policy changes invalidate stale attestations. Trend reports
  require no model.

## Non-goals

- No cloud security SDK, vulnerability database, or sandbox library as a runtime dependency.
- No claim that `specd` sandboxes an arbitrary external coding-agent host; it enforces boundaries
  on processes/tools it owns and defines a conformance/attestation contract for hosts.
- No treating scanning as proof of absence, or a read-only root as a confidential root.
- No LLM, network call, or hidden chain-of-thought in any gate, scope, sandbox, or exception path.
- No exception that waives core evidence integrity or silently broadens authority.
