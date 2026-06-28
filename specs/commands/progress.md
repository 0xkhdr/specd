# Commands Program — Progress & Wave Plan

This program adds user-facing slash/workflow wrappers around native `specd` commands while preserving native gates as the source of truth. Each child spec owns detailed requirements in `spec.md`, task DAGs in `tasks.md`, and local wave status in its own `progress.md`.

## Spec map

| Focus | Title | Spec | Priority | Current status |
|---|---|---|---|---|
| Init UX | Interactive `/init` Command Wrapper | [interactive-init](interactive-init/spec.md) | P0 | pending |
| Steering UX | `/steer` Steering Console | [steering-console](steering-console/spec.md) | P0 | pending |
| Spec lifecycle UX | `/spec` Workflow Dashboard | [spec-dashboard](spec-dashboard/spec.md) | P0 | pending |
| Orchestration UX | `/pinky-brain` Orchestration Console | [pinky-brain-console](pinky-brain-console/spec.md) | P1 | pending |
| Shared packaging | Slash Workflow Packaging, Tests, and Documentation | [workflow-packaging-testing](workflow-packaging-testing/spec.md) | P0 | pending |

## Program waves

Run waves in order. Intra-spec task ordering remains defined by each `tasks.md`; this file coordinates cross-spec sequencing and status.

### Wave 1 — Shared foundations and safe read paths — **status: complete**

| Spec | Tasks | Status | Depends on | Exit criteria |
|---|---:|---|---|---|
| interactive-init | T1-T2 | complete | — | `/init` option model exists; host probe handles JSON/fallback. |
| steering-console | T1-T3 | complete | — | Root discovery, canonical `show`, and stub `status` are safe/read-only. |
| spec-dashboard | T1-T2 | complete | — | Spec listing and slug selection are deterministic and safe. |
| pinky-brain-console | T1-T3 | complete | — | Capability/config/session status render read-only and tolerate native failures. |
| workflow-packaging-testing | T1-T3 | complete | child T1 foundations | Shared shell/Python entrypoints and helpers exist with parity targets. |

### Wave 2 — User actions and native delegation — **status: complete**

| Spec | Tasks | Status | Depends on | Exit criteria |
|---|---:|---|---|---|
| interactive-init | T3-T4 | complete | Wave 1 interactive-init | Interactive/non-interactive flow builds safe `specd init` argv and propagates native exits. |
| steering-console | T4-T5 | complete | Wave 1 steering-console | Edit/bootstrap only touch canonical steering files; dry-run writes nothing. |
| spec-dashboard | T3-T5 | complete | Wave 1 spec-dashboard | `new`, `continue`, and direct lifecycle actions delegate to native `specd`. |
| pinky-brain-console | T4-T8 | complete | Wave 1 pinky-brain-console | Enable/disable and session ops delegate safely; worker view remains read-only. |

### Wave 3 — Safety regression net — **status: pending**

| Spec | Tasks | Status | Depends on | Exit criteria |
|---|---:|---|---|---|
| workflow-packaging-testing | T4-T6 | pending | Wave 1 packaging; Wave 2 action surfaces | Fake `specd` harness, safety invariant tests, parity tests, and exit propagation tests pass. |
| spec-dashboard | T7 | pending | spec-dashboard/T4-T5; fake harness | Tests prove wrappers never auto-run `specd task --status complete`. |
| pinky-brain-console | T9-T10 tests subset | pending | pinky-brain-console/T5,T8 | Platform guard and proof-boundary tests pass; no forged Pinky reports. |

### Wave 4 — Documentation, skills, and release gate — **status: pending**

| Spec | Tasks | Status | Depends on | Exit criteria |
|---|---:|---|---|---|
| interactive-init | T5-T6 | pending | interactive-init/T4; Wave 3 tests | `/init` tests/docs cover dry-run, orchestration, failures, and examples. |
| steering-console | T6-T7 | pending | steering-console/T5 | Memory action documented; usage explains canonical file filtering. |
| spec-dashboard | T6,T8 | pending | spec-dashboard/T5,T7 | Mode fallback and lifecycle docs complete with evidence warning. |
| pinky-brain-console | T9-T10 docs subset | pending | pinky-brain-console/T6-T8 | WSL/POSIX guard and orchestration safety docs complete. |
| workflow-packaging-testing | T7-T11 | pending | Wave 3 | README/AGENTS/skills updated as needed; local test gate documented; final verification run. |

## Status legend

`pending` → `in-progress` → `verifying` → `complete` / `blocked`

## Management rules

- Child `tasks.md` files own task checkboxes; update them through the project workflow, not by editing machine state.
- Update this file when a child wave starts, blocks, verifies, or completes.
- Keep wrappers as UX glue only. Native `specd` commands enforce state transitions, evidence gates, and orchestration proof boundaries.
- Treat fake `specd` wrapper tests as required before any docs or release-gate wave is marked complete.

## Program completion checklist

- [ ] Shell command pack exposes `/init`, `/steer`, `/spec`, `/pinky-brain`, and `specd_workflow`.
- [ ] Python CLI mirrors shell behavior with stdlib-only implementation.
- [ ] Wrapper tests cover native JSON/text fallback, non-TTY behavior, platform guards, and exit propagation.
- [ ] Safety tests prove wrappers do not mutate `state.json`, flip `tasks.md` checkboxes, auto-complete tasks, or forge Pinky reports.
- [ ] Docs include install/source instructions, Python usage, native mapping table, examples, and safety model.
- [ ] `make test` passes; `make ci` passes if Go/core/templates change.
