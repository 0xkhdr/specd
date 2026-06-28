# Tasks — `/spec` Workflow Dashboard

## Wave 1 — Listing and selection
- [x] T1 — Implement spec root guard and list parser
  - why: dashboard needs reliable orientation (Req 1)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: require native `specd status`; parse JSON when possible, text fallback otherwise.
  - acceptance: no `.specd` returns 3; empty repo prints `/spec new` guidance.
  - verify: wrapper tests with fake `specd`
  - depends: none
  - requirements: 1

- [x] T2 — Implement slug selection helper
  - why: continue/check actions need safe target choice (Req 3,6)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: auto-select exactly one spec; prompt/list for many; never guess among many.
  - acceptance: fixtures cover zero/one/many specs.
  - verify: wrapper tests
  - depends: T1
  - requirements: 3,6

## Wave 2 — Lifecycle actions
- [ ] T3 — Implement `/spec new`
  - why: creation is primary entry point (Req 2)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: delegate to `specd new`; support title and orchestrated flag; print next steps after success.
  - acceptance: fake `specd` receives expected argv; native nonzero propagates.
  - verify: wrapper tests
  - depends: T1
  - requirements: 2

- [ ] T4 — Implement `/spec continue`
  - why: users need next-action guidance (Req 3)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: run `specd context`; inspect status; in executing phase run `specd next`; suggest waves for blockers.
  - acceptance: planning/executing/verifying/complete fixtures print correct guidance.
  - verify: wrapper tests
  - depends: T2
  - requirements: 3

- [ ] T5 — Implement direct delegation actions
  - why: dashboard should centralize common lifecycle commands (Req 4)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: implement check, approve, context, next, waves, report as direct native calls.
  - acceptance: argv and exit codes match native calls.
  - verify: wrapper tests
  - depends: T2
  - requirements: 4

## Wave 3 — Mode awareness, hardening, docs
- [ ] T6 — Implement safe `/spec mode`
  - why: action plan mentions mode but native support may vary (Req 5)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: detect command through `specd help --json`/help text; delegate only if supported; otherwise explain fallback.
  - acceptance: unsupported fake specd does not edit files and exits 1 or 2 per usage design.
  - verify: wrapper tests
  - depends: T1
  - requirements: 5

- [ ] T7 — Add safety regression tests
  - why: wrapper must not weaken evidence gate (Req 6)
  - role: reviewer
  - files: wrapper tests, scripts/README.md
  - contract: assert no action auto-runs `specd task --status complete`; docs show verify before complete.
  - acceptance: tests fail if complete is invoked by continue/next.
  - verify: wrapper test suite
  - depends: T4,T5
  - requirements: 6

- [ ] T8 — Document `/spec`
  - why: users need clear lifecycle examples (Req 1-6)
  - role: builder
  - files: scripts/README.md, AGENTS.md or skill docs if shipped
  - contract: include new/list/continue/check/approve/next/report/mode behavior and evidence warning.
  - acceptance: docs match implemented flags and outputs.
  - verify: markdown lint/manual check
  - depends: T6,T7
  - requirements: 1,2,3,4,5,6
