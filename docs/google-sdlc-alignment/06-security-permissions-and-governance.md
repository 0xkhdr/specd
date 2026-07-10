# Domain 06 — Security, Permissions, and Governance

## Purpose

Define the production security domain for a coding agent that drives `specd` through every lifecycle phase. The desired outcome is least-authority tool use, enforceable task scope, safe context ingestion, isolated command execution, dependency and secret controls, and accountable exceptions—without placing an LLM in a trust decision or breaking the existing requirements-to-evidence flow.

This is a hardening plan, not a claim that prompt-level roles already provide an OS security boundary. Recommendations preserve deterministic gates and the Go standard-library-only runtime.

## Paper position

The paper treats security as a property of the **harness**, not model good behavior:

- Harness components include instructions, tools, context policies, hooks, sandboxes, sub-agents, and observability (`sdlc-paper.md:258-281`). Instructions state prohibitions; tool definitions and execution controls determine actual capability.
- Guardrails constrain agents to safe, predictable behavior in the factory model (`sdlc-paper.md:242-254`). Deterministic hooks can block unsafe actions such as hard-coded secrets before they enter the codebase (`sdlc-paper.md:274-281`).
- Production agents need scoped permissions on tools and data, eval coverage, and traces of what they actually did (`sdlc-paper.md:380-390`, `492-494`).
- Background agents commonly operate in sandboxes with repository/build/test access (`sdlc-paper.md:341-374`). Production practice extends that idea to guardrails, sandboxing, and zero-trust development (`sdlc-paper.md:416`).
- AI-generated code requires heightened review for hallucinated dependencies, inadequate error handling, subtle correctness, and security vulnerabilities (`sdlc-paper.md:228-234`, `482-484`).
- The paper warns that rapid generation without automated evaluation produces rapid vulnerability generation and costly remediation (`sdlc-paper.md:426-442`).

The paper does not prescribe a particular permission schema. Its requirement is architectural: authority must be scoped and observable, risky actions must be constrained by the harness, and production security cannot depend on the agent remembering prose.

## Current `specd` handling with evidence paths

| Capability | Current handling | Evidence |
|---|---|---|
| Role authority intent | Embedded role constitutions define scout, craftsman, validator, and auditor behavior. A task must declare a role. | `internal/core/embed_templates/roles/*.md`; `internal/core/roles.go`; `internal/core/gates/core.go` |
| Driver lifecycle | Scaffolded `AGENTS.md` teaches status/context/work/verify/check/approve and tells agents to stay within declared files. Phase dispatch fails closed for out-of-phase verbs. | `internal/core/embed_templates/AGENTS.md`; `internal/cmd/dispatch.go`; `docs/agent-integration.md` |
| Brain authority | Brain cannot dispatch unless explicitly started with authority; high/critical gates are not clearable by its authority model. | `internal/cmd/brain_run.go`; `internal/orchestration/authority.go`; `internal/orchestration/decide.go` |
| File declarations | The core `files` gate requires non-empty task file text; craftsman/auditor prompts refer to declared scope. | `internal/core/gates/core.go` (`files`); `internal/core/tasksparser.go`; role templates |
| Security scan | `specd check --security` runs deterministic secrets, prompt-injection, and Go dependency-typosquat scanners over selected git-tracked files. Per-scanner severity is `off|warn|error`. | `internal/cmd/registry.go` (`runCheck`); `internal/core/gates/security/`; `internal/core/config_loader.go` |
| Security exceptions | Fingerprint allowlist entries require a non-empty reason; malformed entries fail closed. Findings, including allowlisted findings, can be recorded for spec reports. | `internal/core/gates/security/allowlist.go`; `internal/cmd/registry.go` (`recordSecurity`); `SECURITY.md` |
| Verify environment | Verify subprocess receives only `HOME`, `PATH`, and `TMPDIR` from the parent environment. | `internal/core/verify/exec.go` (`scrubbedEnv`); `internal/core/verify/sandbox_test.go` |
| Verify sandbox | `--sandbox` uses a `bwrap`-compatible wrapper: namespaces unshared, network unavailable, root read-only, private `/tmp`, repo writable. Missing sandbox binary fails closed. | `internal/core/verify/exec.go` (`wrapArgv`); `internal/core/verify/sandbox_test.go`; `docs/troubleshooting.md` |
| Failure recovery | `--revert-on-fail` restores the pre-run tracked diff after failed verification. | `internal/cmd/registry.go` (`withRevertOnFail`); `internal/cmd/verify_test.go` |
| Evidence trust | Completion requires passing evidence pinned to a non-unknown HEAD; orchestrated reports also require matching task/worker/lease/HEAD. | `internal/core/task_complete.go`; `internal/cmd/brain_worker.go` |
| Minimal dependency surface | The binary uses the standard library only; scanners are embedded and do not call an LLM or network. | `go.mod`; `internal/core/gates/security/scanner.go`; `SECURITY.md` |

