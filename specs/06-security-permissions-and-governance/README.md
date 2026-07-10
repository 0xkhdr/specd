# Domain 06 — Security, permissions, and governance

## Goal

Move production security from advisory prose into deterministic, tool-gated enforcement.
Least-authority tool use, harness-derived task scope, safe context ingestion, isolated command
execution, dependency/secret controls, and accountable exceptions. No LLM in any trust decision.
No bypass of evidence integrity. Stdlib-only; external hosts covered by attestation/adapter
contract, not by importing sandbox or scanner SDKs.

## Source and intent

Derived from `docs/google-sdlc-alignment/README.md` and
`docs/google-sdlc-alignment/06-security-permissions-and-governance.md`.
Paper position: security is a property of the harness (instructions, tools, context policy,
hooks, sandboxes, observability), not model good behavior (`sdlc-paper.md:258-281`,
`242-254`, `274-281`, `380-390`, `416`). Authority must be scoped and observable; risky actions
constrained by the harness; production security cannot depend on the agent remembering prose.

Current state: role constitutions, `check --security` scanners (secrets/injection/slopsquat),
reasoned allowlist, verify env-scrub, `--sandbox` bwrap wrapper, `--revert-on-fail`, evidence
pinned to HEAD. Gap: roles grant no capability; declared files never compared to real diff;
security gates opt-in and excluded from `submit`; injection scan skips the `.specd/` runtime
tree; sandbox is CLI-flag, verify-only, reads unprotected; dependency scan is Go-name only;
allowlist lacks approver/expiry/scope; no unified policy digest across decisions.

## Ownership

| Area | Domain 06 owns | Other domain owns |
|---|---|---|
| Operating profile | prototype/production resolution, required-gate binding, policy digest | Domain 01 phase/approve semantics |
| Authority packet | role→mode map, allowed/denied tools, declared paths, network/sandbox policy | Domain 05 mission envelope/lease transport |
| Scope | harness-derived task diff, normalized-path scope verdict | Domain 05 completion/report correlation |
| Context trust | trust label/digest per item, pre-dispatch scan of runtime tree | Domain 02 context receipt/selection |
| Execution isolation | required sandbox contract, secret isolation, output redaction, resource limits | Domain 04 verify/evidence freshness |
| Dependency/change policy | manifest-diff, dangerous-command/authz/generated/symlink policies | Domain 10 external evidence adapter transport |
| Exceptions/audit | governed append-only ledger, policy-digest-keyed mission view | Domain 07 trusted measurement/export |

## Deliverable specs

| Wave | Slug | Result | Requires |
|---|---|---|---|
| W0 | `06a-security-contract-baseline` | observed behavior, corrected threat-model wording, failing fixtures for every P0 gap | — |
| W1 | `06b-operating-profiles-and-required-gates` | prototype/production profiles; security required at completion/submit for production; policy digest | 06a, Domain 01 approve |
| W2 | `06c-declared-scope-from-harness-diff` | normalized declared paths/globs, harness-derived diff, scope verdict; explicit test paths | 06a, Domain 05 report |
| W3 | `06d-role-authority-packets-and-tool-policy` | machine-readable authority packet, role→mode map, default-deny tools at dispatch/MCP | 06b, Domain 05 mission |
| W4 | `06e-context-and-change-boundary-scan` | scan `.specd/` runtime tree + untracked pending set; fail-closed enumeration; trust labels | 06b, Domain 02 context |
| W5 | `06f-mandatory-sandbox-and-secret-isolation` | production-required sandbox, synthetic HOME, minimal env, redaction, resource limits | 06b, Domain 04 verify |
| W6 | `06g-dependency-and-dangerous-change-governance` | manifest-diff + external evidence adapters; dangerous-command/authz/generated/symlink policies | 06c,06e |
| W7 | `06h-exceptions-ledger-and-mission-audit` | governed append-only exceptions; unified policy-digest mission view | 06b,06g, Domain 07 export |
| W8 | `06i-cross-platform-adapters-and-regression-governance` | sandbox capability negotiation; incident→regression corpus; release proof | 06f,06h, Domain 10 adapter |

## DAG

```text
06a → 06b ─┬─> 06d ─┐
           ├─> 06e ─┼─> 06g ─> 06h ─> 06i
           └─> 06f ─┘          ↑        ↑
06a → 06c ────────────> 06g    │        │
Domain 05 mission/report ─> 06c,06d     │
Domain 02 context ─> 06e                │
Domain 04 verify ─> 06f                 │
Domain 07 export ─> 06h ────────────────┘
Domain 10 adapter ─> 06i
```

## Program rules

1. No LLM in any gate, scope, sandbox, or exception decision. Pure functions of `.specd/` state.
2. No bypass flag for evidence or scope. Exceptions never waive evidence integrity or silently
   broaden worker authority.
3. Production is default-deny: unknown tool, missing sandbox, stale security = fail closed, never
   silent downgrade. Prototype may warn/confirm; profile is explicit, invalid profile fails closed.
4. Worker-reported paths/digests are audit hints, never authority. Scope is harness-derived from a
   pinned baseline; tests must be declared explicitly (no implicit "plus tests").
5. Scanner inability (git enumeration or file read failure) is an error finding in production,
   never an empty successful scan.
6. Secret references only — never values — in prompts, evidence, exception reasons, tool args,
   reports, or terminal output. Central redaction before display.
7. One resolved `policy_version`/`policy_digest` pinned to dispatch, trace, evidence, report.
8. Stdlib-only, offline core. External adapters produce pinned artifacts; deterministic core
   validates them. No `reference/` edits.

## Completion claim

Fresh fixture selecting the production profile: an out-of-scope tracked/untracked/rename/symlink
change fails completion even when tests pass and the worker reports only the declared file; a
scout write through MCP is denied and logged; an injection marker under `.specd/specs/<slug>/`
is reported pre-dispatch and stays labelled untrusted; an untracked credential fails completion;
a failed `git ls-files` yields an error finding, not a green scan; a verify command reading
`$HOME/.aws/credentials` over a blocked network fails without leaking output; a missing sandbox
binary fails closed before shell; a misspelled/unapproved dependency fails with a provenance
requirement; an expired/approverless allowlist entry suppresses nothing; production submit
without a current security/scope attestation refuses with an exact next command. Docs never
claim a boundary the harness does not enforce.
