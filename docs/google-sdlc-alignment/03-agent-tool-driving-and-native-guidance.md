# Domain 03 — Agent–Tool Driving and Native Guidance

## Purpose

Define the contract that lets a coding agent discover, understand, and drive `specd` correctly at every lifecycle phase without relying on remembered commands, loading the whole repository, bypassing a human boundary, or confusing a model claim with harness evidence.

This is a production-critical domain. A correct state machine is not enough if the agent receives the wrong artifact path, cannot identify the active spec, or cannot determine the one safe next action.

## Paper position

The comparison document maps this domain to the paper's formula **Agent = Model + Harness**, its conductor/orchestrator distinction, and its treatment of tools, instructions, knowledge, memory, and guardrails as separate context types. The paper's implied production contract is:

1. the model supplies situation-dependent reasoning;
2. the harness exposes a legible tool and state surface;
3. context is selected for the current action;
4. permissions and deterministic hooks constrain execution; and
5. the human retains explicit judgment boundaries.

The paper does not prescribe a `specd` command sequence. The command-level contract below is therefore an implementation inference from those principles, not a direct paper requirement.

## Current `specd` handling

### Strong foundations

- `internal/core/embed_templates/AGENTS.md` is installed as a managed region by `internal/core/scaffold.go`. It teaches the core loop, role boundaries, declared-file scope, evidence integrity, and stop-on-block behavior.
- `internal/core/commands.go` is the canonical command palette for help, dispatch metadata, phase compatibility, examples, flags, and exit-code meaning.
- `internal/cmd/dispatch.go` is a single fail-closed dispatch choke point. It validates command existence, flag enums, slugs, and allowed phases before handler side effects.
- `internal/mcp` derives its tool surface from the palette rather than maintaining an unrelated list. `docs/mcp-guide.md` documents tool errors and the intentional boundary between MCP-callable operations and CLI/human operations.
- `internal/core/handshake.go` exposes stable palette and effective-config digests, allowing an agent or CI job to reject a stale integration.
- `internal/context/manifest.go` returns a deterministic, budgeted manifest and drops optional memory before constitutional steering.
- `internal/integration/snippet.go` supplies a short host-neutral work instruction: load task context, stay within declared files, then verify.
- Role-specific files under `internal/core/embed_templates/roles/` constrain capability and define a structured result envelope.

### Production guidance defects found during this analysis

These are observable repository mismatches, not paper-level speculation:

1. **Core context paths do not match the runtime layout.** `internal/context/manifest.go` emits `specs/<slug>/requirements.md` and `specs/<slug>/tasks.md`; the runtime contract in `AGENTS.md`, `docs/concepts.md`, and `core.SpecdDir` is `.specd/specs/<slug>/...`.
2. **Declared task files are absent from the context manifest.** `BuildManifest` creates spec, task-table, task-id, role, steering, and memory items, but does not add `TaskRow.Files`. This conflicts with the documented claim that the manifest contains the files a task needs and leaves the host to rediscover task scope.
3. **Design context is absent.** A task can be asked to implement an architectural constraint recorded only in `design.md`, yet the core manifest does not reference that artifact.
4. **Some generated loop commands are not executable as written.** The managed `AGENTS.md` says `specd status` and `specd verify` without required operands. `runStatus` requires a spec unless `--program` is used, and task verification requires both slug and task id.
5. **Pinned MCP spec state is emitted but not consumed.** `internal/core/mcpconfig.go` writes `SPECD_SPEC`; repository search finds no runtime reader for it. An MCP host may believe a spec is active while every relevant tool still needs an explicit slug.
6. **The handshake detects palette/config drift, not guidance drift.** A cached role or `AGENTS.md` can disagree with the running binary even when an integration neglects `init --refresh`; the handshake does not expose the managed-template digest or active-spec resolution.
7. **Next-action guidance is distributed.** Phase metadata, gate findings, status, frontier, role prompts, and troubleshooting are individually useful, but there is no single machine-readable response that says: current phase, blocking facts, permitted next actions, required human action, and exact context command.

Until the first five defects are fixed and covered by black-box tests, `specd context` and the generated guidance should not be described as production-proven for an agent with no prior `specd` knowledge.

## Common contract and fields