These mechanisms establish useful guardrails, but several are advisory or opt-in. The phrase in `docs/agent-integration.md`—“convention + gates”—is accurate: roles do not currently create tool capabilities, and declared files are not compared with the actual diff.

## Common contract and fields

| Contract field | Paper/harness purpose | `specd` today | Target contract |
|---|---|---|---|
| `actor_id`, `worker_id`, `session_id`, `mission_id` | Attribute every action | Partial ACP/lease identity | Required, authenticated or host-attested identity on tool/evidence events |
| `spec_id`, `task_id`, `phase`, `role` | Bind authority to current mission | Present across state/tasks/dispatch | Mandatory on authority packet; stale phase/task packets rejected |
| `authority_profile` | Separate prototype and production posture | Brain has boolean dispatch authority | Versioned profile resolving tool, path, network, secret, and approval policies |
| `allowed_tools`, `denied_tools` | Least-capability tool use | Role prose only | Machine-readable tool ids and argument constraints; default deny in production |
| `declared_paths` | Restrict write surface | Non-empty free text | Normalized repo-relative paths/globs, explicit test paths, no implicit “plus tests” |
| `baseline_revision`, `baseline_diff_digest` | Know what the agent changed | HEAD appears in evidence | Immutable task baseline plus final tracked/untracked/symlink-aware diff |
| `changed_paths`, `change_digest` | Enforce actual scope | Worker-reported ACP field only | Harness-derived path set/digest, never trusted solely from worker report |
| `filesystem_policy` | Read/write boundaries | Verify sandbox only | Read roots, write roots, temp policy, symlink policy, max bytes/files |
| `network_policy` | Prevent exfiltration/supply-chain actions | Sandbox disables network; default verify does not | Default-deny production, with brokered host/port/method exceptions |
| `environment_policy` | Prevent inherited-secret exposure | Allowlist retains `HOME/PATH/TMPDIR` | Empty/synthetic HOME, minimal PATH, explicit non-secret variables, output redaction |
| `secret_policy` | Avoid source/log/context leaks | Opt-in tracked-file scanner; scanner excerpts redacted | Scan changed tracked/untracked artifacts and outputs; secret references, never values |
| `dependency_policy` | Prevent hallucinated/malicious packages | Go-only typosquat heuristic | Manifest diff, registry/source allowlist, lock/checksum/provenance and vulnerability adapter evidence |
| `context_trust` | Resist prompt injection | Opt-in heuristic on selected tracked text | Trust label and digest per context item; scan runtime specs/steering and tool results before display |
| `sandbox_required`, `sandbox_profile` | Isolate arbitrary commands | CLI flag; policy marker check exists | Production default with capability declaration, resource limits, and fail-closed availability |
| `approval_required`, `approver` | Preserve human boundary | Lifecycle approvals; override reason | Risk/action-specific approval with actor, scope, revision, and policy id |
| `exception_id`, `reason`, `ticket`, `expires_at` | Accountable waiver | Fingerprint + reason | Scoped, time-bounded, approved, append-only exception; status visible in reports |
| `event_seq`, `tool_id`, `args_digest`, `result_class` | Audit tool trajectory | Partial ACP events | Ordered sanitized action ledger; no secrets or hidden reasoning |
| `policy_version`, `policy_digest` | Reproduce decision | Config values, no unified digest | Pin the exact resolved policy to dispatch, trace, evidence, and report |

