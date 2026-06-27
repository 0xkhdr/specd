# Resilience Program — Progress & Wave Plan

Source plan: [`specd-resilience-analysis-and-action-plan.md`](../../specd-resilience-analysis-and-action-plan.md)

This program hardens the Brain/Pinky orchestration layer so a host crash, token-limit
exhaustion, or provider rate-limit becomes a non-event rather than a bottleneck. Each
child spec below has its own `spec.md` (requirements + design) and `tasks.md` (wave DAG).

## Gap → Spec map

| Gap | Title | Spec | Priority |
|---|---|---|---|
| R1, R4 | Proactive checkpointing + worker hibernate/thaw | [checkpoint-protocol](checkpoint-protocol/spec.md) | P0 |
| R5 | Zero-intervention session resumption on host restart | [auto-resume](auto-resume/spec.md) | P0 |
| R3 | Distinguish rate-limited workers from dead workers | [rate-limit-lease](rate-limit-lease/spec.md) | P1 |
| R2 | O(changed-files) re-contextualization on resume | [context-snapshot](context-snapshot/spec.md) | P1 |
| R6 | No false `stalled` on slow-but-progressing workers | [progress-weighted-waits](progress-weighted-waits/spec.md) | P2 |
| — | Resume whole program DAG, not one spec | [cross-spec-recovery](cross-spec-recovery/spec.md) | P2 |

## Program waves

The program runs in three waves. A wave starts only when the prior wave's specs are
`complete`. Cross-wave dependencies are noted; intra-spec waves live in each `tasks.md`.

### Wave 1 — Foundation (P0) — **status: complete**

| Spec | Status | Depends on | Notes |
|---|---|---|---|
| checkpoint-protocol | complete | — | All T1–T10 landed: `CheckpointRecord`+validator, path helpers, `RecordCheckpoint` (clears the lease so the same attempt is re-claimable), `pinky checkpoint` + `brain checkpoint` CLI, `resume-from-checkpoint` action (sense populates `Checkpoints`, decide prefers resume with strict attempt-guard), resume mission brief with "do not restart" header, cleanup-on-completion, `resilience.checkpointEnabled` gate, and the checkpoint→resume e2e test. |
| auto-resume | complete | — | All T1–T7 landed: `ListResumableSessions`, `resilience.autoResume` config, `brain resume --list [--max-age-minutes]` discovery (no-flag lifecycle resume unchanged), MCP `brain_resume` (list when `session` omitted, resume when present), AGENTS.md + `docs/agent-integration.md` startup contract, and the list filter/order/exclude integration test. |

### Wave 2 — Graceful degradation (P1) — **status: complete**

| Spec | Status | Depends on | Notes |
|---|---|---|---|
| rate-limit-lease | complete | checkpoint-protocol (shares lease-state extension) | All T1–T7 landed: `ACPLeaseSuspended` status + omitempty suspend metadata (`SuspendedAt`/`SuspendReason`/`ResumeDeadline`/`SuspendSecondsTotal`), `resilience.maxSuspendSeconds` (default 600, validated `(0,3600]`), `SuspendLease`/`ResumeLease` CAS ops (reason allowlist + cumulative cap), reclaim predicate honors `ResumeDeadline` (suspended-within-window counts in-flight via `OrchestrationLeaseSnapshot.Suspended`, Decide stays pure), `pinky suspend`/`pinky resume` CLI + `resume` ACP event, and the no-retry-storm integration test. |
| context-snapshot | complete | checkpoint-protocol (snapshot referenced by `CheckpointRecord.ContextManifest`) | All T1–T7 landed: `ContextSnapshot`+`LoadedFile` with canonical JSON + validator, file/steering/memory SHA256 digest helpers, `ContextSnapshotDir`/`ContextSnapshotPath` + `resilience.contextSnapshotEnabled` gate, `specd context --snapshot [--out]` (plain output byte-unchanged), `DiffContextSnapshot` comparator, resume brief renders reference/reload delta (guarded, no-op without snapshot), and the emit→edit→delta test. |

### Wave 3 — Hardening (P2) — **status: complete**

| Spec | Status | Depends on | Notes |
|---|---|---|---|
| progress-weighted-waits | complete | — | All T1–T5 landed: server-stamped `lastReport` on progress payloads, `resilience.progressTimeoutSeconds` config validation, sensed `mostRecentProgressAt` from in-flight worker progress, progress-weighted driver waits with `MaxSteps` still hard-bounding chatty workers, and the no-false-stall regression test. |
| cross-spec-recovery | complete | auto-resume | All T1–T6 landed: `ProgramState` canonical JSON + validation, `program-state.json` path/write helpers, per-step frontier persistence, `brain resume --program --session <parent>` driver reuse, resumable discovery `program:true` + child counts, and the crash-recovery/idempotency integration test. |

## Status legend

`not-started` → `in-progress` → `verifying` → `complete` / `blocked`

## How to track

Each child `tasks.md` owns its checkbox DAG (flip with `specd task`, never by hand).
Update the per-wave status tables above as child specs advance. The program is
`complete` when all six child specs are `complete` and the Wave-4 hardening items in
the source plan (stress tests, `doctor --resilience`, AGENTS.md docs) are landed.

## Open program-level decisions

- **Config shape:** all resilience knobs live under `orchestration.resilience` (see
  `OrchestrationCfg` in `internal/core/specfiles.go`). Defaults must keep existing
  `config.json` byte-identical (every new field `omitempty`).
- **Runtime layout:** new on-disk state lives under
  `.specd/runtime/sessions/<session>/` beside `events/`, `workers/`, `artifacts/`
  (see `ACPRuntimePaths` in `internal/core/runtime_paths.go`). Add `checkpoints/`,
  `context-snapshots/`, and a program `program-state.json`.
- **Determinism:** every new Brain decision path must stay a pure function of
  `(snapshot, policy)` — checkpoint/suspend state enters via `SenseOrchestration`,
  not via clock reads inside `DecideOrchestration`.