| Field | Paper-side purpose | `specd` source today | Required production meaning |
|---|---|---|---|
| `protocol_version` | Stable harness contract | help schema, handshake version, manifest version | Version every machine-readable envelope independently and reject incompatible major versions. |
| `root` | Tool/environment grounding | process cwd; optional MCP `cwd` | Canonical project root used for every returned path. |
| `spec_slug` | Current unit of intent | CLI operand; state path | Explicit in every task-oriented response; never inferred ambiguously when multiple specs exist. |
| `phase` / `status` | Lifecycle position | `state.json`, `status` | Current phase plus allowed and blocked transitions. |
| `task_id` | Atomic work identity | frontier/task row | Stable id attached to context, lease, evidence, and completion. |
| `role` / `authority` | Permission boundary | task row, role prompt, Brain authority | Allowed read/write/tool actions and whether human authority is required. |
| `declared_files` | Dynamic work scope | `TaskRow.Files` | Canonical existing or intended paths, included in the context packet and checked after work. |
| `acceptance` | Output intent | task row | Testable success conditions, preserved verbatim with source citation. |
| `verify` | Deterministic check | task row | Exact command, timeout/sandbox policy, and evidence destination. |
| `context_items` | Relevant instructions/knowledge | context manifest | Existing canonical paths with kind, mode, reason, priority, digest, estimated tokens, and required/optional status. |
| `next_actions` | Reduce driver ambiguity | currently distributed | Ordered, phase-valid commands with actor (`agent` or `human`) and side-effect class. |
| `blockers` | Safe stop/recovery | findings and error strings | Stable code, explanation, affected artifact, and recovery action; no prose parsing required. |
| `palette_digest` / `config_digest` / `guidance_digest` | Detect stale integration | first two exist | Pin binary tools, effective policy, and installed managed guidance as one bootstrap compatibility set. |
| `evidence_ref` | Separate claims from proof | evidence files/records | Immutable reference to the current-HEAD verification/eval record. |

## Gaps and failure modes

- A capable agent follows a returned `specs/...` reference, reads the wrong top-level planning file or nothing, and implements from incomplete intent.
- A host treats manifest references as the complete task context even though task files and design constraints are missing.
- A newly initialized agent copies the slugless examples, receives a usage failure, and improvises around the harness instead of receiving deterministic recovery guidance.
- Multiple specs exist and an MCP host's nominal `SPECD_SPEC` pin has no effect; actions can be issued with the wrong explicit slug.
- MCP refuses an authority-sensitive action correctly, but the error does not supply a structured handoff telling the model which human/CLI action is required.
- A binary upgrade changes commands while installed prompts remain old. Palette drift can be detected only if the host saved and checks the prior digest; managed-template drift remains separate.
- Context budget accounting estimates referenced content, but returned items lack a selection reason and integrity digest. A host cannot distinguish required constitutional context from an optional hint without hard-coded knowledge.
- Text errors are actionable for humans but fragile for autonomous drivers that need stable blocker classes and retry rules.

## Target best-practice workflow

1. **Bootstrap:** the agent calls one read-only bootstrap operation. The response identifies root, installed guidance version/digest, binary palette/config digests, known specs, active-spec resolution, and compatibility findings.
2. **Orient:** the agent requests a driver view for a slug. It receives phase, approvals, frontier, blockers, and exact next actions tagged by required actor and authority.
3. **Plan-phase work:** the agent receives only the artifact template, applicable steering, phase-specific instructions, and validation findings. Approval remains a human action.
4. **Task claim:** the agent selects or receives a frontier task. The harness emits a self-contained dispatch envelope with task row, canonical `.specd` artifact paths, declared source files, design references, role, limits, and digests.
5. **Execute:** the agent edits only declared files. A local scope check compares the actual git diff to the declared set; a deviation requires a recorded decision and re-authorization.
6. **Verify:** the agent invokes the exact harness command from the envelope. Results return stable status/blocker codes and immutable evidence references.
7. **Complete or stop:** completion is a distinct gated action; on failure the driver receives retry allowance, escalation state, and the one safe recovery action. It never infers completion from model prose.
8. **Refresh:** any palette/config/guidance digest change forces re-bootstrap before additional mutation.

## Recommended action plan

### P0 — Make the existing contract truthful and executable

1. Correct core context paths to `.specd/specs/<slug>/...` in `internal/context/manifest.go` and add path-existence assertions in context tests. **Acceptance:** a fresh black-box `init → new → context --json` run returns only paths resolvable beneath the project root.
2. Add declared task files and `design.md` to the manifest with required/optional semantics. **Acceptance:** every file declared by a task appears exactly once; design is present for implementation/audit roles; deterministic ordering holds across repeated runs.
3. Replace slugless managed instructions with exact operand-bearing templates or documented placeholders in `internal/core/embed_templates/AGENTS.md` and role files. **Acceptance:** extract every fenced/example `specd` invocation from freshly scaffolded guidance and execute it against a fixture, allowing only explicitly marked placeholders.
4. Either implement `SPECD_SPEC` resolution consistently at the command boundary or remove it from MCP config. Explicit args must take precedence. **Acceptance:** pinned-single-spec, explicit-override, missing-spec, and multi-spec ambiguity cases have black-box tests.
5. Add `specd doctor --agent --json` or equivalent read-only conformance check for binary, runtime paths, managed regions, host artifacts, config, and context resolvability. **Acceptance:** each defect above produces a stable failing finding and no writes.

