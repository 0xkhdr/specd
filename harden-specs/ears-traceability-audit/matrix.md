# EARS Traceability Matrix

Rolling requirement→test audit (P2-1). One spec per iteration, highest-risk
first. Each row maps an EARS acceptance criterion to the test(s) that prove it,
or flags it `UNPROVEN` with a follow-up. A spec is "complete" only when every
criterion is mapped or explicitly waived.

## Audit log

| Iteration | Spec | Date | Criteria | Proven | Gaps closed |
|---|---|---|---|---|---|
| 1 | `resilience/checkpoint-protocol` | 2026-06-30 | 20 | 20 | R6.1, R6.2 (new tests) |

Remaining resilience specs (`auto-resume`, `context-snapshot`,
`cross-spec-recovery`, `progress-weighted-waits`, `rate-limit-lease`) and the
mcp/config/commands surfaces are **not yet audited** — scheduled for subsequent
iterations per the rolling cadence (do not batch).

---

## Iteration 1 — `specs/resilience/checkpoint-protocol`

All criteria proven. Two criteria (R6.1, R6.2) had no direct test at audit time;
both were closed this iteration with new tests in
`internal/core/orchestration_checkpoint_lifecycle_test.go`.

| Criterion | Summary | Covering test(s) | Status |
|---|---|---|---|
| R1.1 | `CheckpointRecord` fields defined | `TestCheckpointRecordCanonicalRoundTripStable`, `TestValidateCheckpointRecord` (core) | PROVEN |
| R1.2 | `ValidateCheckpointRecord` enforces IDs/attempt/progress/time | `TestValidateCheckpointRecord` (case table) | PROVEN |
| R1.3 | Canonical-JSON serialization is byte-stable | `TestCheckpointRecordCanonicalRoundTripStable` | PROVEN |
| R2.1 | `pinky checkpoint` persists record at deterministic path | `TestPinkyCheckpointPersistsAndReleases` (cmd), `TestRecordCheckpointReleasesLeaseAndEmitsEvent` (core) | PROVEN |
| R2.2 | Persist releases lease + emits `checkpoint` ACP event | `TestRecordCheckpointReleasesLeaseAndEmitsEvent` | PROVEN |
| R2.3 | No active lease → fail non-zero, persist nothing | `TestRecordCheckpointWithoutLeaseFails` | PROVEN |
| R2.4 | `--json` prints persisted record as canonical JSON | `TestPinkyCheckpointPersistsAndReleases` (unmarshals stdout) | PROVEN |
| R3.1 | `brain checkpoint` checkpoints every active lease | `TestBrainCheckpointForcesAllActive` | PROVEN |
| R3.2 | `--reason` recorded on each emitted checkpoint | `TestBrainCheckpointForcesAllActive` (asserts `Reason`) | PROVEN |
| R3.3 | No active workers → exit 0, reports nothing checkpointed | `TestBrainCheckpointNoActiveWorkers` | PROVEN |
| R4.1 | `OrchestrationResume` registered in `validOrchestrationAction` | `TestBrainResumeHappyPathNoDoubleDispatch`, decision-validation table | PROVEN |
| R4.2 | Checkpoint + no lease → emit `resume-from-checkpoint` not `dispatch` | `TestBrainCheckpointResumeNoWorkRedone`, `TestBrainResumeHappyPathNoDoubleDispatch` | PROVEN |
| R4.3 | Checkpoint surfaced via `SenseOrchestration`, `Decide` stays pure | `TestBrainCheckpointResumeNoWorkRedone` (`snap.Checkpoints`), `TestOrchestrationDecideTable` (pure decide) | PROVEN |
| R4.4 | New action validates with no `Artifact` requirement | `TestValidateOrchestrationSnapshotBranches` / decision-validation | PROVEN |
| R5.1 | Resume mission carries manifest/notes/changed/head/progress | `TestBrainCheckpointResumeNoWorkRedone` (`Resume.ProgressPercent==70`) | PROVEN |
| R5.2 | Brief prefixed with explicit resume instruction | `TestBrainCheckpointResumeNoWorkRedone` (brief contains "Resuming from checkpoint") | PROVEN |
| R5.3 | No checkpoint → brief byte-stable vs today | `TestPinkyMissionDeterministic` | PROVEN |
| R6.1 | Completed task's checkpoint deleted/archived (no re-resume) | `TestCleanupCheckpointRemovesAllAttempts` *(added this iteration)* | PROVEN |
| R6.2 | Checkpoint older than current attempt ignored when sensing | `TestHasResumableCheckpointAttemptGuard` *(added this iteration)* | PROVEN |
| R6.3 | Feature gated behind `checkpointEnabled` (default false), byte-identical when off | `TestPinkyCheckpointDisabledNoop`, `TestResilienceConfigByteStable` | PROVEN |

### Gaps found and closed

- **R6.1** — `CleanupCheckpoint` existed and was wired into the completion path
  (`pinky_report.go`) but had no direct test. Added
  `TestCleanupCheckpointRemovesAllAttempts`: removes every attempt of a task,
  leaves sibling tasks untouched, and is best-effort/idempotent on a missing dir.
- **R6.2** — `hasResumableCheckpoint` enforced the `(taskID, attempt)` match in
  `orchestration_decide.go` but no test exercised the stale-attempt rejection.
  Added `TestHasResumableCheckpointAttemptGuard`: an older-attempt checkpoint is
  not resumable; only the current attempt is; a different task never matches.
