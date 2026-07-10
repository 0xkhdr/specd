# Design — Deterministic security enforcement

## Decision

Turn advisory/opt-in security into profile-gated enforcement. A resolved `security_profile`
selects a required-gate set and a `policy_digest`. Authority becomes a machine-readable packet the
dispatch/MCP boundary enforces, not role prose. Scope is derived by the harness from a pinned
baseline, never trusted from the worker. Context is scanned and trust-labelled before it reaches
the model, including the `.specd/` runtime tree. Production verify runs in a required sandbox that
hides secrets and bounds resources. Exceptions become a governed append-only ledger. External
supply-chain and cross-platform sandbox facts arrive as pinned adapter artifacts that deterministic
core validates offline.

```text
profile resolve → policy_digest ──────────────────────────────┐
authority packet (role, tools, paths, net, sandbox, expiry) ── │
        ↓                                                      │
context build → scan .specd + untracked → trust label ─────────┼─> dispatch (default-deny)
        ↓                                                      │
worker edits (declared paths only) → verify in sandbox ────────┤
        ↓                                                      │
completion → harness diff vs declared scope → required scans ──┼─> evidence pinned to digest
        ↓                                                      │
governed exception (optional) → mission audit view ────────────┘
```

Prototype warns/confirms; production makes every gate mandatory and default-deny. No profile
downgrade is silent — migration diagnostics only.

## Policy resolution and digest

`config_loader.go` resolves `security_profile` and a per-scanner/per-policy severity map. All
resolved policy serializes canonically to a stable `policy_digest` (same inputs → same digest).
The digest pins to the mission envelope (Domain 05), the security/verify evidence record
(Domain 04), and the report. `submit` refuses when the recorded security digest is absent or does
not match the subject revision, returning `security_evidence_stale` with the exact next command.

```text
SecurityProfileV1
  profile (prototype|production), policy_version, policy_digest,
  scanner_severities{secrets,injection,slopsquat,dangerous,authz,generated,symlink},
  required_gates[], sandbox{required, profile, limits}, network_default (deny|allow),
  dependency{registry_allowlist, require_reason, adapter_refs[]}
```

## Authority packet and tool policy

Role stops being free text. `gates/core.go` role gate accepts only the four documented roles;
`roles.go` removes the craftsman fallback for unknown/auditor. The packet is emitted with the
mission and echoed by the worker; dispatch and MCP enforce it.

```text
AuthorityV1
  actor_id, worker_id, spec_id, task_id, phase, role,
  mode (read_only|write), allowed_tools[]{id, arg_constraints},
  denied_tools[], declared_read_paths[], declared_write_paths[],
  network_policy, sandbox_profile, baseline_revision, expires_at, policy_digest
```

Enforcement: scout/validator/auditor → `mode=read_only`, any write tool call denied and a
sanitized denial event appended. Craftsman → writes only normalized `declared_write_paths`.
Unknown tool under production → deny. Stale/expired/wrong-phase packet → fail before work. When
`specd` cannot control an external host, the packet requires a host attestation and the resulting
diff is still verified independently (scope gate below).

## Scope from harness diff

New scope/diff core package computes the change set from the pinned `baseline_revision`: tracked,
staged, untracked, deleted, renamed, mode, symlink. Paths normalize repo-relative; `../`, absolute,
and symlink escapes fail. Declared globs match the normalized set; tests must be declared. The
worker's `changed_files_claim` is retained as an audit hint and compared, never substituted. Any
out-of-scope entry → `outside_scope`, completion refused. Naïve prefix matching is avoided:
renames/symlinks/submodules/nested repos handled explicitly (see risks).

## Context scan and trust boundary

`gates/security/gate.go` gains a scanner input abstraction so the enumeration source is not
hard-coded to git-tracked non-`.specd` files:

```text
ScanInputV1
  root, item_ref, kind (spec|steering|role|memory|source|untracked|tool_result),
  trust (trusted_instruction|untrusted_data), digest
```

The scan covers the runtime `.specd/` tree and the pending untracked/generated set. Failed
`git ls-files` or an unreadable in-scope file → error finding (never empty-green). Findings carry
exact refs and bounded safe excerpts; secret values and full injection payloads are never inlined.
Only embedded/versioned role and policy material is labelled `trusted_instruction`; everything else
is `untrusted_data` and cannot alter gate behavior.

## Sandbox and redaction

`verify/exec.go` production path requires the sandbox before shell start; a missing binary/adapter
fails closed. The wrapper adds: network off, synthetic/empty HOME, minimal PATH/env, private temp,
repo+temp-only writes, and hides host credential paths (read-only root is insufficient). Resource
limits (CPU, memory, process count, output bytes, wall timeout, filesystem growth) are expressed in
sandbox args; a breach records a failure. A central redactor filters stdout/stderr, evidence,
reports, and context so a secret fixture never appears in full. Platform specifics live behind an
adapter contract, not a runtime library.

## Dependency and dangerous-change policies

Dependency governance moves from Go-name distance to manifest-diff: new dependency needs declared
reason/source; unknown registry/checksum/provenance fails per profile; lockfile-only changes are
inspected. Vulnerability/provenance facts arrive as pinned offline adapter JSON; malformed/stale
artifact fails. Additional deterministic policies over the normalized diff/trace detect destructive
shell, world-writable/executable mode changes, auth/permission changes, generated secret files, and
path/symlink escapes, each with documented false-positive controls.

## Governed exceptions and audit

`allowlist.go` bare fingerprint+reason is replaced by `.specd/security/exceptions.jsonl`
(append-only). Required: finding/action, reason, ticket, owner, scope, revision/environment,
issue/expiry, compensating control. Expired/revoked/wrong-revision suppresses nothing; an edit
changes the digest. `approve`/`revoke` commands manage lifecycle. A unified `specd report`
projection keyed by run/mission/task and policy digest correlates authority, tools, diff, scans,
verify, review, exceptions, submit in order; duplicate/out-of-order ids fail import; no secrets,
raw sensitive args, or hidden reasoning.

## Verification ladder

1. Unit/golden: profile resolution + digest stability; authority packet parse/refusal; scope
   normalization; scanner input classification; exception schema; redaction.
2. Fail-closed: unknown role/tool/profile; missing sandbox; failed enumeration; stale security at
   submit; expired/approverless exception.
3. Black-box fixtures: out-of-scope tracked/untracked/rename/symlink; scout MCP write denial;
   `.specd` injection marker; untracked credential; sandbox network+credential probe; misspelled
   dependency; lockfile-only change; scoped exception re-surface on later revision.
4. Conformance: Linux/macOS/CI sandbox adapters equivalent policy outcome; local CLI/MCP denial
   parity; redaction across output/evidence/report/context.
5. Full race/vet/lint/regression after integrated domain; docs mirror; `go mod tidy` clean.
