# specd Rebuild — Hard Review & Production Action Plan

**Date:** 2026-07-04 · **Branch:** fresh-start @ c16d4f9 · **Reviewer:** independent audit of the Stage 1→3 transition (fresh-start artifacts → specs → implementation) against The New SDLC with Vibe Coding, FRESH_START_BRIEF.md, and fresh-start/00-decisions.md (ADRs).

**Method:** every claim in specs/progress.md was verified against the actual tree — full build, vet, and test run; the built binary driven end-to-end through the whole lifecycle (init → new → approve → next → verify → report → brain → mcp → handshake) in a throwaway git repo; code read at the seams (registry, lifecycle, gates, evidence, orchestration, MCP); paper re-read at the harness/conductor-orchestrator chapters (pp. 28–34).

---

## 1. Verdict in one paragraph

The rebuild is architecturally faithful and mechanically healthy — zero-dep single binary, atomic writes, CAS, reentrant locks, append-only evidence ledger, pluggable gate registry, pure Decide() orchestration, 84 green tests — but it is not yet the product the specs describe. The harness’s central promise (evidence gates every state change; humans approve phase transitions; agents cannot bypass either) is scaffolded but not enforced end-to-end: approvals don’t gate execution, no command can complete a task, the MCP surface lets an agent approve its own gates, and specs/progress.md marks tasks ✅ whose verify commands fail today — the exact “mark ahead of evidence” failure mode the whole project exists to prevent. Everything below is fixable in a short, well-ordered set of waves; the foundations are genuinely good.

---

## 2. What is verified GREEN (checked, not trusted)

| Invariant / claim | Evidence | Status |
|---|---|---|
| Build + vet clean | `go build ./...`, `go vet ./...` | ✅ |
| Test suite | `go test ./...` — 84 pass, 12 packages | ✅ |
| Zero runtime deps (ADR-8) | `go.mod` has no require block | ✅ |
| Atomic write / CAS / reentrant lock | `internal/core/{io,state,lock}.go` + tests; lifecycle commands actually use `WithSpecLock` + `SaveStateCAS` | ✅ |
| Tasks parser round-trip + fuzz | `internal/core` fuzz + round-trip tests | ✅ |
| Append-only evidence ledger | `evidence.jsonl` written with `task_id`/`command`/`exit_code`/`git_head`; failed runs recorded too | ✅ |
| `verify --revert-on-fail` | `internal/cmd/registry.go:389` (`withRevertOnFail`, git-diff snapshot/restore) | ✅ |
| Pluggable gate registry (ADR-4) | `internal/core/gates/registry.go`; security gate registers only behind `--security` | ✅ |
| Pure orchestration core (ADR-3) | `internal/orchestration/decide.go` + `TestDecidePure`/`TestNoLLM`; no LLM/network in decision path | ✅ |
| Report purity | `TestNoLLMInRender`; report/status render from state+evidence only | ✅ |
| Embedded templates, 4 roles exactly (ADR-10) | `internal/core/embed_templates/roles/{scout,craftsman,validator,auditor}.md` — scribe gone, auditor restored | ✅ |
| Deferred verbs fail loud | `specd triage` → “deferred — not yet wired”, exit 0 (R13.8) | ✅ |
| Marker-merged AGENTS.md on init | emitted in e2e run | ✅ |
| CLI regression spec exists | `specs/13-cli-regression/` + `internal/cmd/e2e_test.go` | ✅ |

The subtraction discipline also held: no Postgres/Redis, no program tier, no conductor analytics, no legacy JSON config path. ADR-3/5/6/9/10 are structurally honored.

---

## 3. Findings — where the rebuild violates its own rules

&gt; **Severity:** 🔴 breaks a guardrail/ADR/paper principle · 🟠 spec-vs-impl drift · 🟡 quality/consistency.

### 🔴 F1. specs/progress.md contains falsified completion claims (ADR-8 violation)

`progress.md` marks all 76 tasks ✅ with “verify passed + record written”. Verified false:

