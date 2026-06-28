# Tasks — `/steer` Steering Console

## Wave 1 — Root discovery and read views
- [x] T1 — Implement steering root discovery
  - why: command must work from subdirectories (Req 1)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: walk upward to `.specd/steering`; return 3 if absent; no arbitrary path roots.
  - acceptance: tests pass from repo root and nested dir; missing root returns 3.
  - verify: wrapper tests
  - depends: none
  - requirements: 1

- [x] T2 — Implement `show` and canonical file filtering
  - why: users need all steering files visible safely (Req 2,5)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: list six canonical files; `show <file>` accepts basename only from canonical set.
  - acceptance: `show ../config.json` returns 2; missing files reported clearly.
  - verify: wrapper tests
  - depends: T1
  - requirements: 2,5

- [x] T3 — Implement `status` stub detection
  - why: bootstrap must reveal unauthored steering (Req 2)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: classify missing/stub/placeholders/authored using deterministic thresholds and marker regex.
  - acceptance: fixture files classify deterministically across shell/Python.
  - verify: wrapper tests
  - depends: T2
  - requirements: 2

## Wave 2 — Editing and bootstrap
- [x] T4 — Implement safe edit flow
  - why: users need convenient authorship without unsafe writes (Req 3)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: use `$EDITOR` when set; no editor + no explicit stdin mode returns guidance.
  - acceptance: fake editor receives all canonical files; no editor in non-TTY exits 2.
  - verify: wrapper tests with fake editor
  - depends: T2
  - requirements: 3,5

- [x] T5 — Implement `bootstrap` guidance
  - why: action plan requires guided product/tech/structure authorship (Req 3)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py, scripts/README.md
  - contract: print inspection checklist; edit or stdin-write only `product.md`, `tech.md`, `structure.md`; support dry-run.
  - acceptance: dry-run writes nothing; stdin mode writes only selected canonical file(s).
  - verify: wrapper tests
  - depends: T4
  - requirements: 3

## Wave 3 — Memory and docs
- [x] T6 — Implement `memory` action
  - why: users requested all steering access; memory has special phase semantics (Req 4)
  - role: builder
  - files: scripts/specd-workflow.sh, scripts/specd-workflow.py
  - contract: print memory header/content; missing memory is warning only.
  - acceptance: existing memory printed exactly; missing memory does not create file.
  - verify: wrapper tests
  - depends: T2
  - requirements: 4

- [x] T7 — Document `/steer`
  - why: command needs safe usage patterns (Req 5)
  - role: builder
  - files: scripts/README.md, AGENTS.md or skill docs if shipped
  - contract: docs explain `show`, `status`, `edit`, `bootstrap`, `memory`, canonical file list.
  - acceptance: examples match CLI behavior.
  - verify: markdown lint/manual check
  - depends: T6
  - requirements: 5
