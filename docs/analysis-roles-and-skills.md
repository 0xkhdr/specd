# Roles & Skills Exposure — Reference vs. Fresh-Start Analysis

> **Question:** What *roles* and *skills* did the old `specd` expose to the coding
> agent? Do we still need any of those in the new build, or have we replaced them by
> design with a higher-integrity flow? Plus: any domains still under-implemented.
>
> **Verdict up front:** The old surface was **three overlapping vocabularies** for
> telling an agent "who you are and what you may do." The new build collapses them to
> **one** — a 4-role card set backed by a deterministic tool policy — and deletes the
> other two by *design*, not by omission. Nothing of value is lost; integrity goes up.

---

## 1. What the old specd exposed (three layers)

The reference shipped **three independent mechanisms** that all tried to shape agent
behavior. They did not agree with each other — that disagreement is the accretion the
fresh-start brief exists to remove.

### Layer 1 — `internal/spec/role.go` (the RBAC contract, 8 roles)

A typed role table: each role carried `RW`, a `Tools:` allow-list, budget tier, phase
affinity, file policy, prompt class.

| Role | RW | Tool allow-list (reference) |
| :-- | :-- | :-- |
| `scout` | readonly | inspect, read, query, status, context |
| `researcher` | readonly | + diff, report, check |
| `auditor` | readonly | + waves |
| `architect` | readonly | + approve, waves |
| `craftsman` | readwrite | inspect, read, context, status, next, dispatch, verify, task, report |
| `tester` | readwrite | check, verify, task, report |
| `documenter` | readwrite | memory, decision, diff, check |
| `validator` | readwrite | check, status, state_read, doctor |

*Source: `reference/internal/spec/role.go`.*

### Layer 2 — `embed_templates/roles/*.md` (the role *cards*, a different 7)

A **second, non-matching** role vocabulary written to `.specd/roles/`:
`scout, auditor, pinky, craftsman, reviewer, brain, validator`.

Note the divergence from Layer 1: `pinky`, `reviewer`, `brain` appear here but not in
`role.go`; `researcher`, `architect`, `tester`, `documenter` appear in `role.go` but
have no card. **Two role systems, two different lists — this is the core defect.**

### Layer 3 — MCP `prompts` channel (`internal/mcp/prompts.go`)

Roles + phases were *also* surfaced over the MCP native `prompts` channel: 4 phase
prompts (`requirements/design/tasks/execute`) + **8** role prompts
(`scout, researcher, auditor, architect, tester, documenter, validator, craftsman`) —
matching Layer 1, not Layer 2.

### Layer 4 — bundled **skill packs** (`embed_templates/skills/`, ~12 packs)

The old build also embedded Claude-Code *skill* files: `specd-steering`,
`specd-execute`, `specd-brain`, `specd-pinky`, `foundations`, `stages`, and
phase-scoped skills (referenced in `reference/docs/concepts.md:123,149` and
`agent-integration.md:40,561`). These were prose "skill packs" injected into the host.

### And ~48 MCP **tools**

The reference registered ~48 `specd_*` MCP tools (`inspect, read, query, diff, report,
waves, approve, dispatch, memory, decision, doctor, observe, replay, eval, review,
program, midreq, …`) — the raw material the role allow-lists sliced up.

---

## 2. What the new specd exposes (one layer)

| Concern | Old | New | Where |
| :-- | :-- | :-- | :-- |
| Role cards | 7 (Layer 2) ≠ 8 (Layer 1) | **exactly 4**: scout, craftsman, auditor, validator | `internal/core/embed_templates/roles/` |
| Role prompt source | hardcoded in `prompts.go` **and** card files | **single**: `RolePrompt` reads the embedded card (`roles.go`) | `internal/core/roles.go` |
| MCP tools | ~48 | **7 core** (`check, next, verify, task, status, context, handshake`) + **3 opt-in** (`brain_orchestrate, brain_status, brain_approve`) | `docs/07-mcp-surface.md` |
| Skill packs | ~12 embedded | **0** — reconceived as context-manifest item modes | ADR (`00-decisions.md:138`) |
| brain / pinky | role cards | orchestration **verbs** (Tier 3), inert unless enabled | `docs/09` |
| reviewer / scribe / researcher / architect / tester / documenter | roles | **cut** | `00-decisions.md:133-138` |

The new role prompt loader (`internal/core/roles.go`) reads **one** source of truth —
the embedded `.md` card — with craftsman as fallback. No second hardcoded copy. The
old Layer-1/Layer-2/Layer-3 triple-authority problem is structurally gone.

---

## 3. Do we still need any of the old roles/skills? — item by item

| Old primitive | Keep? | Rationale (new integrity story) |
| :-- | :-- | :-- |
| **scout** (readonly locate) | ✅ KEEP | Core read-only role; now the sole card + `pinky-scout` subagent. |
| **craftsman** (bounded edit) | ✅ KEEP | Core readwrite role; file-scoped, verify-gated. |
| **auditor** (readonly diff audit) | ✅ KEEP (restored) | Named by Spec 06 R6.8; absorbs old `reviewer`. |
| **validator** (gates + bookkeeping) | ✅ KEEP | Runs gates, repairs specd state only; no source edits. |
| **researcher** | ❌ CUT | Was scout+diff/report — a scope superset of scout. Fold into scout; the *tool policy*, not a new role, grants the extra reads. |
| **architect** | ❌ CUT | "Shape requirements/design" is a **phase** (requirements/design), not a role. Phase prompts already cover it. |
| **tester** | ❌ CUT | Testing is *inside* a craftsman task's declared scope + its `verify` command. A separate role added a vocabulary, not authority. |
| **documenter** | ❌ CUT | Doc/changelog edits are just a craftsman task with a docs file-scope. No distinct guarantees. |
| **reviewer** | ❌ CUT | Duplicate of auditor. |
| **scribe** | ❌ CUT | Invented, zero provenance. |
| **brain / pinky as roles** | ❌ CUT as roles | They are orchestration *processes*, not identities. Now Tier-3 verbs + subagents. |
| **~12 skill packs** | ❌ CUT / REDESIGN | Reconceived as **context-manifest item modes** (Spec 08 R8.2). A "skill" was really "inject this text at this phase" — that is what the context manifest already does deterministically, without a parallel skill-file registry to keep in sync. |
| **~41 of ~48 MCP tools** | ❌ CUT | Most were passthroughs or read variants that gave "no additional authority beyond intent-level commands" (Spec 07 §2). Keep the 7 that map to real lifecycle verbs. |

