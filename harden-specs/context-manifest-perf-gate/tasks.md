# Tasks — Context-Manifest Measured Zero-Overhead Gate (A4)

## Wave 1 — Observability hook
- [x] T1 — Add invocation/read counters on manifest path
  - why: enable deterministic observation of disabled-mode work (Req 1)
  - role: builder
  - files: internal/context/ (manifest builder + test hook)
  - contract: expose a test-readable counter/spy for builder invocation and file
    reads without changing production behavior.
  - acceptance: counters readable in tests; no behavior change when enabled.
  - verify: go test ./internal/context/ -run Manifest
  - depends: —
  - requirements: 1

## Wave 2 — Measured guard
- [x] T2 — Allocation + invocation budget test
  - why: catch silent disabled-path regressions (Req 1,2)
  - role: verifier
  - files: internal/context/manifest_perf_test.go
  - contract: assert disabled-mode builder invocations == 0 and file reads == 0;
    bound allocs with `testing.AllocsPerRun` at documented budget.
  - acceptance: fails if disabled path does manifest work or allocates.
  - verify: go test ./internal/context/ -run "Perf|ZeroOverhead"
  - depends: T1
  - requirements: 1,2

## Wave 3 — Gate wiring
- [x] T3 — perf-gate target + CI wiring
  - why: regressions must block merge (Req 3)
  - role: builder
  - files: Makefile, .github/workflows/ (CI config)
  - contract: add `make perf-gate` running the focused test; invoke from CI.
  - acceptance: CI runs perf-gate; failing budget blocks merge.
  - verify: make perf-gate
  - depends: T2
  - requirements: 3
