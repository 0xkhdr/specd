# Tasks — Checkpoint Fault-Injection Stress (A3)

## Wave 1 — Injection hook
- [ ] T1 — Env-gated checkpoint fault hook
  - why: reproducible crash at the lease-clear→file-write window (Req 1)
  - role: builder
  - files: internal/ (checkpoint/RecordCheckpoint path), test-only gating
  - contract: honor `SPECD_FAULT_CHECKPOINT=after-lease-clear` to hard-exit at the
    window; no effect in normal runs.
  - acceptance: hook fires only when env set; production path unchanged.
  - verify: go test ./... -run Checkpoint
  - depends: —
  - requirements: 1

## Wave 2 — Stress script + invariants
- [ ] T2 — stress-checkpoint-fault.sh
  - why: exercise crash + resume across seeds (Req 1,2,3)
  - role: builder
  - files: scripts/stress-checkpoint-fault.sh
  - contract: loop launch→kill-at-injection→resume across seeds; assert no
    double-claim, no orphaned lease; print seed on failure.
  - acceptance: script red on injected double-claim; green on correct behavior.
  - verify: scripts/stress-checkpoint-fault.sh
  - depends: T1
  - requirements: 1,2,3

- [ ] T3 — Invariant assertions on on-disk state
  - why: prove single-owner + no stuck lease (Req 2,3)
  - role: verifier
  - files: internal/ resilience test helpers
  - contract: post-resume, assert exactly one checkpoint owner and lease
    reclaimed/expired; MaxSteps still bounds run.
  - acceptance: fails if two owners or permanently-held lease.
  - verify: go test ./... -run "DoubleClaim|OrphanLease"
  - depends: T2
  - requirements: 2,3

## Wave 3 — CI wiring
- [ ] T4 — Add fault-stress to CI
  - why: regressions block merge (Req 1)
  - role: builder
  - files: .github/workflows/ (CI config)
  - contract: add a third stress job running stress-checkpoint-fault.sh.
  - acceptance: CI runs the fault variant alongside existing two.
  - verify: N/A (CI run)
  - depends: T2
  - requirements: 1
