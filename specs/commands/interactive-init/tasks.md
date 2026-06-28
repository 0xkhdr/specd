# Tasks — Interactive `/init` Command Wrapper

## Wave 1 — Command model and probing
- [ ] T1 — Define `/init` option model
  - why: one canonical model keeps shell/Python behavior aligned (Req 1,2,3)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py, docs or README as needed
  - contract: model includes agent, dry-run, repair, refresh, json, orchestration policy, workers, retries, timeout, cost, role mode, sandbox, yes/non-interactive.
  - acceptance: invalid option returns exit 2 with usage; no prompts when `--non-interactive` or stdin is not TTY.
  - verify: shellcheck scripts/specd-workflow.sh && python3 -m py_compile scripts/specd-workflow.py
  - depends: none
  - requirements: 1,2,3

- [ ] T2 — Implement host detection probe
  - why: `/init` should present detected hosts when possible (Req 1)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: probe `specd doctor --json` first, tolerate command/JSON failure, return known host fallback list.
  - acceptance: mocked doctor JSON yields detected hosts; malformed JSON yields fallback without crash.
  - verify: go test ./... or wrapper-specific tests chosen by implementer
  - depends: T1
  - requirements: 1

## Wave 2 — Interactive and non-interactive execution
- [ ] T3 — Build interactive menu flow
  - why: action plan requires explicit behavior selection (Req 1,2)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: prompt for agent, orchestration policy, workers/retries/timeout/cost/mode/sandbox only when needed.
  - acceptance: menu choices map deterministically to native flags; invalid menu choices re-prompt or return 2.
  - verify: wrapper tests with stdin fixtures
  - depends: T2
  - requirements: 1,2

- [ ] T4 — Implement native `specd init` delegation
  - why: wrapper must not own scaffold policy (Req 3)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: construct argv without `eval`; pass through dry-run/repair/refresh/json; propagate exit code.
  - acceptance: `--dry-run --agent none --orchestration none` invokes only native dry-run and exits with native code.
  - verify: integration test in temp dir with fake `specd` on PATH
  - depends: T3
  - requirements: 3,4

## Wave 3 — Hardening and documentation
- [ ] T5 — Add wrapper tests
  - why: slash command should remain stable across CLI changes (Req 5)
  - role: builder
  - files: scripts/*test*, internal/cmd tests if wrapper embedded
  - contract: tests cover agent fallback, non-TTY behavior, dry-run, orchestration flags, command failure propagation.
  - acceptance: tests deterministic, no network, no real global config writes.
  - verify: make test or project-selected wrapper test command
  - depends: T4
  - requirements: 5

- [ ] T6 — Document `/init`
  - why: users need safe examples (Req 5)
  - role: builder
  - files: scripts/README.md, AGENTS.md or shipped skill docs if applicable
  - contract: docs include interactive use, CI/non-interactive use, dry-run, repair/refresh, orchestration examples.
  - acceptance: examples match implemented flags and native command mapping.
  - verify: markdown lint if available; manual command copy check
  - depends: T4
  - requirements: 5