## Gaps and failure modes

1. **Role prompts are not capability enforcement.** The CLI checks only that the role string is non-empty, not that it is one of the four documented roles. `RolePrompt` falls back to craftsman for an unknown role, and `ModeForTask` currently falls through to craftsman for auditor, so malformed or incompletely mapped role metadata can grant the driver a misleading write-mode signal. Direct CLI/tool/filesystem access is not authorized per role either; an externally hosted scout can still write if its host grants write tools.
2. **Declared-file scope is not enforced.** The `files` gate checks only non-empty text. `ChangedFiles` is a worker claim and `acceptWorkerReport` does not compare it to task declarations or a harness-derived diff. The craftsman phrase “plus their tests” also creates an undefined scope expansion.
3. **Security gates are opt-in.** `CoreRegistry()` excludes security; `runCheck` invokes it only with `--security`, and `submit` runs the core registry without it. Default severities therefore do nothing unless the caller remembers the flag. This is unsafe for an autonomous production driver.
4. **The prompt-injection scan misses the highest-value runtime tree.** `trackedFiles` excludes `.specd/`, while managed requirements, design, tasks, steering, roles, and memory live there and are loaded into agent context. The current threat-model prose overstates the effective coverage for runtime spec content.
5. **File enumeration can fail open.** A failed `git ls-files` returns an empty scan set, and unreadable tracked files are silently skipped. A production scan should emit an error finding when its intended boundary cannot be inspected.
6. **Untracked/staged/generated artifacts can escape scanning.** The scan is based on git-tracked working-tree paths, not the complete task diff or pending output set. New untracked files and ignored generated artifacts may carry secrets or injections.
7. **Arbitrary shell is unsandboxed by default.** `verify:` runs `/bin/sh -c` unless the driver supplies `--sandbox`. The sandbox requirement config currently checks an environment marker during security analysis; it does not itself wrap every command.
8. **The sandbox protects writes/network better than reads.** `--ro-bind / /` makes the host filesystem read-only but readable. Keeping real `HOME` can expose file-based credentials; scrubbed environment variables do not stop a command from reading known credential paths. Verify output is truncated but not generally secret-redacted before printing.
9. **Sandbox scope is narrow.** It covers verify/submit execution paths, not the external coding agent's edit tools, repository reads, MCP calls, package installation, or other host tools. `bwrap` is also Linux-specific.
10. **Dependency protection is incomplete.** Slopsquat scans only `go.mod` and compares against an embedded popular-Go list. It does not verify registry provenance, new dependency necessity, checksums, lockfile changes, known vulnerabilities, install scripts, or non-Go ecosystems. Lockfiles are intentionally excluded from all security scanners.
11. **Prompt-injection detection is heuristic.** Phrase/zero-width rules can miss indirect instructions, malicious code comments, poisoned tool output, or context-confusion attacks, and can also warn on legitimate security documentation. Scanning cannot replace trust labels and authority separation.
12. **Allowlist governance is too thin for production.** A reasoned fingerprint is good, but there is no approver, ticket, owner, issue date, expiry, environment, task/revision scope, or revocation lifecycle. Repo-wide security-only scans are not attached to a spec record.
13. **No unified audit of policy decisions.** Lifecycle approvals, security allowlists, task overrides, ACP events, and evidence exist in different records without one policy version/digest or end-to-end mission view.
14. **No resource quotas.** Verify timeout is opt-in/default-unbounded, and sandbox arguments do not express CPU, memory, process, output, or filesystem-growth limits. A hostile command can cause denial of service.