- **T1.1 verify:** `grep -q 'harness component' docs/charter.md` — `docs/` does not exist at all.
- **T8.6** (`docs/context.md`) and **T12.3** (`docs/deferred-flywheel.md`) — same: files absent.
- **T2.6 verify:** `status demo --json | grep '"mode":"simple"'` — status JSON contains no `mode` field (report model never reads `state.json`); this command cannot pass.
- **T2.4 verify:** `test -f .specd/specs/demo/state.json` — repo’s `.specd/specs/demo/` has only a stale `tasks.md`.

No evidence records exist anywhere for the 76 rebuild tasks (the rebuild did not dogfood specd).

This is the project’s own cardinal sin: *“Keep this file the projection of reality — never mark ahead of evidence.”* Trust in the tracker is now zero until re-audited.

---

### 🔴 F2. The core loop cannot close — no task-completion path

There is no `task complete` verb, verify success does not transition task state, and task status is derived from a Marker column in `tasks.md` (`internal/cmd/registry.go:367-382`) that the scaffolded table doesn’t even include. The only way to complete a task is to hand-edit `tasks.md` with a marker — undocumented, ungated, and disconnected from the evidence ledger. T5.4 (“complete requires evidence”) is claimed done, but the completion command it should guard doesn’t exist. The paper’s think → act → observe loop (p. 30) has no “observe→record→advance” closure.

---

### 🔴 F3. Approvals and phases gate nothing

`loadSpec` (`internal/cmd/registry.go:350`) never reads `state.json`. Consequently `next`, `verify`, `context`, and `report` all ignore status/phase/approvals — verified live: `specd next` returned a dispatchable frontier while the spec was still in `requirements` status, pre-approval. The phase ratchet (`approve` → `AdvanceStatus`) writes state that nothing downstream consults. The paper’s phase-gated SDLC (harness present at every phase, humans gate transitions) is currently decorative.

---

### 🔴 F4. MCP surface lets an agent approve its own human gates

`tools/list` over MCP exposes: `help`, `init`, `new`, `approve`, `midreq`, `next`, `status`, `task`, `check`, `verify`, `context`, `mcp`, `handshake`, `brain`, `triage`.

`ForbiddenTool` (`internal/core/manifest_tools.go`) blocks only `report`, `decision`, `memory`. An orchestrated agent can therefore call `approve` — the one verb that exists specifically to record human judgment — plus `init`, `brain`, and recursively `mcp`. This inverts the human-in-the-loop principle the whole gate system is built on.

---

### 🔴 F5. ADR-7 (mode enum) unimplemented

`internal/core/state.go:18` defines `ModeDefault = "default"`. ADR-7 mandates exactly `simple | orchestrated` (paper’s conductor/orchestrator, p. 31–33), default `simple`, set via `new --mode`, changed only via auditable `approve --mode`. None of that exists: no `--mode` flag on `new`, no mode transition, no mode in status output, and orchestration eligibility never keys off it.

---

### 🔴 F6. decision / midreq capture no content — the audit trail is hollow

`appendScoped` (`internal/cmd/lifecycle.go:122`) records only `{"kind":"midreq"}` / `{"kind":"decision"}` — no text, no rationale, no scope, no author, no timestamp, no git HEAD. Approval records are likewise just `{"gate":"design"}`. A “recorded human decision” that records nothing of the decision fails both the brief’s evidence-integrity mandate and the paper’s observability requirement (“audit exactly why a decision was made”, p. 30).

---

### 🟠 F7. CLI surface is 18 verbs; Spec 01 R1.5 says 16 — the two extras violate ADR-5

`memory` and `triage` are the extras. ADR-5 is explicit: “v1 ships no flywheel commands; only the security gate module.” `triage` is a registered stub; `memory` is a live command whose output nothing consumes. Either cut both (subtractive bias says cut) or write a superseding ADR — the current state is silent drift, exactly what ADR-10 warns against.

---

### 🟠 F8. Gate registry is missing the content gates the specs promise

Registered gates (`internal/core/gates/core.go:21-28`): `task-ids`, `dependencies`, `dag`, `roles`, `files`, `verify`, `evidence`, `context-budget` — all structural checks over `tasks.md` + ledger. Missing versus Spec 03 / the analyses:

- **EARS gate** (T3.2 claims `ears.go`; file doesn’t exist) — `requirements.md` content is never validated. The scaffold placeholder literally passes because the template text is EARS-shaped.
- **Approval/phase gate** — nothing checks that requirements/design are approved before task waves.
- **Sync gate** (ADR-1’s “Gate 6”, checkbox/marker ↔ state agreement) — absent.
- **Design gate**: `design.md` says “The design gate reads this file before tasks execute” — no gate reads it.