### P1 — Give the driver a deterministic next-action API

1. Add a versioned `driver`/`guide` JSON envelope backed by the existing command palette, gate registry, state, and frontier. Likely surfaces: `internal/core/commands.go`, a pure core projection, and a thin `internal/cmd` renderer. **Acceptance:** every lifecycle phase returns at least one valid action or a typed terminal/blocking reason; suggested commands round-trip through argument parsing.
2. Add `actor`, `side_effect`, `authority_required`, `allowed_phases`, and stable error codes to machine guidance. **Acceptance:** approval and policy-exception actions can never be mislabeled as agent-authorized.
3. Extend handshake output with managed-guidance and context-schema digests. **Acceptance:** changing a role, steering template, or manifest schema changes the appropriate digest without changing unrelated digests.
4. Return canonical path, selection reason, priority, content digest, and required/optional status for each context item. **Acceptance:** the same on-disk inputs produce byte-stable JSON; missing required items fail closed.
5. Emit a structured CLI handoff when MCP policy refuses a mutation. **Acceptance:** the response names the required CLI command and actor but never performs it.

### P2 — Prove host-neutral native operation

1. Publish a driver conformance suite for raw CLI, MCP, Codex, Claude, and Pinky integrations. **Acceptance:** the same lifecycle fixture produces equivalent phase/task/evidence outcomes across hosts.
2. Add capability negotiation so external hosts can declare context loading, sandbox, cost telemetry, eval, and A2A support. **Acceptance:** unsupported capabilities yield deterministic downgrade/refusal policy, never silent omission.
3. Add signed or checksum-pinned dispatch envelopes for remote workers. **Acceptance:** task, role, scope, configuration, or context drift invalidates a stale claim/report.
4. Generate host instructions from palette metadata plus small reviewed templates where feasible. **Acceptance:** command/flag examples cannot drift from the parser and documentation lint covers the generated integration guide.

## Production validation scenarios

| Scenario | Expected result |
|---|---|
| Fresh machine, fresh repo, no model memory of `specd` | Bootstrap and generated guidance lead from `init` to the first human approval without an undocumented command. |
| One active spec | Driver view resolves the slug explicitly and returns the correct phase action. |
| Multiple active specs | No implicit selection; the agent must choose from a deterministic list. |
| Context for a write task | Manifest includes requirements, design, task row, role, applicable steering, and every declared file using resolvable canonical paths. |
| Budget pressure | Optional examples/memory shed before required role, task, design, and declared-file contracts; omissions are reported. |
| Wrong-phase command | No mutation; stable blocker code and valid recovery action are returned. |
| MCP authority boundary | Read-only call works; forbidden mutation yields a structured human/CLI handoff. |
| Binary or project-policy upgrade | Digest mismatch blocks mutation until bootstrap/refresh; user-authored content is preserved. |
| Verify failure and retry exhaustion | Evidence records the failure; the agent follows bounded retry/escalation policy and stops. |
| Worker reports undeclared edits or stale HEAD | Completion is rejected with a typed scope or evidence mismatch. |

## Context-safety considerations

- The driver envelope should contain decisions and references, not inline the repository. Content remains loaded progressively by kind and phase.
- Requirements, design, task row, role, and declared files are correctness-critical; optional memory and examples may be dropped. The current policy of dropping memory before steering is sound but incomplete without task/design/file inclusion.
- Include why an item was selected so the model can avoid opening every reference defensively.
- Store digests and byte/token estimates, not duplicated content, in ledgers and handshakes.
- Never inject raw tool output or repository text into constitutional instructions; keep data, instructions, and untrusted content typed separately.
- Stable error codes reduce the need to feed long troubleshooting manuals into every agent turn.

## Non-goals and risks

- Native guidance does not mean giving the agent approval authority. Human gates and explicit orchestration authority remain boundaries.
- `specd` should not become a prompt-heavy autonomous model loop. It should expose deterministic state and contracts that any host can reason over.
- An active-spec default is dangerous when ambiguous; convenience must fail closed in multi-spec repositories.
- A richer manifest can itself cause context bloat. Required/optional tiers, progressive disclosure, and hard budgets are necessary.
- Digests prove sameness, not correctness. The conformance suite must validate semantic behavior as well as hash drift.