## Target best-practice workflow

1. **Select an explicit operating profile.** `prototype` can warn and require confirmation; `production` makes baseline security, scope, evidence, review, and sandbox policies mandatory. The profile is resolved before the agent receives a mission.
2. **Issue a versioned authority packet.** Bind actor/worker, spec, task, phase, role, allowed tools, argument constraints, declared read/write paths, network policy, sandbox profile, baseline revision, expiry, and policy digest.
3. **Build context through a trust boundary.** Validate path/digest, classify each item as trusted instruction or untrusted data, scan the actual runtime spec/steering/memory plus retrieved tool results, and render injection warnings without elevating untrusted text to instructions.
4. **Launch the worker with least capability.** Scout/validator/auditor are read-only at the host boundary; craftsman writes only normalized declared paths. When `specd` cannot control the external host, require a host attestation and always verify the resulting diff independently.
5. **Broker risky tools.** Network, package install, credentials, deployment, and destructive commands go through named tools with constrained arguments and explicit approval policy. Raw shell is not a universal production capability.
6. **Use isolated execution by default.** Production verify runs in a required sandbox with network off, synthetic/empty HOME, minimal environment, private temp, bounded resources, controlled writable paths, and fail-closed adapter discovery. Platform-specific sandboxing remains an external adapter contract where necessary.
7. **Derive scope from reality.** At completion, compute tracked, staged, untracked, deletion, rename, mode, and symlink changes from the task baseline. Normalize paths and compare them to declarations; tests must be declared explicitly. Worker claims are audit hints, not authority.
8. **Scan the pending change set.** Apply secrets, injection, dependency, permission, dangerous-command, and generated-artifact policies to all outputs the task can ship. Scanner inability is an error in production.
9. **Verify and review.** Tests remain non-bypass. Security/eval artifacts pin the same revision and policy digest. Auditor review focuses on dependency provenance, authz changes, input handling, secret flows, error behavior, and scope deviations.
10. **Govern exceptions.** A human-approved exception states exact finding/action, reason, ticket, owner, scope, revision/environment, issue/expiry, and compensating control. It is append-only, visible in reports, and cannot waive evidence integrity or silently broaden the worker's authority.
11. **Submit only from a green production profile.** Core, required security, scope, eval, and review gates all pass at the same subject revision. Submit/deploy transport receives only the minimal scoped credentials required for that terminal action.
12. **Audit and refine.** Preserve sanitized observable tool events, policy decisions, exceptions, and evidence references. Turn security incidents or near misses into deterministic regression rules without adding verbose history to every agent context.

## Recommended action plan