---

### 🟠 F9. Steering and memory are scaffolded but inert (paper P8)

`.specd/steering/*.md` (6 files) and per-spec `memory.md` are written by `init`/`new`, and `specd memory` appends patterns — but the context manifest (`internal/context/manifest.go`) includes none of them, and no other code path reads them. The “constitution” and “learning flywheel” exist as files only. The paper is unambiguous that rule files are the first harness component (p. 28).

---

### 🟠 F10. Config layer: wrong filename, fail-silent, dead flag

`contextBudget`/`handshake` load `project.yml` (`internal/cmd/registry.go:206,276`); ADR-2 and the scaffold story say `config.yml`. Neither file is ever seeded by `init` (deferred in ADR-10 and then never delivered).

Errors are swallowed: `config, _ := core.LoadConfig(...)` — ADR-2’s fail-loud requirement violated; a corrupt config silently becomes defaults.

`init --agent=&lt;name&gt;` is accepted and ignored (`runInit` drops flags).

---

### 🟠 F11. Orchestration fail-closed is only half-closed; pinky verbs missing

`brain start` succeeds and creates a session with no `orchestration.enabled` config present (dispatch then refuses — good — but ADR-3 says the tier is “inert unless enabled, fail-closed”; session creation is not inert). ADR-3’s shipped surface includes `pinky {claim|heartbeat|report|inbox|checkpoint}`; `internal/cmd/pinky.go` exists but no `pinky` verb is registered in `core.Commands` — the worker side of the control plane is unreachable from the CLI.

---

### 🟠 F12. The repo’s own .specd/ contradicts the shipped scaffold (dogfood drift)

Root `.specd/roles/` still contains `scribe.md` (explicitly CUT by ADR-10) and lacks `auditor.md`; `.specd/specs/demo/` is leftover junk (a task with role `builder` — not one of the four roles — no `state.json`, no evidence). The flagship repo of a spec-discipline tool should be its cleanest user.

---

### 🟡 F13. Progress.md task files: don’t match the tree

Many claimed files never existed under those names (`evidence/ledger.go`, `customgate.go`, `ears.go`, `frontier.go`, `new.go`, `approve.go`, `prsummary.go`, `report_metrics.go`, …) — the work was consolidated into `lifecycle.go`/`registry.go`/`report.go` etc. Consolidation is fine; the DoD (“touches only the files: its task declares”) was not honored, and the tracker was never corrected.

---

### 🟡 F14. Smaller correctness/UX gaps

- Evidence accepts `git_head: "unknown"` (pre-first-commit) without complaint — an evidence record that can’t be pinned to a commit shouldn’t count toward completion.
- No timestamps anywhere (evidence records, approvals, decisions) — weak observability (paper p. 28).
- `specd check &lt;slug&gt;` prints nothing on success — fine for pipelines, but a one-line `8 gates: all green` summary serves the human conductor.
- `specd task &lt;id&gt;` takes no spec argument and scans all specs — inconsistent with every other verb’s `&lt;slug&gt;` shape.
- Context manifest item paths are emitted without the `.specd/` prefix for spec files but with it for role files — inconsistent contract for consumers.
- The `--unverified --evidence` escape hatch for read-only roles (brief §5) is documented in templates but implemented nowhere.

---

## 4. Paper-adherence matrix

| Paper principle | State | Blocking findings |
|---|---|---|
| P1 Agent = Model + Harness; harness is deterministic code | ✅ core honors it (pure gates/Decide/render) | — |
| P2 Specs as source of truth, agent-authored, git-diffable | ✅ structure; 🟠 tasks table lost human-visible status (no marker column scaffolded) | F2 |
| P3 Evidence gates every state change | 🔴 not enforced — no completion path, approvals ungated | F1 F2 F3 |
| P4 Decomposed waves / DAG execution | ✅ parser, DAG, frontier, waves all real and tested | — |
| P5 Agent-agnostic floor (adapters, handshake) | 🟠 works, but `--agent` dead, config never seeded | F10 |
| P6 Humans gate phase transitions | 🔴 approvals decorative; MCP lets agents approve | F3 F4 F5 |
| P7 Deterministic reporting/observability | 🟠 pure but thin: no mode/phase in status, no timestamps | F5 F6 |
| P8 Steering constitution + memory flywheel | 🔴 files exist, nothing reads them | F9 |

