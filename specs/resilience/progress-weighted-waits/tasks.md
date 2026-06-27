# Tasks — Progress-Weighted MaxWaits (R6)

## Wave 1 — Progress timestamp
- [x] T1 — Stamp `LastReport` on progress
  - why: driver needs report recency (Req 1)
  - role: builder
  - files: internal/core/pinky_report.go
  - contract: persist a server-side `LastReport` (RFC3339, omitempty) when `RecordPinkyProgress`
    writes; do not trust a worker-supplied time. Existing output byte-stable but for the field.
  - acceptance: progress write records server time; old records without the field still load.
  - verify: go test ./internal/core/ -run Progress
  - depends: —
  - requirements: 1

- [x] T2 — Config `resilience.progressTimeoutSeconds`
  - why: tunable window, default-off-equivalent (Req 3)
  - role: builder
  - files: internal/core/specfiles.go, internal/core/embed_templates/config.json
  - contract: add `ProgressTimeoutSeconds` (default 300) to shared `Resilience` block; validate
    `0 < v <= 3600`; 0/unset → today's behavior; byte-identical config when absent.
  - acceptance: invalid → load error; absent → no new bytes.
  - verify: go test ./internal/core/ -run "Config|Drift"
  - depends: —
  - requirements: 3

## Wave 2 — Sense + driver
- [x] T3 — Surface `MostRecentProgressAt` in snapshot
  - why: keep wait decision derived from sensed state (Req 2.4)
  - role: builder
  - files: internal/core/orchestration_sense.go, internal/core/orchestration.go
  - contract: add `MostRecentProgressAt` (omitempty) to `OrchestrationSnapshot`; populate from
    in-flight workers' `LastReport` in `SenseOrchestration`.
  - acceptance: snapshot carries newest in-flight report time; empty when none in-flight.
  - verify: go test ./internal/core/ -run "Sense|Determinism"
  - depends: T1
  - requirements: 2

- [x] T4 — Progress-weighted wait logic
  - why: don't stall slow-but-progressing work (Req 2)
  - role: builder
  - files: internal/core/orchestration_driver.go
  - contract: add `waitWeight` using `MostRecentProgressAt` vs `progressTimeoutSeconds`; skip the
    `waits++` when within window; keep `MaxSteps` hard ceiling and existing `waits=0` resets.
  - acceptance: progressing worker never trips `DriverStalled` within window; no-progress still
    stalls; MaxSteps still bounds.
  - verify: go test ./internal/core/ -run "Driver|Stall"
  - depends: T2, T3
  - requirements: 2

## Wave 3 — Test
- [x] T5 — Long-task no-false-stall test
  - why: prove R6 fix (Req 2)
  - role: verifier
  - files: internal/cmd/brain_driver_cov_test.go
  - contract: simulate a worker reporting progress every few waits over more than `MaxWaits`
    steps; assert outcome is not `DriverStalled`; then stop reporting and assert it stalls.
  - acceptance: test green; both branches covered.
  - verify: go test ./internal/cmd/ -run Driver
  - depends: T4
  - requirements: 1, 2, 3
