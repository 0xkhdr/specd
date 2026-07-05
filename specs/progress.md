# Progress — FINDINGS.md remediation program

Source analysis: `FINDINGS.md` (2026-07-05). Each spec below is independently
shippable. Waves map 1:1 to the FINDINGS roadmap tiers; a wave may start only
when every spec in the prior wave is `done` (or explicitly descoped with a
recorded decision). Within a wave, specs are independent and may run
concurrently.

Status legend: `pending` → `in-progress` → `done`; `descoped` requires a
decision record (see `00-hygiene`).

## Wave 0 — Hygiene (FINDINGS Tier 0: no design work, fix now)

| spec | scope (FINDINGS refs) | status | notes |
|---|---|---|---|
| [00-hygiene](00-hygiene/spec.md) | CLAUDE.md/reality drift, missing lint scripts + CHEATSHEET, skip-decision records, config key validation (C.1, C.2, C.7, D.1, D.4) | pending | |
| [01-version-release](01-version-release/spec.md) | `specd version` verb, ldflags injection, goreleaser pipeline (C.3, B.13, D.2) | pending | |
| [02-state-schema-version](02-state-schema-version/spec.md) | `schemaVersion` in state.json, forward-migration hook, `check --schema` (C.5, B.14, B.23, D.3) | pending | |

## Wave 1 — Enforcement completeness (FINDINGS Tier 1: core-promise gaps)

| spec | scope (FINDINGS refs) | status | notes |
|---|---|---|---|
| [03-command-metadata](03-command-metadata/spec.md) | Rich command metadata: exit codes, flag enums, phase/mode compatibility, fail-closed dispatch, `help --json`, MCP consumption (B.2, B.3, C.8, D.5) | done | dispatch choke-point gating live; suite green |
| [04-criterion-evidence](04-criterion-evidence/spec.md) | Per-acceptance-criterion verify records, `verify --criterion` (B.1, D.6) | done | criteria.jsonl store, coverage in status/report, opt-in completion gate; suite green |
| [05-security-suite](05-security-suite/spec.md) | Real security gate: entropy secrets + reasoned allowlist, injection heuristics, slopsquat, per-scanner severity (B.4, C.4, D.7) | pending | |
| [06-escalation-ratchet](06-escalation-ratchet/spec.md) | N failed verifies ⇒ block task until human `--override --reason` (B.6, D.8) | pending | |
| [07-brain-safety](07-brain-safety/spec.md) | Brain `cancel`, crash-safe `resume`, per-step checkpoint (B.19, C.6, D.9) | pending | |

## Wave 2 — Lifecycle completion & operability (FINDINGS Tier 2)

| spec | scope (FINDINGS refs) | status | notes |
|---|---|---|---|
| [08-submit](08-submit/spec.md) | Terminal `submit` verb: all-gates-green check + operator-configured command via sandboxed exec (B.7, D.10) | pending | |
| [09-review-gate](09-review-gate/spec.md) | Scaffolded `review_report.md` + opt-in completion block (B.5, D.11) | pending | |
| [10-telemetry](10-telemetry/spec.md) | Cost/duration annotations on task + ACP records, surfaced in `report --metrics` (B.15, B.20, D.12) | pending | |
| [11-integration-polish](11-integration-polish/spec.md) | `mcp --config <host>`, `init --repair/--refresh/--dry-run`, handshake config digests (B.21, B.22, B.26, D.13) | pending | |
| [12-program-links](12-program-links/spec.md) | Cross-spec `link/unlink` + program frontier view (B.18, D.14) | pending | |
| [13-report-history](13-report-history/spec.md) | `report --history` audit replay + Prometheus textfile format (B.16, D.15) | pending | |

## Wave 3 — Demand-gated (FINDINGS Tier 3: decision records only, no implementation)

No specs. Wave 0's `00-hygiene` records the skip/defer decision for each:
eval/prototype (B.10), deploy/observe (B.8, B.9), packs (B.24), harness
sharing (B.25), ingest (B.11), conductor (B.12), dashboard (B.17),
`triage` verb (C.2). Revisit only on demonstrated demand; a revisit means a
new spec here, not an edit to a decision record.

## Explicit non-goals (FINDINGS "What NOT to bring back")

- `dashboard`, `report --serve` SSE — UI surface, zero enforcement value.
- `conductor` — duplicated by interactive agent hosts.
- `status --program schedule/tick` — host cron + `check` covers it.
- v1 mega-`init` shape (27 KB) — port capabilities, never the shape.
- Any repo perception inside the binary (`boot`/`enrich` class) —
  Foundational Split violation per v1's own changelog.
