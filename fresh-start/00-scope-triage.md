# 00 — Scope Triage

Classification of **every** current `specd` command surface into
KEEP / SIMPLIFY / REDESIGN / DEFER / CUT, each tied to a principle (P1–P8) or a
concrete use case. Source of truth for the command list is
`internal/cmd/registry.go` (29 registered commands); hidden subsurfaces
(flags/subcommands that live in their own files) are listed separately so they
are not lost.

**Verdict legend**
- **KEEP** — core, ships in v1 substantially as-is.
- **SIMPLIFY** — ships in v1 but with a smaller surface / less code.
- **REDESIGN** — ships in v1 but the internal architecture changes materially.
- **DEFER** — not in the v1 MVP; belongs to a later wave or a plugin/opt-in tier.
- **CUT** — deleted, no successor in the fresh tree.

**Subtractive bias (per brief guardrail):** when unsure whether something is
core, it is DEFER or CUT, and the reason is recorded.

---

## Tier 1 — Core lifecycle (the product itself)

| Command | Verdict | Reason (principle / evidence) |
|---|---|---|
| `init` | **SIMPLIFY** | P1/P8. Biggest single blob in the tree: `internal/cmd/init.go` (803 LOC) + `internal/core/initplan.go` (15.1K). Init-plan machinery + pack listing is over-built. Reduce to: scaffold `.specd/`, write embedded templates, emit one plan JSON. |
| `new` | **KEEP** | P2. `internal/cmd/new.go` (92 LOC) — validates slug (`^[a-z0-9][a-z0-9-]*$`), refuses overwrite, scaffolds a spec. Already minimal. |
| `check` | **REDESIGN** | P1/P3. Gate runner is the product's beating heart. Keep, but restructure gate wiring into a pluggable registry (domain 03) instead of hardcoded branches. |
| `approve` | **KEEP** | P6. `internal/cmd/approve.go` (237 LOC) — human phase-boundary transitions under spec lock. This is the "last 20%" human gate the paper names (p.34). |
| `next` | **KEEP** | P4. Frontier/scheduler query over the task DAG. |
| `verify` | **SIMPLIFY** | P3. `internal/cmd/verify.go` (399 LOC) — evidence gate is absolute, but the sandbox (bwrap/container) + `--revert-on-fail` + record format carry a lot of surface. Keep evidence integrity; simplify the record and make the sandbox a fail-closed opt-in. |
| `task` | **KEEP** | P3. Evidence-gated state mutation through the `CompleteTask` integrity path. |
| `status` | **KEEP** | P7. Deterministic projection of `state.json`. |
| `context` | **REDESIGN** | Context engineering (paper pp.15–18). The real engine already lives in `internal/context` (contextpkg); elevate it to a first-class, documented central module feeding `context`, `next --dispatch`, and worker briefs (domain 08). |
| `waves` | **SIMPLIFY → MERGE** | P4. `internal/cmd/waves.go` (77 LOC) is a pure projection of the DAG. Fold its output into `next`/`status`; do not ship a separate verb. |
| `decision` | **KEEP** | P2. `internal/cmd/decision.go` (72 LOC) — appends an ADR record under lock. Cheap, durable, on-disk. |
| `midreq` | **KEEP** | P6. Mid-requirement gates; high/critical are never auto-cleared. |
| `memory` | **KEEP** | P8. Steering memory add/promote. Absorbs `promote` (below). |
| `report` | **SIMPLIFY** | P7. `report_actions.go` (611 LOC) mixes deterministic Markdown/PR-summary with live watch/SSE/webhook streaming. Keep the deterministic projections; DEFER the live streams (domain 11). |
| `promote` | **MERGE → `memory`** | Registered as a top-level command but implemented by `RunPromote` in `eval.go` — a drift smell. Fold into `memory promote`. |

## Tier 2 — Agent-agnostic plumbing (keep, minimal)

| Command | Verdict | Reason |
|---|---|---|
| `handshake` | **KEEP** | P5. Host bootstrap + policy digest (`HandshakeBootstrapVersion`). The universal on-ramp. |
| `mcp` | **SIMPLIFY** | P5. Keep the stdio JSON-RPC server; trim tools to the parity-tested core set + the intent-level tools. Cut raw passthroughs (`specd_brain` / `specd_pinky`) — they add no authority (domain 07). |

## Tier 3 — Orchestration (keep the idea, aggressively minimize)

