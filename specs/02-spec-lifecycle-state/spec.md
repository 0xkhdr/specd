# Spec 02 — Spec Lifecycle & State Model

> **Authoring order:** 3 / 12 · **Critical path:** yes
> **Sources:** `fresh-start/02-spec-lifecycle-state.md`, paper pp.28–29
> **ADRs:** ADR-1, ADR-2, ADR-6, ADR-7, ADR-8
> **Reference:** `reference/internal/core/{state,phases,task_complete}.go`, `reference/internal/cmd/{new,approve,status}.go`

The state spine. Every other domain reads or writes it. A spec moves forward through
Requirements → Design → Tasks → Executing → Verifying → Complete; every semantic transition is
an explicit, human-approvable event with an on-disk record.

---

## 1. Purpose & principles
- **Principles owned:** P2 (Specs as the Source of Truth), P6 (Human Gates at Phase
  Boundaries). Underpins P3, P7.
- **Paper concept:** *instructions/context* + the "Configuring the Harness" phase (pp.28–29).
  The plan lives as versioned Markdown on disk; machine truth lives in `state.json`; the
  harness never holds the plan in a context window.

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| `state.json` machine truth + CAS + atomic write + lock discipline | **KEEP** (hard) | `reference/internal/core/state.go`; ADR-8; contributor-guide inv#2–#4 |
| Forward-only phase ratchet (`PlanningAdvance`) | **KEEP** | P6. `reference/internal/core/phases.go` |
| Pure readiness gates (`PhaseReadiness`) | **KEEP** | Feeds Spec 03 |
| Fat `State` with per-flywheel record structs | **SIMPLIFY** → `records` extension map | ADR-6; strips ~9 structs |
| Three execution modes (`Simple/Orchestrated/Conductor`) | **REDESIGN** → two (`simple`/`orchestrated`) | ADR-7; drops analytics `Conductor` |
| `migrate` / schema migration | **CUT** from MVP; reset `SchemaVersion: 1` | ADR-2 |

**Minimal accurate surface:** commands `new`, `approve`, `status`, `midreq`, `decision` (all
Tier 1); modules `state.go` (thin), `phases.go`, `task_complete.go`, `lock.go`, `io.go`;
on-disk `.specd/specs/<slug>/{requirements.md,design.md,tasks.md,state.json,.lock}`.

## 3. Requirements (EARS)
- **R2.1** When a spec is created, the system shall write `state.json` with `schemaVersion: 1`,
  `revision: 0`, `mode: "simple"`, and status `requirements`.
- **R2.2** When any command mutates spec state, the system shall hold the reentrant per-spec
  lock for the whole read-modify-write and shall persist via compare-and-swap on `revision`,
  refusing the write if the on-disk revision changed.
- **R2.3** When a persisted write is attempted, the system shall write atomically (temp file →
  fsync → rename) so a partial write can never replace the target.
- **R2.4** When `approve` is invoked on a spec whose current phase readiness gates do not pass,
  the system shall refuse the transition and report the failing gates.
- **R2.5** The system shall advance status only along the forward-only `PlanningAdvance` map
  and shall reject any backward or skipping transition.
- **R2.6** When `state.json` is corrupt, newer-schema, or has an invalid status, the system
  shall fail loud with a gate error rather than coerce it.
- **R2.7** When a plugin attaches evidence, the system shall store it under
  `state.records[<key>]` without altering core fields, validating only that it is valid JSON.
- **R2.8** When `mode` changes after creation, the system shall permit it only via an
  auditable `approve --mode` transition.

## 4. Design

### Module boundaries
- `state.go` — types + CAS/atomic persistence. `phases.go` — pure ratchet + readiness.
- `task_complete.go` — evidence-integrity completion (shared with Spec 05).
- `lock.go` / `io.go` — primitives from Spec 10.

### Key types
- `State{ SchemaVersion, Revision, Mode, Status, Phase, Tasks[], Decisions[], MidReqGates[],
  Records map[string]json.RawMessage }`.
- `TaskState`; `VerificationRecord` (thin — defined in Spec 05).
- `Mode` enum: `simple` (paper's *conductor*: human-in-the-loop) | `orchestrated` (paper's
  *orchestrator*: async delegation). Set at `new --mode`, default `simple`.

### On-disk contracts
- `state.json` (0644); forward-only status; `revision` monotonic; injectable `Clock`.
- Optional/plugin evidence under `state.records[<key>]` (ADR-6); core fields all `omitempty`
  so a Simple spec that never opts into orchestration keeps a byte-identical `state.json`.

### External interfaces
- `LoadState` / `SaveState`; `PhaseReadiness`; `CompleteTask`.

## 5. Invariants preserved (ADR-8)
CAS on revision; SaveState must-be-locked (`assertLocked` test-panics if not); atomic write;
loud-load; forward-only ratchet.

## 6. Cross-domain dependencies
- Consumed by: Spec 03 (`PhaseReadiness`), Spec 05 (writes `VerificationRecord`), Spec 09
  (reads `mode` for orchestration eligibility + CAS), Spec 11 (state projection).
- Depends on: Spec 10 (lock/io/CAS primitives), Spec 01 (charter).

## 7. Risks & open questions
- **Risk:** `Records` becomes an untyped junk drawer. → each plugin owns a documented schema;
  core validates only valid JSON.
- **Resolved (ADR-7):** `mode` is mutable mid-flight, but only via auditable `approve --mode`.