**Bottom line:** every *retained capability* survives; only the *redundant vocabularies*
that expressed it were deleted. The four kept roles are exactly the four with a distinct
read/write + tool-authority footprint. Everything else was a synonym.

---

## 4. Why the new flow has more integrity

1. **One source of truth per role.** Old: prompt text lived in `prompts.go` *and* card
   files *and* an RBAC table — three copies that drifted (7 ≠ 8 lists). New:
   `RolePrompt` reads the embedded card; scaffold writes the same bytes. One copy.
2. **Authority is a deterministic policy, not a role's personality.** Tool access is a
   pure function of on-disk manifest policy (`forbidden`/`required`, fail-closed when
   `manifest.json` is missing). Roles describe intent; the *policy* enforces it. No LLM
   in the decision path (guardrail §5).
3. **Skills → context modes.** A skill pack was undifferentiated injected prose. As a
   context-manifest item mode it is scoped, budget-counted, and deterministic — it
   flows through the same manifest the HUD reports on, instead of a shadow channel.
4. **brain/pinky demoted to verbs.** Orchestration is opt-in and inert unless
   `orchestration.enabled`. It is no longer a role an agent can "be" in single-agent mode.
5. **Surface shrank ~48→10 MCP tools.** Fewer tools = smaller attack/confusion surface,
   and every survivor has CLI↔MCP parity (`TestMCPParity`, Spec 07 §2).

This directly serves the north star: same thesis (`Agent = Model + Harness`), minimal
accurate path, accretion removed.

---

## 5. Domains still under-implemented / to verify

Observed while tracing (root already has `internal/`, `specs/`, `.specd/`, `docs/` — so
Stage 3 implementation has begun despite the brief's "await approval" gate). Flagging,
not fixing:

| # | Domain | Status signal | Gap to close |
| :-- | :-- | :-- | :-- |
| 07 | **MCP tool registration** | `internal/mcp/tools_core.go` routes via a `command.Name` registry, but the 7 core tool *names* aren't grep-visible as literals — couldn't confirm all 7 + 3 are registered. | Verify the core-command registry actually emits `check,next,verify,task,status,context,handshake` and the 3 brain tools gate on `orchestration.enabled`. Confirm `TestMCPParity` covers each. |
| 07 | **Per-spec tool policy / fail-closed** | Design (Spec 07 §4) specifies `manifest.json` `forbidden` → server-side block, fail-closed empty policy if missing. `ForbiddenTool` referenced in `tools_core.go`. | Confirm the fail-closed default path is implemented and tested (missing/corrupt manifest → status+check only). |
| 06 | **Handshake / Policy Digest** | `specd_handshake` listed; `HandshakeBootstrap` + policy-digest hash described. | Verify the digest actually hashes steering + config invariants + allowed tools, and host-detection (`agents.go` `HostStatus`) is wired. |
| 08 | **Context-manifest item *modes* (the ex-skills)** | The ADR reassigns skills → "item modes (R8.2)", but the new `embed_templates` ships no artifact for them. | Confirm Spec 08 R8.2 item-modes are implemented in `internal/context/manifest.go`, not just declared. This is where the old skill value must re-land — if it's missing, skills were *dropped*, not *redesigned*. |
| 09 | **Orchestration inertness** | Brain/Pinky verbs + `pinky-*` subagents exist; commands "compiled always, inert unless enabled." | Verify the gate is real (commands no-op / error cleanly when `orchestration.enabled=false`) and ACP file transport + lease/cost/time brakes are present, not stubs. |
| 06 | **AGENTS.md marker-merge** | ADR: `init` writes a marker-merged `AGENTS.md` (R6.3/R6.4). | Confirm idempotent marker-merge (re-running `init` doesn't duplicate blocks). |

**Highest-priority follow-up:** domain **08 item-modes**. The whole "we replaced skills
by design" claim rests on item-modes existing. If `internal/context/` has no item-mode
implementation, the skill capability was cut outright and that should be an explicit,
recorded decision rather than an implicit gap.

---

## 6. One-paragraph answer

The old specd told the agent who it was in three drifting dialects — an 8-role RBAC
table, a non-matching 7-card set, and ~12 injected skill packs — over a ~48-tool MCP
surface. We do **not** need any of those as separate mechanisms. The new build keeps the
four roles that have genuinely distinct authority (scout, craftsman, auditor, validator),
serves each from a single embedded card, enforces tool access through a deterministic
fail-closed policy instead of a role's prose, reconceives skills as context-manifest item
modes, and demotes brain/pinky from identities to opt-in verbs. Same capability, one
vocabulary, higher integrity — provided domain-08 item-modes are actually implemented,
which is the one thing to verify next.