---

## 5. Action plan — waves to production

&gt; Ordered by dependency, sized small. Rule for every task below: drive it through specd itself (create a real spec in `.specd/`, verify with evidence) — the rebuild’s biggest process failure was not dogfooding; Wave 0 makes that possible honestly.

### Wave 0 — Restore truth (½ day; blocks everything)

| id | task | verify |
|---|---|---|
| P0.1 | Re-audit `specs/progress.md`: run every `verify:` literally; flip false ✅ back to ⬜ (at minimum T1.1, T2.4, T2.6, T5.4, T8.6, T12.3); correct `files:` to the real tree (F13) | every remaining ✅ has a passing command |
| P0.2 | Reset repo `.specd/`: delete `roles/scribe.md` + junk `specs/demo/`, re-run `specd init`, commit clean scaffold (F12) | `ls .specd/roles` = exactly 4, auditor present |
| P0.3 | Write the three missing docs the tracker already claims: `docs/charter.md` (verb→harness-component map), `docs/context.md`, `docs/deferred-flywheel.md` (F1) | the original T1.1/T8.6/T12.3 verify commands pass |

---

### Wave 1 — Close the loop (the product doesn’t exist without this)

| id | task | verify |
|---|---|---|
| P1.1 | Add `specd task complete &lt;slug&gt; &lt;id&gt;`: refuses without a passing evidence record for the task at current HEAD; updates status in `state.json` under lock+CAS (per ADR-1 — state, not marker, is machine truth); make `taskStatus` read state, not markers (F2) | `complete`-without-evidence exits non-zero; with evidence, report shows task complete |
| P1.2 | Gate execution on phase: `next`/`verify` load `state.json` and refuse task dispatch until requirements and design are approved (F3) | `next` on a fresh spec returns empty + explains why; after both approvals returns frontier |
| P1.3 | Implement ADR-7: Mode enum `simple\|orchestrated`, default `simple`; `new --mode`, auditable `approve &lt;slug&gt; mode --to orchestrated`; expose mode/phase/status in `status --json`; orchestration (`brain`) eligibility requires `orchestrated` (F5) | original T2.6 verify passes; `brain start` on a simple spec refuses |
| P1.4 | Add `--unverified --evidence &lt;text&gt;` to `task complete` for read-only roles only (scout/auditor), recorded as a distinct evidence kind (F14, brief §5) | craftsman task refuses `--unverified`; scout task accepts and ledger shows kind |

---

### Wave 2 — Seal the trust boundary

| id | task | verify |
|---|---|---|
| P2.1 | Extend `ForbiddenTool` to block `approve`, `init`, `mcp`, `brain` (and `task complete` unless evidence-backed) from the MCP surface; keep MCP↔CLI parity test honest by asserting the deny list too (F4) | MCP `tools/list` excludes `approve`; `tools/call approve` returns policy error |
| P2.2 | Make `brain start` fail closed: refuse to create a session unless `orchestration.enabled: true` in config and spec mode is `orchestrated` (F11) | `brain start` on default config exits non-zero with reason |
| P2.3 | Register the `pinky {claim\|heartbeat\|report\|inbox\|checkpoint}` verbs per ADR-3, or write a superseding ADR deferring the worker CLI and delete `pinky.go` (F11) | surface matches whichever ADR governs |

---

### Wave 3 — Make records mean something

| id | task | verify |
|---|---|---|
| P3.1 | Enrich all records: `midreq`/`decision` take required `--text` (and optional `--scope`); every record (approval, decision, midreq, evidence) gets timestamp, `git_head`, and actor (`$SPECD_ACTOR` or OS user) (F6, F14) | `decision demo --text "…"` round-trips through `status --json` |
| P3.2 | Reject evidence with `git_head: "unknown"` for completion purposes (warn at verify time, refuse at `task complete`) (F14) | completion in a commitless repo fails with clear message |

---

### Wave 4 — Finish the gate engine + wake the constitution

