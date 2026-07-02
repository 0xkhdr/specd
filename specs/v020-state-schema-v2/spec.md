# V1 — State Schema v2 (v0.2.0 Foundation)

## 1. Purpose and requirement coverage

Extend `state.json` with the v0.2.0 blocks (`mode` extended enum, `evals`,
`routing`, `conductor`, `escalation`) so every downstream feature has a durable,
CAS-protected home. Covers plan task **P1.1** (P0). This is the single
foundation spec: every other v0.2.0 spec depends on it.

## 2. Verified current state

- State machinery: `internal/core/state.go` — **SchemaVersion is 5**, not 1 as
  the action plan assumes (deviation DV1 in `progress.md`). Existing silent
  migration pattern on load; writer emits current version.
- CAS/revision semantics + lock discipline already tested
  (`regression-state-atomicity` spec, `make stress`).
- `ExecutionMode` enum in `internal/core/mode.go` (`simple | orchestrated`);
  deterministic advisory in `internal/core/mode_recommend.go`.
- Corruption handling patterns in existing corruption tests.

## 3. Proposed design and end-to-end flow

Bump SchemaVersion **5 → 6**. Add top-level optional blocks, all `omitempty`:

- `mode`: enum gains `conductor` (validation rejects unknown values).
- `evals`: map suite → latest score summary `{suite, score, minScore, pass, seq, time}`.
- `routing`: per-task `{tier, budgetUSD, ruleIndex}` stamps.
- `conductor`: active session descriptor `{sessionID, task, micro, startedAt}`.
- `escalation`: `{task, rule, facts, time}` record (empty when none).

Loader migrates v5 → v6 silently (fields absent → zero values), idempotent;
writer always emits v6. No change to CAS, revision, lock, or exit-code
contract. Round-trip: load v5 fixture → save → valid v6 → reload identical.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** all existing v0.1.x fields byte-compatible; additive JSON only;
  exit codes 0/1/2/3 untouched.
- **New contract:** v6 block shapes above are the API every other v0.2.0 spec
  writes through — changes after this spec ships require their own migration.
- **Dependencies:** none (Wave 1). **Dependents:** V2–V12.

## 5. Invariants, security, errors, observability, compatibility, rollback

- Constitution invariants 3 (stdlib JSON only), 4 (state mutated only via CLI),
  7 (deterministic round-trips), 9 (backward compat) hold.
- Migration idempotent: migrating an already-v6 file is a no-op.
- Corrupt/hostile state: extend existing corruption tests to the new blocks
  (truncated JSON, wrong types, absurd sizes) — fail closed with exit 2 class
  errors, never partial writes.
- **Rollback:** revert schema bump; v6 readers must tolerate absent blocks so a
  downgrade only loses new-block data, never core spec state.

## 6. Acceptance criteria and validation commands

- v5 fixtures load, round-trip to valid v6; all existing state tests pass
  unmodified.
- `go test ./internal/core/... -run 'State|Migrat' -race -count=2` green.
- `make stress` green (CAS untouched).
- Migration idempotency + corruption table tests present.

## 7. Open decisions and deviations

- **DV1:** plan says `internal/spec/state.go` v1→v2. Actual:
  `internal/core/state.go`, v5→v6. All plan path references map to
  `internal/core/`.
- Open: whether `escalation` is a single record or a history list. Decision:
  single active record in state; history lives in ledgers (V7) — state stays
  small, ledgers stay append-only.