| Priority | Recommended change | Code/artifact surface | Deterministic acceptance checks |
|---|---|---|---|
| **P0** | Add production/prototype security profiles and make required security gates part of completion/submit for production. | `internal/core/config_loader.go`; `internal/core/gates/`; `internal/cmd/registry.go`; `internal/cmd/submit.go`; `project.yml`; docs | Production submit fails when security was not run/current; prototype behavior is explicit; same inputs yield stable findings; invalid profile fails closed. |
| **P0** | Enforce declared-file scope against a harness-derived task diff from a pinned baseline. Replace free-form ambiguity with normalized paths/globs; require tests to be declared. | `internal/core/tasksparser.go`; new scope/diff core package; task completion and worker report paths; role templates | Outside-scope tracked/untracked/delete/rename/mode/symlink changes fail; `../`, absolute paths, and symlink escapes fail; worker-reported paths cannot override derived paths; byte-stable parser tests remain green. |
| **P0** | Introduce machine-readable role authority packets and role-aware tool policy at dispatch/MCP boundaries. Remove role fallback ambiguity. | `internal/core/gates/core.go`; `internal/core/roles.go`; `internal/context/manifest.go`; `internal/cmd/dispatch.go`; `internal/mcp/`; `internal/orchestration/authority.go`; role templates | Unknown roles fail the role gate; each documented role maps to its exact mode; scout/validator/auditor write calls are denied; craftsman writes outside declared paths are denied; stale/expired/wrong-phase packets fail; unknown tools default deny in production. |
| **P0** | Scan the real context and change boundary. Include runtime `.specd/specs`, steering, role/memory inputs, and task-created untracked files; fail on enumeration/read errors. | `internal/core/gates/security/gate.go`; scanner input abstraction; context builder; security tests/fixtures | Injection fixture under runtime spec/steering is found before dispatch; untracked secret is found; git/read failure yields error finding; exclusions are explicit per scanner rather than blanket trust. |
| **P0** | Make production verify sandboxing mandatory and harden secret isolation/output handling. | `internal/core/verify/exec.go`; config/CLI resolution; platform sandbox adapter contract; evidence/output redaction | Missing required sandbox fails before shell execution; network probe fails; host credential path is unavailable; only repo/temp writes succeed; secret fixture never appears in stdout/stderr/evidence; timeout/resource breach records failure. |
| **P1** | Expand dependency governance beyond Go-name heuristics using manifest-diff and external evidence adapters while keeping the binary dependency-free. | Security scanner interfaces; project policy; dependency evidence JSON; CI scripts/adapters | New dependency requires declared reason/source; unknown registry/checksum/provenance fails per profile; malformed/stale vulnerability artifact fails; gate remains offline and stdlib-only. |
| **P1** | Add dangerous-command, permission/authz-change, generated-file, and symlink policies over normalized diffs and traces. | Security policy package; trajectory imports; review template | Stable fixtures detect destructive shell, world-writable/executable changes, auth policy changes, generated secret files, and path escapes with documented false-positive controls. |
| **P1** | Replace bare allowlist entries with governed exceptions and an append-only decision ledger. | `.specd/security/exceptions.jsonl`; core schema; approve/revoke commands; report projections | Missing approver/reason/ticket/scope/expiry fails; expired/revoked/wrong-revision exception suppresses nothing; edits change digest; reports show active and historical exceptions. |
| **P1** | Add a unified sanitized mission audit view keyed by run/mission/task and policy digest. | ACP/evidence/security/report models; `specd report` | Ordered events correlate authority, tools, diff, scans, verify, review, and submit; secrets/raw sensitive arguments are absent; duplicate/out-of-order identifiers fail import. |
| **P2** | Add cross-platform sandbox capability negotiation and organization policy adapters. | Adapter protocol and conformance suite; no runtime library dependency | Linux/macOS/CI adapters declare capabilities; production refuses an adapter missing required capabilities; conformance fixtures produce equivalent policy outcomes. |
| **P2** | Feed incidents and policy drift into regression governance. | Security regression corpus, policy version reports, maintenance specs | Every promoted incident fixture has redacted provenance and expected finding; policy changes invalidate stale attestations; deterministic trend reports require no model. |

## Production validation scenarios

