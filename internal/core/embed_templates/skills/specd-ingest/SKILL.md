---
name: specd-ingest
description: Bring an existing (legacy) codebase under the harness — read the deterministic inventory, reverse-engineer requirements/design/tasks, and drive it to 100% coverage through the ingest gate.
---

# specd-ingest — legacy ingestion workflow

Use this skill after `specd ingest new <slug> --path <dir>`. The binary has
already written a deterministic `inventory.json` (countable facts only: file
list, sizes, manifest-derived module names). **The binary never reads legacy
semantics** — that is your job. This is the boot/enrich lesson codified: the
binary inventories, you understand, the gate enforces coverage.

## The division of labor

| Actor | Responsibility | Artifact |
|-------|----------------|----------|
| Binary | Inventory (countable) | `inventory.json` |
| You (agent) | Understand (semantic) | `requirements.md`, `design.md`, `tasks.md` |
| Gate | Enforce coverage (countable) | `ingest` gate |

## Workflow

1. **Read the inventory.** Open `.specd/specs/<slug>/inventory.json`. It lists
   every file the harness will hold you accountable for and the modules it
   detected. This is the source of truth for scope.

2. **Reverse-engineer requirements.** Read the actual code. For each coherent
   capability you find, write a requirement in `requirements.md` (EARS form) and
   **name the concrete files that implement it** — the ingest gate maps a file
   as covered when its path appears verbatim in `requirements.md`. Group files
   by behavior, not by directory.

3. **Waive what is genuinely out of scope.** Generated code, vendored
   dependencies, fixtures, or dead files that no requirement should own go in
   `inventory.json` under `waivers` as `"<path>": "<reason>"`. **A waiver with
   an empty reason does not count** — reasons are mandatory (same discipline as
   the security allowlist). Waive deliberately, not to silence the gate.

4. **Author design + tasks** as for any spec (`specd-design`, `specd-tasks`),
   then run the normal approve ratchet. An ingestion spec is a normal spec —
   every existing gate applies unchanged.

5. **Close coverage.** Enable the gate (`gates.ingest: error`) and run
   `specd check <slug>`. It fails while any inventoried file is neither
   referenced by a requirement nor waived, naming the offenders. Iterate until
   coverage is 100% mapped-or-waived — the success metric.

## Invariants

- Do not edit `inventory.json`'s `files` list — it is CLI-owned. You may only
  add `waivers` entries.
- Every claim about the legacy code must cite `file:line`; do not hallucinate
  behavior the code does not exhibit.
- Coverage is a countable fact, not a judgment: map or waive, nothing in
  between.
