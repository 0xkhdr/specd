# Tasks — Rate-Limit Aware Lease (R3)

## Wave 1 — Lease state model
- [x] T1 — Add `ACPLeaseSuspended` + suspend metadata
  - why: lease must encode "away, will return" (Req 1)
  - role: builder
  - files: internal/core/acp_lease.go
  - contract: add `ACPLeaseSuspended` status (accept in validation) and omitempty
    `SuspendedAt`, `SuspendReason`, `ResumeDeadline`, cumulative-suspend tracking fields.
    Non-suspended leases serialize byte-identical.
  - acceptance: validation accepts new status; existing lease JSON round-trips unchanged.
  - verify: go test ./internal/core/ -run Lease
  - depends: —
  - requirements: 1

- [x] T2 — Config `resilience.maxSuspendSeconds`
  - why: cap suspension duration (Req 5)
  - role: builder
  - files: internal/core/specfiles.go, internal/core/embed_templates/config.json
  - contract: add `MaxSuspendSeconds` to shared `Resilience` block (default 600); validate
    `0 < v <= 3600`; byte-identical when absent.
  - acceptance: invalid value → load error; absent → no new bytes.
  - verify: go test ./internal/core/ -run "Config|Drift"
  - depends: —
  - requirements: 5

## Wave 2 — Lease operations
- [x] T3 — `SuspendLease` core op
  - why: extend lease without failing (Req 2)
  - role: builder
  - files: internal/core/acp_lease.go
  - contract: CAS-guarded; set status suspended, `ResumeDeadline = now + s + heartbeatBuffer`;
    enforce reason allowlist and cumulative cap; reject if caller lacks the lease.
  - acceptance: suspend extends deadline; over-cap rejected; bad reason rejected.
  - verify: go test ./internal/core/ -run Suspend
  - depends: T1, T2
  - requirements: 2

- [x] T4 — `ResumeLease` core op
  - why: returning worker keeps its task (Req 3)
  - role: builder
  - files: internal/core/acp_lease.go
  - contract: CAS-guarded; status→active, refresh heartbeat + normal lease window; if already
    reclaimed, error telling worker to re-claim; emit `resume` event with suspended duration.
  - acceptance: resume restores active lease; post-deadline resume errors clearly.
  - verify: go test ./internal/core/ -run Resume
  - depends: T3
  - requirements: 3

- [x] T5 — Reclaim predicate honors suspension
  - why: no false failure while suspended (Req 4)
  - role: builder
  - files: internal/core/acp_lease.go, internal/core/orchestration_sense.go
  - contract: expiry/reclaim uses `ResumeDeadline` for suspended leases; expose suspension on
    `OrchestrationLeaseSnapshot`; in-flight accounting counts suspended leases. Keep Decide pure.
  - acceptance: suspended-within-deadline never reclaimed/failed; past-deadline reclaims normally.
  - verify: go test ./internal/core/ -run "Reclaim|Lease|Determinism"
  - depends: T3
  - requirements: 4

## Wave 3 — CLI + test
- [x] T6 — `pinky suspend` / `pinky resume` commands
  - why: worker-facing entry points (Req 2,3)
  - role: builder
  - files: internal/cmd/pinky.go
  - contract: add `suspend` and `resume` cases mirroring `heartbeat` flag parsing; call core ops;
    support `--json`.
  - acceptance: suspend then resume keeps the same lease/task; flags validated.
  - verify: go test ./internal/cmd/ -run "PinkySuspend|PinkyResume"
  - depends: T4, T5
  - requirements: 2, 3

- [x] T7 — Rate-limit no-storm integration test
  - why: prove suspend prevents false retries (Req 4)
  - role: verifier
  - files: internal/cmd/brain_recovery_test.go
  - contract: dispatch → suspend(rate-limit) → advance clock under deadline → assert task still
    in-flight, no failure, no re-dispatch; then resume and complete.
  - acceptance: zero false failures while suspended; exactly one worker completes the task.
  - verify: go test ./internal/cmd/ -run Recovery
  - depends: T6
  - requirements: 1, 2, 3, 4
