# Tasks — Checkpoint Protocol (R1 + R4)

## Wave 1 — Data model & storage
- [x] T1 — Add `CheckpointRecord` type + validator
  - why: foundation for all checkpoint persistence (Req 1)
  - role: builder
  - files: internal/core/orchestration.go
  - contract: add `CheckpointRecord` struct and `ValidateCheckpointRecord`; wire into
    `normalizeOrchestrationValue` for canonical JSON. Do NOT touch existing action consts yet.
  - acceptance: `ValidateCheckpointRecord` rejects bad IDs, attempt<1, progress out of [0,100];
    canonical JSON round-trips byte-stable.
  - verify: go test ./internal/core/ -run Checkpoint
  - depends: —
  - requirements: 1

- [x] T2 — Add checkpoint path helpers
  - why: deterministic on-disk location (Req 2.1)
  - role: builder
  - files: internal/core/runtime_paths.go
  - contract: add `CheckpointDir(sessionID)` and `CheckpointPath(sessionID, task, attempt)`
    using `sessionJoin`; validate segments. No behavior change to existing helpers.
  - acceptance: paths resolve under `sessions/<id>/checkpoints/<task>-<attempt>.json`; reject
    traversal in IDs.
  - verify: go test ./internal/core/ -run RuntimePaths
  - depends: —
  - requirements: 2

- [x] T3 — `RecordCheckpoint` core function
  - why: write + lease release + event in one atomic-ish op (Req 2.2)
  - role: builder
  - files: internal/core/orchestration_checkpoint.go (new), internal/core/acp_lease.go
  - contract: mirror `RecordPinkyProgress`; validate, write canonically, release the caller's
    lease, append a `checkpoint` ACP event. Fail if caller holds no active lease (Req 2.3).
  - acceptance: writing a checkpoint releases the lease and creates a `checkpoint` event;
    no-lease caller errors with nothing written.
  - verify: go test ./internal/core/ -run Checkpoint
  - depends: T1, T2
  - requirements: 2, 3

## Wave 2 — CLI surface
- [x] T4 — `specd pinky checkpoint` command
  - why: worker-facing entry point (Req 2)
  - role: builder
  - files: internal/cmd/pinky.go
  - contract: add `checkpoint` case parsing the documented flags; call `RecordCheckpoint`;
    support `--json`. Reuse existing flag-parse helpers in pinky.go.
  - acceptance: command persists record, releases lease, prints canonical JSON under `--json`.
  - verify: go test ./internal/cmd/ -run PinkyCheckpoint
  - depends: T3
  - requirements: 2

- [x] T5 — `specd brain checkpoint` force-all command
  - why: host can checkpoint every worker before /clear (Req 3)
  - role: builder
  - files: internal/cmd/brain.go, internal/cmd/brain_commands.go
  - contract: add `checkpoint` case; iterate active leases; request a checkpoint per worker
    with `--reason`. Exit 0 with message when none active.
  - acceptance: with N active leases, N checkpoint events recorded each carrying the reason.
  - verify: go test ./internal/cmd/ -run BrainCheckpoint
  - depends: T3
  - requirements: 3

## Wave 3 — Brain decision & mission
- [x] T6 — `resume-from-checkpoint` action + sense
  - why: Brain prefers resume over fresh dispatch (Req 4)
  - role: builder
  - files: internal/core/orchestration.go, internal/core/orchestration_sense.go,
    internal/core/orchestration_decide.go
  - contract: add `OrchestrationResume` const + register in `validOrchestrationAction`; add
    `Checkpoints` field (omitempty) to `OrchestrationSnapshot`; populate it in
    `SenseOrchestration`; in `DecideOrchestration` emit resume when a runnable task has a
    matching checkpoint and no active lease. Keep Decide pure. Honor attempt-guard (Req 6.2).
  - acceptance: snapshot with a valid checkpoint + no lease yields `resume-from-checkpoint`;
    stale-attempt checkpoint is ignored; empty case byte-identical to today.
  - verify: go test ./internal/core/ -run "Resume|Decide|Determinism"
  - depends: T1
  - requirements: 4, 6

- [x] T7 — Resume mission brief
  - why: resuming worker gets prior progress (Req 5)
  - role: builder
  - files: internal/core/pinky.go, internal/core/pinky_brief.go
  - contract: when decision is resume, thread checkpoint manifest/notes/changed-files/git-head
    into the mission and prepend the resume instruction header. Non-resume path byte-stable.
  - acceptance: resume brief contains "resuming … do not restart" + working notes; non-resume
    brief unchanged (golden test).
  - verify: go test ./internal/cmd/ -run Brief
  - depends: T6
  - requirements: 5

## Wave 4 — Lifecycle, config, tests
- [x] T8 — Cleanup on completion + attempt guard
  - why: prevent resurrection of finished work (Req 6)
  - role: builder
  - files: internal/core/orchestration_checkpoint.go, internal/core/acp_lease.go
  - contract: on task verified-complete, delete/archive its checkpoint; ignore stale-attempt
    checkpoints when sensing.
  - acceptance: completing a task removes its checkpoint; older-attempt files never resume.
  - verify: go test ./internal/core/ -run Checkpoint
  - depends: T6
  - requirements: 6

- [x] T9 — Config gate `resilience.checkpointEnabled`
  - why: default-off, byte-stable config (Req 6.3)
  - role: builder
  - files: internal/core/specfiles.go, internal/core/embed_templates/config.json
  - contract: add `Resilience` block to `OrchestrationCfg` with `CheckpointEnabled bool`
    (omitempty); gate sense/decide/commands on it. Existing config.json stays byte-identical
    when block absent.
  - acceptance: disabled → no checkpoint behavior, no new bytes; enabled → feature active.
  - verify: go test ./internal/core/ -run "Config|Drift"
  - depends: T6
  - requirements: 6

- [x] T10 — End-to-end checkpoint→resume test
  - why: prove the full loop (all reqs)
  - role: verifier
  - files: internal/cmd/brain_recovery_test.go
  - contract: simulate dispatch → progress 70% → pinky checkpoint → brain step yields
    resume-from-checkpoint → fresh worker brief carries notes. Assert no work re-done.
  - acceptance: test green; lease released and re-issued exactly once.
  - verify: go test ./internal/cmd/ -run Recovery
  - depends: T7, T8, T9
  - requirements: 1, 2, 3, 4, 5, 6
