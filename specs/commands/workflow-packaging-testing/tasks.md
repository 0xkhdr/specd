# Tasks — Slash Workflow Packaging, Tests, and Documentation

## Wave 1 — Shared command pack structure
- [x] T1 — Create unified shell command pack
  - why: users should source one file for all slash commands (Req 1)
  - role: builder
  - files: scripts/specd-workflow.sh
  - contract: expose `/init`, `/steer`, `/spec`, `/pinky-brain` where supported plus portable `specd_workflow <cmd>` entry.
  - acceptance: sourcing file defines commands; nested-dir invocation works through shared root discovery.
  - verify: shellcheck scripts/specd-workflow.sh && wrapper tests
  - depends: interactive-init/T1, steering-console/T1, spec-dashboard/T1, pinky-brain-console/T1
  - requirements: 1

- [x] T2 — Create unified Python CLI
  - why: cross-platform fallback required by action plan (Req 1)
  - role: builder
  - files: scripts/specd-workflow.py
  - contract: argparse subcommands mirror slash actions; Python stdlib only; returns native-compatible exit codes.
  - acceptance: `python3 scripts/specd-workflow.py --help` and subcommand helps work.
  - verify: python3 -m py_compile scripts/specd-workflow.py && wrapper tests
  - depends: interactive-init/T1, steering-console/T1, spec-dashboard/T1, pinky-brain-console/T1
  - requirements: 1

- [x] T3 — Centralize shared helpers
  - why: avoid shell/Python behavior drift (Req 1,4)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: helpers cover native command execution, JSON probe fallback, root discovery, command availability, exit propagation.
  - acceptance: parity tests show same argv for equivalent shell/Python commands.
  - verify: wrapper tests
  - depends: T1,T2
  - requirements: 1,4

## Wave 2 — Test harness
- [ ] T4 — Add fake `specd` fixture harness
  - why: tests need deterministic native responses (Req 3)
  - role: builder
  - files: scripts/testdata or internal test fixture location, wrapper test files
  - contract: fake binary captures argv, emits configurable JSON/text, returns configured exit codes.
  - acceptance: tests can simulate status, doctor, brain, failures without real repo/global writes.
  - verify: wrapper test command
  - depends: T3
  - requirements: 3

- [ ] T5 — Add safety invariant tests
  - why: wrappers must not weaken specd gates (Req 4)
  - role: reviewer
  - files: wrapper test files
  - contract: assert no direct writes to `state.json`, no checkbox mutation, no auto `task --status complete`, no forged `pinky report`.
  - acceptance: mutation traps fail if wrappers touch forbidden files/commands.
  - verify: wrapper test command
  - depends: T4, spec-dashboard/T7, pinky-brain-console/T8
  - requirements: 4

- [ ] T6 — Add parity and failure propagation tests
  - why: shell/Python need matching behavior and native exit correctness (Req 3,4)
  - role: builder
  - files: wrapper test files
  - contract: compare key command argv across implementations; simulate native exits 0/1/2/3.
  - acceptance: wrapper exits match fake native exits for delegated actions.
  - verify: wrapper test command
  - depends: T4
  - requirements: 3,4

## Wave 3 — Documentation and skill integration
- [ ] T7 — Write wrapper README
  - why: users need install and mapping docs (Req 2)
  - role: builder
  - files: scripts/README.md or docs/slash-workflows.md
  - contract: include source/install, Python usage, native mapping table, safety model, examples.
  - acceptance: every implemented command/action has at least one example.
  - verify: markdown lint/manual check
  - depends: T1,T2
  - requirements: 2

- [ ] T8 — Update AGENTS/user-facing quick reference
  - why: agents need discoverable command use (Req 5)
  - role: builder
  - files: AGENTS.md or embedded AGENTS template if shipping to user repos
  - contract: concise quick reference; state wrappers are UX glue and native specd enforces gates.
  - acceptance: no conflict with existing five non-negotiable rules.
  - verify: make test if embedded assets changed
  - depends: T7
  - requirements: 5

- [ ] T9 — Add or update skill docs if shipped
  - why: action plan includes skill docs for slash workflows (Req 5)
  - role: builder
  - files: internal/core/embed_templates/skills or relevant pack paths, docs
  - contract: progressive disclosure preserved; no requirement to load all skills at once.
  - acceptance: fresh init includes expected docs if templates are embedded.
  - verify: go test ./internal/core ./internal/cmd -run 'Init|Scaffold|Skill'
  - depends: T7
  - requirements: 5

## Wave 4 — CI readiness
- [ ] T10 — Wire wrapper tests into documented local gate
  - why: quality gate must include new command pack (Req 3,6)
  - role: builder
  - files: Makefile, TESTING.md, scripts/README.md as needed
  - contract: add or document `make test` integration or separate wrapper test target.
  - acceptance: local contributor can run one documented command and see wrapper tests.
  - verify: make test
  - depends: T4,T5,T6
  - requirements: 3,6

- [ ] T11 — Run final verification
  - why: implementation must be release-ready (Req 6)
  - role: verifier
  - files: none
  - contract: run `make test`; run `make ci` if Go/core/template changes occurred.
  - acceptance: tests pass; failures trigger backprop/fix before completion.
  - verify: make test && make ci
  - depends: T8,T9,T10
  - requirements: 6
