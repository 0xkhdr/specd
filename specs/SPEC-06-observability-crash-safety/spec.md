# SPEC-06: Observability & Crash-Safety

## Overview
- **Domain:** Observability & Debugging (Analysis Plan Domain 8)
- **Risk Level:** Low (deterministic reporting is a design strength)
- **Priority:** P2
- **Dependencies:** SPEC-01 (the ACP/checkpoint crash-safety stress jobs overlap SPEC-01's script
  reconciliation).

## Current State

- **Deterministic reporting:** `specd report` (`--pr|--metrics|--json|--history|--format
  prometheus`), `specd status --program`, `handshake` digests, and `context --hud` operator HUD are
  all generated from `state.json` + task artifacts with no LLM in the path.
- **Crash-safety now proven in CI (updated Wave 2):** the ACP ledger
  (`internal/orchestration/acp.go`) append/replay is crash-safe, proven by the `stress-acp.sh` /
  `stress-checkpoint-fault.sh` jobs — SPEC-01 restored and wired both (commit `a5e3935`; the
  earlier "both missing — B2" note is stale). T-06-04 fixed the double-dispatch race those jobs
  exposed; they now pass 30/30.
- **Worker metrics stored verbatim:** worker-reported `--tokens` / `--cost` / `--duration-ms` are
  stored as-is, but where they surface in reports is undocumented.
- **Doc gaps:** no documented logging levels / telemetry strategy for the CLI itself; CAS/lock
  error actionability is documented in `troubleshooting.md` (confirm it stays accurate).

## Target State

Every reporting output is regression-tested for validity and ordering; worker-metric surfacing and
CLI logging strategy are documented; ACP/checkpoint crash-safety is proven by the restored stress
jobs.

## Scope Boundaries

- **In Scope:** validation of `report --format prometheus` / `--history` / `--json` / `--metrics`
  and `context --hud` outputs; documenting logging levels + worker-metric surfacing; the *content*
  of `stress-acp.sh` / `stress-checkpoint-fault.sh` crash-safety assertions (SPEC-01 owns whether
  the scripts exist; SPEC-06 owns what they prove); CAS/lock error actionability.
- **Out of Scope:** the CI script reconciliation mechanism (SPEC-01); performance benchmarks
  (SPEC-03); adding telemetry that phones home; anything under `reference/`.

## Technical Requirements

1. **Prometheus validity:** `report --format prometheus` emits valid textfile-collector metrics
   (well-formed metric names/labels/values). Assert with a parser/format check.
2. **History ordering:** `report --history` replays the audit trail in timestamp order. Assert
   ordering deterministically.
3. **JSON/metrics shape:** `--json` and `--metrics` outputs have a stable, documented schema.
4. **HUD:** `context --hud` renders the operator HUD from state without error.
5. **Error discipline:** consistent exit 1 vs 2; no silent failures; CAS/lock errors are
   actionable and match `troubleshooting.md`.
6. **Crash-safety assertions:** define what `stress-acp.sh` and `stress-checkpoint-fault.sh` must
   prove — ACP ledger append/replay survives an interrupted write (fault-injected) and replays to a
   consistent state. Provide these assertions to SPEC-01 for wiring (or wire them if SPEC-01
   deletes those jobs and SPEC-06 re-adds them as targeted tests).
7. **Docs:** document CLI logging levels/telemetry strategy and where worker `--tokens`/`--cost`/
   `--duration-ms` surface in reports.

## Verification Strategy

- A test parses `report --format prometheus` output and asserts validity.
- A test asserts `--history` timestamp ordering and `--json`/`--metrics` schema stability.
- A fault-injection stress run proves ACP/checkpoint replay reaches a consistent state after an
  interrupted write (green in CI once wired).
- Docs describe logging levels and worker-metric surfacing; `troubleshooting.md` matches actual
  CAS/lock error messages.
- No LLM in report/gate/DAG paths (reporting stays deterministic); no bypass flag; `reference/`
  untouched.

## References
- Analysis Plan: Domain 8; Blocker B2 (missing stress scripts); Recommended Spec Breakdown row
  SPEC-06.
- Related Specs: SPEC-01 (stress-script existence/wiring), SPEC-03 (perf, release), SPEC-04
  (evidence integrity).
- Source Files: `internal/cmd/` report/status/handshake/context handlers,
  `internal/orchestration/acp.go`, `state.json` schema, `docs/troubleshooting.md`,
  `.github/workflows/ci.yml` (stress-acp / stress-checkpoint-fault).