| Command | Verdict | Reason |
|---|---|---|
| `brain` | **REDESIGN** | The deterministic controller = the paper's orchestrator mode (pp.31–34). Philosophically central, but the surface is huge: `brain.go` (408) + `brain_commands.go` (254) + `brain_policy.go` (155) + `brain_worker.go` (159), backed by `orchestration*.go` (~133K in core). Reduce to the smallest verb set that still guarantees evidence integrity, lease safety, cost/time brakes, cooperative cancel (domain 09). |
| `pinky` | **SIMPLIFY** | Worker ACP protocol. Keep the safety-bearing verbs (claim/heartbeat/report/inbox/checkpoint); drop telemetry-only verbs to optional. |
| `conductor` | **DEFER** | `conductor.go` (204) — rejection-reason clustering analytics. Not on the evidence path; a reporting nicety. |
| `orchestrate` | **CUT** | `orchestrate.go` (105) — surfaces/resolves auto-escalations. Fold into `brain` decisions; no standalone command needed. |
| program tier (`status --program`, `program*.go` ≈55K core + `program.go`/`program_schedule.go` in cmd) | **DEFER** | Orchestration-of-orchestrations (multi-spec). An entire second control plane. Whole tier is v2. |

## Tier 4 — Extended loop / flywheel (default posture: DEFER)

| Command | Verdict | Reason |
|---|---|---|
| `eval` | **DEFER (keep gate hook)** | Gate 10. `eval.go` (17.7K core) rubric engine. The opt-in gate module stays (domain 03); the *command* is a plugin/v2. |
| `review` | **DEFER (keep gate hook)** | Gate 11. Scaffold + freshness parse. Gate module stays; command is v2. |
| `security` (`check --security`) | **KEEP-as-plugin-gate** | Gate 12. stdlib-only scanners, off by default. Fits the pluggable-gate model exactly (domain 03) — the one flywheel piece that is genuinely core-shaped. |
| `deploy` | **DEFER** | Maintenance phase. The production human-approval record (`DeployApproval`) is worth preserving when the tier returns. |
| `observe` | **DEFER** | Feedback phase. Offline error-payload correlation. |
| `ingest` | **DEFER** | Brownfield onboarding (inventory + coverage gate 13). Useful, not MVP. |
| `harness` | **DEFER** | Policy-share + import quarantine. Real security value, but a v2 concern once there is a bundle ecosystem. |
| `dashboard` | **DEFER → MERGE** | Read-only aggregate projection. Fold into `report` if demand appears. |
| `submit` | **CUT** | `submit.go` (115) — a thin timeout wrapper around a configured shell command. No harness value a user cannot get from their own script. |
| `migrate` | **CUT (from MVP)** | A fresh start has no legacy schema to migrate. Reintroduce only when the v1 `state.json`/config schema first evolves. |

## Tier 5 — State backends

| Item | Verdict | Reason |
|---|---|---|
| `backend_git.go` (default) | **KEEP** | git-native, no Go git lib, stdlib only. Aligns with the zero-dep, single-static-binary product value. |
| `backend_postgres.go` (`specd_postgres`) | **CUT (→ optional build tag)** | External-infra coupling. Violates the zero-dep, git-native promise. Not in the default binary; keep only as a build-tag plugin if ever demanded. |
| `backend_redis.go` (`specd_redis`) | **CUT (→ optional build tag)** | Same reasoning. |

## Hidden subsurfaces (not registry commands — recorded so they survive triage)

| File | Backs | Verdict |
|---|---|---|
| `internal/cmd/dispatch.go` | `next --dispatch` | **KEEP** (part of context engine, domain 08) |
| `internal/cmd/security.go` | `check --security` | **KEEP-as-plugin-gate** (domain 03) |
| `internal/cmd/program.go` / `program_schedule.go` | `status --program`, `program schedule` | **DEFER** (program tier) |
| `internal/cmd/watch_webhook.go` | `report watch --webhook` | **DEFER** (live streams, domain 11) |
| `internal/cmd/doc.go` | package doc, 0 functions | **CUT** |
| `internal/cmd/brain_worker.go` | `brainRunner` seam | **KEEP** (needed test seam, domain 09) |

---

## Net MVP after triage

**v1 command surface (16 verbs):**
`init · new · check · approve · next · verify · task · status · context · decision · midreq · memory · report · handshake · mcp` + the opt-in orchestration tier **`brain · pinky`**.

**Merged away:** `waves`→`next/status`, `promote`→`memory`, `dashboard`→`report`.

**Cut outright:** `orchestrate · submit · migrate · doc` + Postgres/Redis backends.

**Deferred to v2 / plugin tier:** `conductor` · program tier · the entire flywheel
(`eval · review · deploy · observe · ingest · harness`) + live report streams.
`security` returns as a pluggable gate module, not a command.

**Reduction:** 29 registered commands → **16 core verbs (2 of them an opt-in
tier)**; the ~350K orchestration/program/ACP mass in `internal/core` is the
primary code the fresh tree does *not* carry forward wholesale.
