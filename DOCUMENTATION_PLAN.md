# specd — Documentation Analysis & Action Plan

> Planning artifact. Not user-facing docs. Drives the creation of `docs/`.
> Author: docs pass, 2026-07-05. Branch: `fresh-start`.

---

## 0. TL;DR

- The live repo (branch `fresh-start`) ships **`README.md` only**. There is **no `docs/` directory**, even though `CLAUDE.md` and `scripts/docs-lint.sh` reference `docs/command-reference.md` and `docs/CHEATSHEET.md`. Docs must be **created**, not edited.
- `reference/docs/` (20 files) is the **frozen v1 museum**. It is a *structural and philosophical* template only. **Do not copy it.** It documents features that **do not exist** in the current binary (see §3 gap table).
- The **single source of truth** for the CLI surface is `internal/core/commands.go` (`var Commands`). Every command, flag, exit code, and allowed-phase in the new docs must be traceable to it or to code — never to the old docs.
- Deliverable of this pass: a lean, GitHub-friendly `docs/` set (§5) that is **verifiably in sync** with the codebase, wired into `docs-lint.sh`.

---

## 1. What specd is (grounded in current code)

`specd` is a **spec-driven coding harness CLI** — Go, standard library only, zero runtime deps, single static binary (`github.com/0xkhdr/specd`, Go 1.22+). It moves process enforcement out of the LLM context window into a deterministic, local, tool-gated pipeline.

**Thesis: the agent reasons; the harness enforces.**

Pipeline: `requirements → design → tasks → evidence-gated execution`, guarded by human-approved phase gates and a no-bypass evidence rule.

### Philosophy pillars (to carry into `concepts.md`)
1. **Determinism first** — no LLM in any gate, DAG, or report path. All are pure functions of on-disk `.specd/` state.
2. **Evidence integrity** — a task completes *only* against a passing verify record (exit 0 pinned to a real git HEAD). **No bypass flag exists.**
3. **Planning ratchet** — phases advance only on human `approve` once gates pass; you cannot move status backward (`CanAdvanceStatus`).
4. **Structural invariants** — atomic writes, CAS on `state.json` revision, reentrant per-spec lock, byte-stable tasks parser, `go:embed` templates, zero runtime deps.
5. **Subtractive bias** — when unsure, cut or defer and record the decision.
6. **Agent-agnostic** — any command-running agent or MCP client; roles constrain capability, not identity.

---

## 2. Domain map (the real internal architecture)

Source of truth for domains = the package tree, not the old docs. Every domain below is a documentation target.

| Domain | Packages / key files | What it owns |
|---|---|---|
| **CLI & dispatch** | `main.go`, `internal/cli`, `internal/cmd` (`registry.go`, `dispatch.go`, `lifecycle.go`) | Arg parsing, verb→`Handler` map, phase enforcement, fail-closed (exit 2) on unknown verbs, deferral notice for deferred verbs. |
| **Command palette (SOT)** | `internal/core/commands.go` | `var Commands` — every verb, flag, exit code, allowed phase, examples. Feeds help/dispatch/MCP/roles. `HelpSchemaVersion`. |
| **State & storage** | `state.go`, `io.go`, `lock.go`, `paths.go` | `core.AtomicWrite`, CAS on `state.json` revision, reentrant `core.WithSpecLock`, path resolution. |
| **Lifecycle / phases** | `phases.go` | Statuses (`requirements→design→tasks→executing→verifying→complete`, plus `blocked`) → Phases (`perceive→analyze→plan→execute→verify→reflect`). Forward-only. |
| **DAG & execution** | `dag.go`, `frontier.go` | Acyclic task DAG; "frontier" = concurrent runnable set (waves, not lines). |
| **Tasks parser** | `tasksparser.go` (+ fuzz test) | Byte-stable round-trip parse of `tasks.md`. |
| **Evidence & verify** | `evidence.go`, `task_complete.go`, `verify/exec.go`, `criteria.go` | Verify record (exit code + git HEAD); task completion; per-criterion evidence. |
| **Gates** | `internal/core/gates/` (`core.go` registry, `ears.go`, `sync.go`, `contextbudget.go`, `approval.go`, `criteria.go`, `review.go`) + `gates/security/` | The 14 core gates (§4) + opt-in security gate. |
| **Templates & scaffold** | `embed_templates/` (roles, steering, reports), `roles.go`, `scaffold.go`, `managed.go` | `specd init`/`new` scaffolding; managed-region repair/refresh; `AGENTS.md` emission. |
| **Config** | `config_loader.go`, `config_validate.go` | Effective config, digests (handshake). |
| **Memory (flywheel)** | `memory.go` | Append/promote steering-memory patterns. |
| **Program / cross-spec** | `program.go`, `commitlink.go` | `link`/`unlink`, cross-spec dep ordering, program view. |
| **Reporting** | `report.go`, `report_metrics.go`, `prometheus.go`, `prsummary.go`, `history.go`, `telemetry.go` | Deterministic status/PR/metrics/history reports; Prometheus textfile; token/cost ledger. |
| **Handshake** | `handshake.go`, `manifest_tools.go`, `mcpconfig.go` | Bootstrap material, palette/config digests. |
| **Orchestration (Brain/Pinky)** | `internal/orchestration/` (`lease.go`, `decide.go`, `acp.go`, `driver.go`, `session.go`, `brakes.go`, `checkpoint.go`, `recover.go`, `authority.go`, `sense.go`) | Opt-in deterministic controller. No LLM in the decision path. |
| **MCP server** | `internal/mcp/` | Serves the palette as a stdio MCP server (`specd mcp`). |
| **Context manifest** | `internal/context/` | Bounded, cited per-task context (`specd context`). |
| **Integration** | `internal/integration/` | Role/steering snippet registry + conformance tests. |

