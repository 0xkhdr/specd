# Tasks — Regression: State Backends (default/git/postgres/redis, locking, CAS)

## Wave 1
- [ ] T1 — Inventory backend implementations and conformance coverage
  - why: know which backends the conformance suite actually exercises (R1)
  - role: investigator
  - files: internal/core/backend.go, internal/core/backend_conformance_test.go
  - contract: list Backend methods and which impls the suite runs; mark gaps; do NOT edit
  - acceptance: a table of {method × backend} with covered/uncovered marks
  - verify: N/A
  - depends: —
  - requirements: 1, 2, 3, 4

## Wave 2
- [ ] T2 — Parameterize conformance over all four backends with graceful skips
  - why: R1 parity requires one suite over every impl, skipping absent services cleanly
  - role: builder
  - files: internal/core/backend_conformance_test.go
  - contract: drive default/git/postgres/redis from one table; env-gate PG/redis with t.Skip + reason; do NOT delete existing assertions
  - acceptance: R1.1-R1.3 pass; skipped backends print a reason; default+git always run
  - verify: go test ./internal/core/ -run Conformance -v
  - depends: T1
  - requirements: 1

- [ ] T3 — CAS + revision monotonicity tests across backends
  - why: R2 forbids lost updates on every backend
  - role: builder
  - files: internal/core/backend_conformance_test.go, internal/core/concurrency_test.go
  - contract: add stale-revision-rejection and concurrent-writer single-winner tests per available backend
  - acceptance: R2.1-R2.3 pass on each available backend
  - verify: go test ./internal/core/ -run 'Conformance|Concurren' -race
  - depends: T1
  - requirements: 2

- [ ] T4 — Lock recovery + durability/atomicity tests
  - why: R3, R4 guarantee no deadlock and no partial state
  - role: builder
  - files: internal/core/lock_test.go, internal/core/backend_conformance_test.go
  - contract: simulate dead holder recovery; assert interrupted write leaves prior-or-new state; git backend = one commit per write
  - acceptance: R3.1-R3.3 and R4.1-R4.3 pass; replay reconstructs git timeline
  - verify: go test ./internal/core/ -run 'Lock|Conformance|Replay'
  - depends: T1
  - requirements: 3, 4

## Wave 3
- [ ] T5 — Review backend parity and document service requirements
  - why: parity claims must be honest about what CI actually ran
  - role: reviewer
  - files: internal/core, TESTING.md
  - contract: review T2-T4 for silent skips masquerading as passes; confirm TESTING.md lists PG/redis setup; flag only
  - acceptance: every backend either runs or visibly skips with reason; no parity gap unrecorded
  - verify: go test ./internal/core/ -run Conformance -v
  - depends: T2, T3, T4
  - requirements: 1, 2, 3, 4