| Scenario | Expected result |
|---|---|
| Craftsman edits its declared source file and an undeclared CI workflow | Scope gate derives both paths and refuses completion, even if tests pass and worker reports only the source file. |
| Scout attempts a write through MCP or a hosted tool | Role-aware boundary denies the call and records a sanitized denial event. |
| Task context contains an injection marker in `.specd/specs/<slug>/requirements.md` | Pre-dispatch context scan reports it under the configured policy; content remains labelled untrusted and cannot alter gate behavior. |
| A new untracked file contains a credential | Pending-change secret scan detects and redacts it; production completion fails. |
| `git ls-files` or a required file read fails | Security analysis returns an error finding rather than an empty successful scan. |
| Verify command tries network access and reads `$HOME/.aws/credentials` | Required production sandbox blocks network and does not expose the real home/credential path; attempt fails without leaking output. |
| Sandbox binary/adapter is missing | Production verify fails closed before raw shell starts; no silent fallback. |
| Agent adds a misspelled Go module or an unapproved package registry dependency | Slopsquat/manifest-diff policy fails and records exact provenance requirement. |
| Agent changes a lockfile only | Dependency-diff policy inspects the change rather than excluding it globally; unexpected transitive changes are reported. |
| Allowlist entry has a reason but is expired or lacks approver/ticket | It suppresses nothing and produces a governance error. |
| Same finding is intentionally accepted for one task/revision | Scoped exception suppresses only that exact policy/fingerprint/scope until expiry; later revisions re-surface it. |
| Security scanner prints a candidate secret or verify echoes one | Central redaction prevents the full value from entering terminal output, evidence, reports, or context. |
| Production submit runs without a current security/scope attestation | Submit refuses; the coding agent receives the exact next command/artifact required, not a vague remediation prompt. |

## Context-safety considerations

- The driver needs a short immutable authority summary in every task packet: role, allowed tools, declared paths, network/sandbox policy, required gates, baseline revision, expiry, and policy digest. It does not need the full security manual.
- Treat requirements, design, tasks, memory, retrieved docs, code comments, issue text, test fixtures, and tool output as potentially untrusted data. Only embedded/versioned role and policy material should be rendered as instructions.
- Scan and label context **before** it reaches the model. Scanner findings should point to exact references and safe excerpts; never inline a secret or the full malicious payload to explain a hit.
- Use progressive disclosure: show active constraints and current findings; keep scanner rule catalogs, old traces, exception history, and dependency databases behind tools/references.
- Tool descriptions must make the safe path native: phase-valid commands, required arguments, effect class, approval need, and likely remediation. Avoid exposing deferred or forbidden operations as if they were available choices.
- Return structured, bounded failures such as `outside_scope`, `sandbox_unavailable`, or `security_evidence_stale`, each with one deterministic next action. This reduces model retry loops and prevents the agent from improvising a bypass.
- Never place credentials in prompts, manifests, evidence text, exception reasons, or tool arguments. Use opaque secret references resolved only by the narrowly scoped terminal tool.
- Prompt-injection scanners are signals, not instruction parsers. Security comes from keeping untrusted content unable to grant authority, not from assuming every malicious phrase can be recognized.
- Audit observable actions and policy decisions only. Do not request or store hidden chain-of-thought.

## Non-goals and risks

- `specd` cannot, by itself, sandbox an arbitrary external coding-agent host. It should enforce boundaries on processes/tools it owns, verify resulting artifacts and diffs, and define a conformance/attestation contract for hosts.
- The project should not add cloud security SDKs, vulnerability databases, or sandbox libraries as runtime dependencies. External adapters may produce pinned artifacts; deterministic core code validates them.
- Security scanning is not proof of absence. Entropy and injection heuristics have false positives and false negatives; dependency-name distance is not supply-chain verification.
- A read-only root is not a confidential root. Sandbox design must explicitly hide secrets and sensitive host paths, not rely on filesystem write protection.
- Default-deny production policy may reduce portability or break legacy workflows. Use explicit profiles and migration diagnostics, never a silent downgrade.
- Exceptions are necessary operational controls but can become a bypass culture. Make them narrow, expiring, reviewable, and unable to waive core evidence integrity.
- Full raw command arguments, outputs, traces, and diffs can contain secrets or personal data. Audit usefulness must be balanced with redaction, retention, and access control.
- Scope enforcement must handle generated files, renames, symlinks, submodules, nested repositories, and platform path rules consistently; naïve prefix matching is unsafe.
- Do not turn every warning into static context. Overloaded security prose dilutes the task and can misguide the model; compact authority packets plus on-demand details are safer.