### Runtime surface (in a managed project)
```
.specd/specs/<slug>/{requirements.md,design.md,tasks.md,state.json,.lock}
.specd/roles/*.md
.specd/steering/*.md
AGENTS.md            # host integration guide, written by init
```
**Split to document explicitly:** runtime reads `.specd/specs/`; this repo's own planning artifacts live in top-level `specs/`. `regress-lint.sh` smell "A" catches verify lines targeting the wrong one.

---

## 3. Gap analysis — old docs vs. current binary (CRITICAL)

The old `reference/docs/` describes a richer v1. **Documenting its features as if present would be wrong.** Cross-check before writing.

| Old-doc feature | Current status | Action |
|---|---|---|
| `report --serve` live dashboard, `dashboard.md` | **Absent** from `report` flags | **Omit.** No dashboard doc. |
| `report --watch` / `FrontierEvent` SSE stream | **Absent** | **Omit** unless found in code. |
| `report --diff` across git refs | **Absent** | **Omit** unless found. |
| `specd init --pack` / spec packs, `spec-packs.md` | `init` has no `--pack` | **Omit** unless found. |
| `new --orchestrated`, `status --recommend` | `new` has only `--agent`; `status` has `--json`/`--program` | **Omit** those flags; describe orchestration opt-in per actual mechanism (verify in code). |
| "7 core gates" | **14 core gates** registered (§4) | Use the real 14. |
| Custom external gates (subprocess JSON contract), `custom-gates.md` | Registry is static Go gates; no external contract seen | **Verify in code first.** If absent, omit `custom-gates.md`. |
| `report --pr-summary` GitHub action verb | `report` has `--pr`; `.github/` has `action.yml` | Document from **actual** action + report flags. |

**Rule for the whole pass:** every command, flag, gate name, phase, and file path in new docs must be greppable in the current tree. If it is only in `reference/`, it does not go in.

### Confirmed current command palette (from `commands.go`)
`help · version · init · new · approve · midreq · decision · next · status · task · check · verify · context · memory · mcp · handshake · brain · report · link · unlink · review · submit · triage`
- `triage` is **`Deferred: true`** → prints deferral notice, exits 0. Document as deferred, not functional.

---

## 4. The 14 core gates (from `gates/core.go` `CoreRegistry`)

Document each with: what it checks, which phase it fires in, severity, failure message shape.

`task-ids · dependencies · dag · roles · files · verify · evidence · context-budget · ears · approval · sync · design · criteria · review`

Plus **opt-in security gates** (`gates/security/`: injection, secrets, slopsquat testdata) run via `specd check --security`.

---

## 5. Proposed `docs/` structure

Lean set that mirrors the *good* parts of the old index, trimmed to current reality. Audience-first (GitHub-friendly: one landing index, task-oriented guides, one reference).

| File | Audience | Purpose |
|---|---|---|
| `docs/README.md` | everyone | Landing index + "fast paths" table. Links every doc below. |
| `docs/concepts.md` | evaluators, new users | Why specd exists: the foundational split, philosophy pillars (§1), lifecycle model, execution modes (base vs orchestrated). |
| `docs/user-guide.md` | operators in a managed repo | Install, `init`/`new`, the phase lifecycle, authoring EARS `requirements.md` / `design.md` / `tasks.md`, the **verify → complete** flow, `approve`, mid-stream `midreq`/`decision`, reading `status`/`next`. |
| `docs/command-reference.md` | everyone (**SOT doc**) | Every verb: usage, flags, exit codes, allowed phases, examples — generated to match `commands.go`. Grouped (lifecycle / execution / inspection / orchestration / integration). |
| `docs/CHEATSHEET.md` | everyone | Condensed mirror of command-reference. **`docs-lint.sh` enforces verbatim match** — must be produced/maintained together. |
| `docs/validation-gates.md` | authors, contributors | The 14 gates (§4) + security gates: what each checks, when it fires, how to fix a failure. |
| `docs/agent-integration.md` | agent/harness authors | `AGENTS.md` loop, the 4 roles (scout/craftsman/validator/auditor), steering constitution, `context` manifest, `next --dispatch` packets, Brain/Pinky orchestration, cross-spec programs (`link`). |
| `docs/architecture.md` (a.k.a. contributor-guide) | contributors | Codebase walkthrough by domain (§2), the non-negotiable invariants, concurrency/durability model (atomic write + CAS + lock), byte-stable parser, extension recipes (adding a verb/gate), the `reference/` museum rule. |
| `docs/mcp-guide.md` | MCP client users | `specd mcp` stdio server, `specd mcp --config <host>`, `handshake bootstrap` digests. |
| `docs/open-spec-format.md` | integrators | On-disk artifact schema (`specd check --schema` / `--schema-only`). **Verify schema surface in code before writing.** |
| `docs/github-action.md` | CI users | The `.github/action.yml` composite action + `report --pr` summary in CI. Document from the actual action file. |
| `docs/troubleshooting.md` | operators | Blocked tasks, lock contention, CAS conflicts, verify/sandbox failures, `task --override` escalation reset. |

