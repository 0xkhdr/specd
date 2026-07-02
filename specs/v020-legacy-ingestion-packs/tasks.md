# V10 Tasks — Legacy Ingestion & Migration Packs

Plan coverage: P5.3, P5.4. Dependencies: V1, V5, V7. Dependents: V12.

## Wave 1 — Inventory + scaffold (P5.3)

- [ ] `internal/core/ingest.go`: deterministic inventory (sorted file list,
  sizes, manifest-derived module names via stdlib); `git ls-files` scoping
  with non-git fallback.
- [ ] `specd ingest new <slug> --path <dir>`: path validation (traversal,
  symlink policy), ingestion-flavored scaffold + `inventory.json`.
- [ ] Manifest-parser fuzz tests; inventory determinism test (two runs
  byte-identical).
- **Validation:** `go test ./internal/core/... ./internal/cmd/... -run Ingest -race -count=2`

## Wave 2 — Coverage gate + skill (depends on Wave 1)

- [ ] `ingest` gate: every inventory file → ≥1 requirement reference or
  reasoned waiver; coverage math table tests.
- [ ] `specd-ingest` skill (same PR): reverse-engineering workflow into the
  scaffold; normal approve ratchet documented.
- [ ] e2e: fixture module → simulated agent-authored spec → all gates green,
  100% mapped/waived.
- **Validation:** `go test ./internal/core/... -run 'IngestGate|Coverage' -race`

## Wave 3 — Migration packs (P5.4, depends on Wave 2)

- [ ] `embed_packs/`: `migrate-deps`, `modernize-tests`, `upgrade-go` — spec
  template + skill + task DAG shape + V5 rubric each.
- [ ] `specd init --pack <name>` produces gate-passing scaffolds (test per
  pack); runnable via V7 `program schedule/tick` (integration test).
- **Validation:** `go test ./internal/pack/... -race -count=2`

## Rollout & cleanup

- [ ] Docs: command-reference (ingest new), validation-gates (ingest gate),
  user-guide ingestion walkthrough, CHANGELOG; parity green.
- **Rollback:** delete ingestion spec; packs are opt-in scaffolds.
- **Completion evidence:** `make ci` green; determinism + coverage-math tests
  committed; boot/enrich lesson referenced in docs (why the binary only
  inventories).
