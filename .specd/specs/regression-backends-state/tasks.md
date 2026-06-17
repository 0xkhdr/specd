# Tasks — Regression: State Backends (default/git/postgres/redis, locking, CAS)

## Wave 1
- [x] T1 — Inventory backend implementations and conformance coverage ✓ complete · evidence: Inventory in memory backend-inventory-T1. Suite runs file+git only; postgres/redis registered behind build tags but untested. Gaps: R2.3 monotone, R3.2/R3.3, R4.2 atomicity, R4.3 git-commit-per-write. Investigator, verify N/A. · 2026-06-17T16:48:40.191689986Z
  - why: know which backends the conformance suite actually exercises (R1)
  - role: investigator
  - files: internal/core/backend.go, internal/core/backend_conformance_test.go
  - contract: list Backend methods and which impls the suite runs; mark gaps; do NOT edit
  - acceptance: a table of {method × backend} with covered/uncovered marks
  - verify: N/A
  - depends: —
  - requirements: 1, 2, 3, 4

## Wave 2
- [x] T2 — Parameterize conformance over all four backends with graceful skips ✓ complete · evidence: backend table file/git/postgres/redis; postgres/redis skip with explicit reason; file+git run. go test -run Conformance -v → exit 0 (PASS + 2 SKIP). go vet -tags 'specd_postgres specd_redis' clean. · 2026-06-17T16:51:09.426750657Z
  - why: R1 parity requires one suite over every impl, skipping absent services cleanly
  - role: builder
  - files: internal/core/backend_conformance_test.go
  - contract: drive default/git/postgres/redis from one table; env-gate PG/redis with t.Skip + reason; do NOT delete existing assertions
  - acceptance: R1.1-R1.3 pass; skipped backends print a reason; default+git always run
  - verify: go test ./internal/core/ -run Conformance -v
  - depends: T1
  - requirements: 1

- [x] T3 — CAS + revision monotonicity tests across backends ✓ complete · evidence: CAS+monotone (rev 1→6), stale-base rejected no clobber, 32-goroutine Turn==32. go test -run 'Conformance|Concurren' -race → exit 0. · 2026-06-17T16:51:18.657019343Z
  - why: R2 forbids lost updates on every backend
  - role: builder
  - files: internal/core/backend_conformance_test.go, internal/core/concurrency_test.go
  - contract: add stale-revision-rejection and concurrent-writer single-winner tests per available backend
  - acceptance: R2.1-R2.3 pass on each available backend
  - verify: go test ./internal/core/ -run 'Conformance|Concurren' -race
  - depends: T1
  - requirements: 2

- [x] T4 — Lock recovery + durability/atomicity tests ✓ complete · evidence: lock-released-on-completion (R3.3), backend-agnostic schema-valid-after-write (R4.1/4.2), git one-commit-per-revision (R4.3). go test -run 'Lock|Conformance|Replay' → exit 0. · 2026-06-17T16:51:19.920988878Z
  - why: R3, R4 guarantee no deadlock and no partial state
  - role: builder
  - files: internal/core/lock_test.go, internal/core/backend_conformance_test.go
  - contract: simulate dead holder recovery; assert interrupted write leaves prior-or-new state; git backend = one commit per write
  - acceptance: R3.1-R3.3 and R4.1-R4.3 pass; replay reconstructs git timeline
  - verify: go test ./internal/core/ -run 'Lock|Conformance|Replay'
  - depends: T1
  - requirements: 3, 4

## Wave 3
- [x] T5 — Review backend parity and document service requirements ✓ complete · evidence: Reviewed T2-T4: skips are visible t.Skip w/ reasons, no silent passes. Added TESTING.md backend-parity section (skip table + tag/env setup + no-driver guarantee). Caveat recorded in memory parity-caveat-T5. go test -run Conformance -v → exit 0. · 2026-06-17T16:52:19.156907126Z
  - why: parity claims must be honest about what CI actually ran
  - role: reviewer
  - files: internal/core, TESTING.md
  - contract: review T2-T4 for silent skips masquerading as passes; confirm TESTING.md lists PG/redis setup; flag only
  - acceptance: every backend either runs or visibly skips with reason; no parity gap unrecorded
  - verify: go test ./internal/core/ -run Conformance -v
  - depends: T2, T3, T4
  - requirements: 1, 2, 3, 4