**Deliberately omitted** (until proven present in code): `dashboard.md`, `spec-packs.md`, `custom-gates.md`, `report --watch/--diff/--serve`. Add later only if the feature is found.

---

## 6. Cross-cutting rules for the pass

1. **Grep-before-write.** Every CLI token cited must resolve in `internal/core/commands.go` or the relevant package. Keep a scratch checklist.
2. **`command-reference.md` ⇄ `CHEATSHEET.md` sync.** Author together; run `./scripts/docs-lint.sh` before done. Read the script first to learn the exact match rule it enforces.
3. **Match house voice.** Old docs use "The agent reasons. The harness enforces." + emoji section headers + tables. Keep that register; it is GitHub-friendly and already the project's brand.
4. **No feature invention.** If unsure whether something exists, mark it TODO and verify — never document aspirationally.
5. **Link graph.** `docs/README.md` links all; each guide cross-links siblings (concepts↔user-guide↔command-reference).
6. **Update `README.md` root** to point at `docs/` and drop any stale claim.

---

## 7. Action plan (ordered)

**Phase A — Verify surface (no writing yet)**
- A1. `grep` the palette; confirm the 23 verbs + flags list in §3.
- A2. Read `scripts/docs-lint.sh` — learn the exact `command-reference.md` ↔ `CHEATSHEET.md` contract.
- A3. Confirm/deny each §3 gap-table item in code (dashboard, packs, custom gates, watch/diff, schema surface). Update this plan's "omitted" list with findings.
- A4. Read one handler each for the non-obvious verbs (`brain`, `verify --criterion`, `memory`, `handshake`, `review`, `submit`) to describe behavior accurately.

**Phase B — Reference core (SOT first)**
- B1. Write `docs/command-reference.md` straight from `commands.go` (grouped, full flag/exit/phase tables).
- B2. Derive `docs/CHEATSHEET.md`; run `docs-lint.sh` until green.
- B3. Write `docs/validation-gates.md` from `gates/core.go` + `gates/security/`.

**Phase C — Narrative guides**
- C1. `docs/concepts.md` (philosophy, lifecycle, execution modes).
- C2. `docs/user-guide.md` (install → lifecycle → verify/complete → troubleshoot pointer).
- C3. `docs/agent-integration.md` (roles, steering, dispatch, Brain/Pinky, programs).
- C4. `docs/architecture.md` (domain walkthrough, invariants, concurrency).

**Phase D — Integrations & edges**
- D1. `docs/mcp-guide.md`, D2. `docs/open-spec-format.md` (if schema confirmed), D3. `docs/github-action.md`, D4. `docs/troubleshooting.md`.

**Phase E — Index & wire-up**
- E1. `docs/README.md` index + fast paths.
- E2. Update root `README.md` to link `docs/`.
- E3. Final sync: `./scripts/docs-lint.sh`, `gofmt -l .` (docs don't affect it but run the full pre-push set), and a manual link-check.

**Exit criteria:** every doc's CLI/gate/path references grep-clean against the tree; `docs-lint.sh` green; no reference to omitted/absent features; README points at `docs/`.

---

## 8. Risks & drift traps

- **Old-doc contamination** — biggest risk. Treat `reference/docs/` as prose style only; never as fact.
- **CHEATSHEET drift** — the one doc with a machine-enforced contract. Break it and CI fails. Author it last-in-pair with the reference and lint immediately.
- **Deferred verbs** — `triage` reads like a feature; it is not wired. Label clearly.
- **Orchestration overreach** — Brain/Pinky is opt-in and deterministic; do not imply an LLM sits in its decision path.
- **Schema/action specifics** — `open-spec-format.md` and `github-action.md` must be written from the actual schema/`action.yml`, not assumed shapes.
