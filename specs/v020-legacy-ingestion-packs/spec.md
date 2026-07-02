# V10 — Legacy Ingestion & Migration Packs

## 1. Purpose and requirement coverage

Bring existing codebases under the harness without repeating the boot/enrich
mistake: the binary **inventories** (countable), the agent **understands**
(semantic, via skill), the gate **enforces coverage** (countable). Plus
shipped migration spec packs runnable on V7 schedules. Covers plan tasks
**P5.3** (P0) and **P5.4** (P1).

## 2. Verified current state

- CHANGELOG records `boot`/`enrich` **removed** for violating the Foundational
  Split — the design constraint this spec exists to respect.
- Pack machinery: `internal/pack/` with `embed_packs/`; `specd init --pack`.
- Spec scaffolding: `internal/cmd/new.go`, `internal/core/scaffold.go`.
- Approve ratchet + gates apply unchanged to ingestion-flavored specs.
- V7 `program schedule/tick` is the execution vehicle for migration packs.

## 3. Proposed design and end-to-end flow

- **Ingestion (P5.3):** `specd ingest new <slug> --path <dir>` — validates the
  path, creates an ingestion-flavored spec scaffold with a deterministic
  **inventory** written to `inventory.json`: file list, sizes, package/module
  names parsed from manifests with stdlib. Countable facts only; the binary
  never reads legacy semantics. The `specd-ingest` skill (same PR) teaches the
  agent to reverse-engineer requirements/design/tasks from the code into the
  scaffold. New `ingest` gate: every inventory file referenced by ≥1
  requirement **or** explicitly waived with a reason — coverage as a countable
  fact. Normal approve ratchet applies end-to-end.
- **Migration packs (P5.4):** `internal/pack/embed_packs/` gains
  `migrate-deps`, `modernize-tests`, `upgrade-go` — each a spec template +
  skill with pre-filled task DAG shape and eval rubrics (V5). Runnable via V7
  schedules for the SDLC "auto-refactor" concept.

## 4. Interfaces, contracts, data, configuration, dependencies

- **New artifact:** `.specd/specs/<slug>/inventory.json` (CLI-owned facts).
- **New commands:** `ingest new`; new gate `ingest` (registry discipline).
- **Stable:** ingestion specs are normal specs — every existing gate applies.
- **Dependencies:** V1; V5 (pack rubrics), V7 (schedules) for P5.4.
  **Dependents:** V12.

## 5. Invariants, security, errors, observability, compatibility, rollback

- Foundational Split (invariant 1): inventory = countable facts; zero
  perception in the binary. This is the boot/enrich lesson codified.
- Inventory deterministic: same tree → byte-identical `inventory.json`
  (sorted entries, no timestamps beyond FakeClock-able fields).
- Path validation on `--path` (no traversal outside repo; symlink policy
  explicit); manifest parsers are hostile-input parsers → fuzz.
- Waivers require reason strings (same discipline as security allowlist).
- **Rollback:** ingestion spec is a normal spec — delete it.

## 6. Acceptance criteria and validation commands

- Inventory determinism test (`-count=2`, two runs byte-identical).
- Coverage gate math table-tested (mapped, waived, unmapped → fail).
- e2e: ingest a fixture module → agent-authored spec (fixture-simulated)
  passes all gates → 100% inventory mapped/waived (success metric).
- Pack scaffolds: `specd init --pack <name>` produces gate-passing scaffolds
  for all three packs.
- Manifest-parser fuzz tests.
- `go test ./internal/core/... ./internal/pack/... ./internal/cmd/... -run 'Ingest|Pack' -race -count=2`

## 7. Open decisions and deviations

- Path deviation DV1.
- Open: inventory scope filters (vendor/, node_modules/, build artifacts).
  Decision: respect `.gitignore` via `git ls-files` when in a git repo;
  explicit `--include-ignored` override; non-git → full walk with documented
  default excludes.