| id | task | verify |
|---|---|---|
| P4.1 | EARS gate: warn when `requirements.md` lines lack `When …, the system shall …` shape; error when the file is still the unedited scaffold stub (F8) | fresh scaffold: `check` errors on placeholder; edited EARS file passes |
| P4.2 | Approval gate: error when tasks show progress while requirements/design unapproved; design-stub gate: error when `design.md` sections are empty at design approval (F8) | `approve demo design` with empty design refused |
| P4.3 | Include `steering/*.md` and spec + steering `memory.md` in the context manifest (bounded, budget-counted); this is what makes `specd memory` output actually reach agents (F9) | `context demo T1 --json` lists steering items within budget |
| P4.4 | Preserve byte-identical `check` output when new gates are off/green (extend `parity_test.go`) | parity test green |

---

### Wave 5 — Surface & config reconciliation

| id | task | verify |
|---|---|---|
| P5.1 | Decide the 16-vs-18 verb conflict by ADR: recommended — cut `triage` verb (stub; ADR-5 says no flywheel commands) and fold `memory` into scope via a superseding ADR only after P4.3 makes it functional; update Spec 01 R1.5 + charter to the final count (F7) | `bare specd` verb count == spec count; ADR recorded |
| P5.2 | Config: single name `config.yml` (per ADR-2), seeded by `init`, loaded fail-loud (parse error = hard exit, not silent defaults); wire or remove `init --agent` (F10) | corrupt `config.yml` → non-zero exit with message |
| P5.3 | CLI consistency: `task &lt;slug&gt; &lt;id&gt;`; normalize manifest paths (always repo-relative incl. `.specd/`); `check` prints one-line green summary (F14) | e2e regression spec updated and green |

---

### Wave 6 — Production hardening & release

| id | task | verify |
|---|---|---|
| P6.1 | CI (GitHub Actions already scaffolded in `.github/`): build + vet + `test -race` + fuzz smoke + e2e on every push; release job builds static binaries (`CGO_ENABLED=0`, version stamped via `-ldflags`) | green pipeline on branch |
| P6.2 | Add `specd --version` / version in `handshake` from build stamp | `specd --version` prints tag |
| P6.3 | Dogfood gate: this repo’s own `.specd/` carries specs P1–P6 above, each closed via `specd task complete` with real evidence; `specd report --pr` output pasted into the release PR | report shows 100% with evidence for the hardening waves |
| P6.4 | README/docs pass: charter, quickstart (conductor flow + orchestrated flow), MCP setup per host adapter | docs build; links resolve |

&gt; **Suggested sequencing:** Wave 0 immediately (it’s honesty, not features) → 1 → 2 in series (they share `lifecycle.go`/`registry.go`), then 3‖4 in parallel, then 5 → 6. Rough effort: Waves 0–2 ≈ 2–3 focused days; 3–5 ≈ 2–3 days; 6 ≈ 1–2 days. Nothing here requires new dependencies or new packages beyond what exists.

---

## 6. What NOT to do (guardrails re-affirmed)

- Don’t add packages, dependencies, or a “task engine” abstraction — every fix above lands in existing files (`lifecycle.go`, `registry.go`, `gates/core.go`, `manifest.go`, `state.go`).
- Don’t restore flywheel features to fix F7 — subtract the stub; ADR-5’s re-entry seams (`Gate` interface + `records` map) are already in place and correct.
- Don’t move task status back into `tasks.md` markers — ADR-1 chose `state.json`; finish that choice instead of half-living in both worlds.
- **Don’t ship until Wave 0 + Wave 1 + P2.1 are done.** A spec-discipline tool whose own tracker lies, whose loop can’t close, and whose agent surface can self-approve is worse than no tool: it teaches users that evidence is theater.

---

## 7. Bottom line

The fresh-start transition succeeded at architecture and subtraction (ADR-3/4/5/6/9/10 honored, invariants preserved and tested, genuinely minimal surface) and failed at its own process discipline (evidence-free ✅s, approvals that gate nothing, an uncloseable loop, agent-reachable human gates). The gap between “the code passes its tests” and “the product enforces its philosophy” is exactly the paper’s 80% problem — and the remaining 20% is enumerated above as six small waves. Complete Waves 0–2 and specd becomes the thing it describes; complete 3–6 and it’s production-ready.
