# Tasks — `/pinky-brain` Orchestration Console

## Wave 1 — Capability and config status
- [x] T1 — Implement orchestration capability probe
  - why: console must know whether Brain/Pinky commands exist (Req 1)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: inspect `specd help --json`/help text for brain/pinky; handle unavailable native commands.
  - acceptance: fake specd without brain returns unsupported guidance, no crash.
  - verify: wrapper tests
  - depends: none
  - requirements: 1

- [x] T2 — Implement config discovery and read-only status
  - why: users need enabled/disabled visibility (Req 1)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: locate `.specd/config.yml` or `.specd/config.json`; parse orchestration keys or report unknown if unparseable.
  - acceptance: YAML, JSON, missing, invalid config fixtures produce deterministic output.
  - verify: wrapper tests
  - depends: T1
  - requirements: 1

- [x] T3 — Implement session list view
  - why: status should show resumable sessions (Req 1)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: call `specd brain resume --list --json`; tolerate failures; print session id/spec/status/updated when available.
  - acceptance: JSON fixture lists sessions; failure prints warning not false certainty.
  - verify: wrapper tests
  - depends: T1
  - requirements: 1

## Wave 2 — Enable/disable
- [x] T4 — Implement explicit enable flow
  - why: users need policy selection and safe config (Req 2)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: prompt/flag policy, workers, retries, timeout, cost; prefer native init/repair orchestration command.
  - acceptance: generated native argv matches selections; non-interactive requires all needed values/defaults.
  - verify: wrapper tests
  - depends: T2
  - requirements: 2

- [x] T5 — Implement disable flow safely
  - why: users need opt-out without corrupting sessions (Req 2)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: disable future orchestration by native command or atomic config update; warn active sessions unaffected.
  - acceptance: unrelated config keys preserved; existing session files untouched in fixture.
  - verify: wrapper tests
  - depends: T4
  - requirements: 2

## Wave 3 — Session operations and worker view
- [x] T6 — Implement start/run/step actions
  - why: console manages primary orchestration loop (Req 3)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: delegate to native brain commands with policy/workers/retries/timeout from config/defaults.
  - acceptance: argv tests cover start, run with worker-cmd, step with session.
  - verify: wrapper tests
  - depends: T2,T3
  - requirements: 3

- [x] T7 — Implement pause/resume/cancel/compact actions
  - why: users need lifecycle controls (Req 3)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: delegate to native commands; resume without session may select most recent resumable only when unambiguous or documented.
  - acceptance: exit codes propagate; missing session returns usage 2.
  - verify: wrapper tests
  - depends: T3
  - requirements: 3

- [x] T8 — Implement workers read-only view
  - why: users need Pinky visibility without forging reports (Req 4)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: show worker/session data from native output or read-only session files; never write claim/report.
  - acceptance: tests assert no `specd pinky report` invocation.
  - verify: wrapper tests
  - depends: T3
  - requirements: 4

## Wave 4 — Platform guards, tests, docs
- [x] T9 — Add POSIX/Windows guard
  - why: orchestration is POSIX-only (Req 1,5)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: read-only status allowed; mutating/session actions fail fast on native Windows without WSL.
  - acceptance: platform fixture returns clear WSL message.
  - verify: wrapper tests
  - depends: T6,T7
  - requirements: 1,5

- [x] T10 — Add orchestration safety tests and docs
  - why: avoid config corruption and fake evidence (Req 5)
  - role: reviewer
  - files: wrapper tests, scripts/README.md, AGENTS.md or skill docs if shipped
  - contract: tests cover config formats, direct write atomicity if used, unsupported commands, docs explain proof boundary.
  - acceptance: docs and tests complete; no unreviewed direct writes.
  - verify: wrapper test suite
  - depends: T5,T8,T9
  - requirements: 5
