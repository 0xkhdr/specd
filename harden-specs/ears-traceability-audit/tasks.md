# Tasks — EARS Requirement → Test Traceability Audit (P2-1)

Rolling: one spec per iteration, highest-risk first. Add a task row per spec
audited; do not batch.

## Wave 1 — Highest-risk specs
- [ ] T1 — Audit resilience specs (rolling, one spec per iteration)
  - why: most safety-critical surface (Req 1,2,3)
  - role: verifier
  - files: harden-specs/ears-traceability-audit/matrix.md, specs/resilience/**
  - contract: map every SHALL criterion in each resilience spec → test or
    UNPROVEN; file gaps as follow-up tasks.
  - acceptance: resilience matrix complete; gaps tracked.
  - verify: N/A (audit artifact)
  - depends: —
  - requirements: 1,2,3
  - progress:
    - [x] iter 1 — `checkpoint-protocol` (20/20 criteria mapped; gaps R6.1, R6.2
      closed with new tests in `orchestration_checkpoint_lifecycle_test.go`)
    - [ ] iter 2 — `auto-resume`
    - [ ] iter 3 — `rate-limit-lease`
    - [ ] iter 4 — `cross-spec-recovery`
    - [ ] iter 5 — `progress-weighted-waits`
    - [ ] iter 6 — `context-snapshot`

- [ ] T2 — Audit mcp + config specs
  - why: networked + precedence surfaces (Req 1,2,3)
  - role: verifier
  - files: matrix, specs/fusion/** (mcp), specs/config/**
  - contract: same mapping; gaps → tasks.
  - acceptance: mcp+config matrix complete; gaps tracked.
  - verify: N/A
  - depends: T1
  - requirements: 1,2,3

## Wave 2 — Remaining specs
- [ ] T3 — Audit commands + fusion remainder
  - why: complete the coverage (Req 1,2,3)
  - role: verifier
  - files: matrix, specs/commands/**, specs/fusion/**
  - contract: same mapping; gaps → tasks; mark each spec complete when mapped or
    waived.
  - acceptance: all specs audited; matrix index complete.
  - verify: N/A
  - depends: T2
  - requirements: 1,2,3
