# specd roadmap — progress

Status across all 15 roadmap specs (`specs/*/tasks.md`). One spec done
(**mcp-server**), 14 pending. Legend: ✅ complete · ⬜ not started · 🟡 in progress.

## Waves to work on next

Ordered by report §9 north star (`specs/README.md`). Each spec runs its waves in
dependency order; tasks within a wave may run in parallel.

| Order | Spec | Idea | Status | Waves left | Next task |
|-------|------|------|--------|-----------|-----------|
| 1 | mcp-server | A1 | ✅ done | 0 / 3 | — |
| 2 | semantic-acceptance-gate | B1 | 🟡 | 2 / 4 | W3·T4 GateAcceptance (off/warn/error) |
| 3 | open-spec-format | E2 | 🟡 | 1 / 3 | W3·T4 `specd schema [--version]` |
| 4 | prompt-scaffolding | A2 | 🟡 | 1 / 3 | W3·T4 wire `--from` into `new` |
| 4 | spec-pack-registry | E1 | 🟡 | 1 / 3 | W3·T4 pack resolver (pinned SHA256) |
| 5 | watch-daemon | C1 | 🟡 | 1 / 3 | W3·T4 SSE transport over net/http |
| 5 | github-native-integration | E3 | 🟡 | 1 / 3 | W3·T5 composite Action + PR comment |
| 6 | coverage-diff-scope-evidence | B2 | 🟡 | 1 / 3 | W3·T4 GateScope (warn/error) |
| 6 | verify-sandboxing | B3 | 🟡 | 2 / 4 | W3·T4 bwrapRunner (fail-closed) |
| 7 | verify-revert-on-fail | D1 | 🟡 | 1 / 3 | W3·T5 flag-unset byte-identical test |
| 7 | custom-gate-api | D2 | 🟡 | 1 / 3 | W3·T4 gates.custom pipeline integration |
| 7 | replay-spec-diff | D3 | 🟡 | 1 / 3 | W3·T4 `specd diff --from --to` |
| 8 | ide-dashboard | A3 | 🟡 | 1 / 3 | W3·T4 served-view == static report test |
| 8 | distributed-state-backend | C2 | 🟡 | 1 / 3 | W3·T4 git-native backend |
| 8 | cost-telemetry-ledger | C3 | 🟡 | 1 / 3 | W3·T5 per-wave/per-spec roll-up |

**Totals:** 55 / 103 tasks done (53%). 1 / 15 specs complete · 14 in progress
(all Wave 1 recon + **all Wave 2 builders landed**, full `-race` suite green,
coverage 67.4% / core 63.7% above floors, 16×20 concurrency stress intact).

---

## ✅ mcp-server (A1) — Native MCP Server — COMPLETE

### Wave 1 — Schema & transport foundation
- [x] T1 — Map the command schema source of truth · investigator
- [x] T2 — JSON-RPC 2.0 + MCP envelope structs · builder

### Wave 2 — Tool generation & dispatch
- [x] T3 — Generate `tools/list` from the command schema · builder
- [x] T4 — `tools/call` dispatch into existing handlers · builder
- [x] T5 — `specd mcp` command + registry entry · builder

### Wave 3 — Integration & guardrails
- [x] T6 — End-to-end handshake test over a pipe · verifier
- [x] T7 — Tool count parity + stdlib-only · verifier
- [x] T8 — Review: no new deps, no LLM/network · reviewer

---

## 🟡 semantic-acceptance-gate (B1) — Semantic Acceptance Gate

### Wave 1 — Map the existing scaffolding
- [x] T1 — Inventory the acceptance stubs · investigator

### Wave 2 — Parse criteria & mapping
- [ ] T2 — Number EARS acceptance criteria with stable IDs · builder
- [ ] T3 — Parse `acceptance:` mapping in tasks.md · builder

### Wave 3 — Gate 8 + evidence binding
- [ ] T4 — `GateAcceptance` (off/warn/error) appended to CheckGates · builder
- [ ] T5 — Record CriterionRecords on completion · builder

### Wave 4 — Surface + backward-compat
- [ ] T6 — Show criterion coverage in `check` + `report` · builder
- [ ] T7 — Test: `acceptance: off` byte-identical to today · verifier
- [ ] T8 — Review: no LLM judgment, enforcement-only · reviewer

---

## 🟡 open-spec-format (E2) — Open Spec Format

### Wave 1 — Canonical type recon
- [x] T1 — Inventory the canonical artifact types · investigator

### Wave 2 — Schema + conformance
- [ ] T2 — Author versioned JSON Schema (v1) for all artifacts · builder
- [ ] T3 — Conformance test: schema ↔ Go types (drift fails CI) · verifier

### Wave 3 — Commands + docs
- [ ] T4 — `specd schema [--version]` emits embedded schema · builder
- [ ] T5 — `specd validate --schema` format conformance mode · builder
- [ ] T6 — `docs/spec-format.md` versioned prose standard · builder
- [ ] T7 — Review: schema is single source of truth, v1 non-breaking · reviewer

---

## 🟡 prompt-scaffolding (A2) — One-shot Scaffolding

### Wave 1 — Derive constraints from gates
- [x] T1 — Map gate constraints to a single source · investigator

### Wave 2 — Persist prompt + brief generator
- [ ] T2 — Persist `--from` prompt into the spec · builder
- [ ] T3 — `authoring.go` gate-shaped brief generator · builder

### Wave 3 — Wire & validate
- [ ] T4 — Wire `--from` into `new` to emit the brief · builder
- [ ] T5 — Test: brief stays in sync with real gates · verifier
- [ ] T6 — Test: faithful draft passes `specd check` · verifier
- [ ] T7 — Review: no LLM/network leaked into the binary · reviewer

---

## 🟡 spec-pack-registry (E1) — Spec-pack Registry

### Wave 1 — Init + verify recon
- [x] T1 — Map init scaffolding + SHA256 verify pattern · investigator

### Wave 2 — Pack format + built-ins
- [ ] T2 — `pack.json` manifest format + parser (declarative only) · builder
- [ ] T3 — Embed built-in packs + `--list-packs` · builder

### Wave 3 — Resolve + apply
- [ ] T4 — Pack resolver: embedded + remote (pinned SHA256, fail-closed) · builder
- [ ] T5 — `specd init --pack` transactional apply · builder
- [ ] T6 — Test: fail-closed remote + default regression · verifier
- [ ] T7 — Document the pack manifest contract · builder

---

## 🟡 watch-daemon (C1) — Watch Daemon + Event Stream

### Wave 1 — Change-signal recon
- [x] T1 — Map frontier computation + revision signal · investigator

### Wave 2 — Core loop + events
- [ ] T2 — `FrontierEvent` model + change detector · builder
- [ ] T3 — JSON-lines emitter + `specd watch` command · builder

### Wave 3 — Transports + lifecycle
- [ ] T4 — SSE transport over net/http · builder
- [ ] T5 — Webhook POST with bounded backoff · builder
- [ ] T6 — Signal handling + clean shutdown · builder
- [ ] T7 — Review: read-only, no duplicate events, stdlib-only · reviewer

---

## 🟡 github-native-integration (E3) — GitHub-native Integration

### Wave 1 — Output + exit-code recon
- [x] T1 — Map check exit codes + report/status output · investigator

### Wave 2 — Deterministic PR summary (no network)
- [ ] T2 — `specd report --pr-summary` Markdown + JSON · builder
- [ ] T3 — Commit↔task link map · builder
- [ ] T4 — Test: PR-summary path makes no network call · verifier

### Wave 3 — Action wrapper + docs
- [ ] T5 — Composite Action: check status + upsert PR comment · builder
- [ ] T6 — Workflow snippet + permissions docs · builder
- [ ] T7 — Review: no network in binary, supply-chain pinned · reviewer

---

## 🟡 coverage-diff-scope-evidence (B2) — Coverage & Diff-scope Evidence

### Wave 1 — Record & contract recon
- [x] T1 — Map verify record + files-contract plumbing · investigator

### Wave 2 — Capture evidence
- [ ] T2 — Add `ChangedFiles` + `Coverage` to the record · builder
- [ ] T3 — Capture changed files + optional coverage in `RunVerify` · builder

### Wave 3 — Scope gate + surface
- [ ] T4 — `GateScope` (warn/error, `*`/unset = no-op) · builder
- [ ] T5 — Report shows changed-file count + coverage · builder
- [ ] T6 — Review: coverage is evidence, not a binary floor · reviewer

---

## 🟡 verify-sandboxing (B3) — Verify Sandboxing

### Wave 1 — Runner recon
- [x] T1 — Map the current verify execution path · investigator

### Wave 2 — Runner abstraction
- [ ] T2 — Extract `Runner` interface; default `shRunner` = today · builder
- [ ] T3 — Add `Sandbox` to VerificationRecord (back-compat) · builder

### Wave 3 — Sandbox backends
- [ ] T4 — `bwrapRunner` (fail-closed if absent) · builder
- [ ] T5 — `containerRunner` (docker/podman, fail-closed if absent) · builder
- [ ] T6 — Wire `verify.sandbox` config + `--sandbox` flag · builder

### Wave 4 — Safety + docs
- [ ] T7 — Test: fail-closed on missing isolator; `none` regression · verifier
- [ ] T8 — Update SECURITY.md isolation + fail-closed contract · builder

---

## 🟡 verify-revert-on-fail (D1) — Automatic Rollback on Failed Verify

### Wave 1 — Verify-path recon
- [x] T1 — Map RunVerify exit handling + git usage · investigator

### Wave 2 — Safe revert
- [ ] T2 — Repo-safety pre-check (skip+warn on unsafe state) · builder
- [ ] T3 — `--revert-on-fail` recoverable stash on non-zero exit · builder
- [ ] T4 — Record `Reverted`/`StashRef` in VerificationRecord · builder

### Wave 3 — Regression + review
- [ ] T5 — Test: flag unset byte-identical; pass never touches tree · verifier
- [ ] T6 — Review: no reset --hard, evidence gate intact · reviewer

---

## 🟡 custom-gate-api (D2) — Plugin / Custom-Gate API

### Wave 1 — Gate pipeline recon
- [x] T1 — Map CheckGates pipeline + env-scrub helper · investigator

### Wave 2 — Contract + runner
- [ ] T2 — Define `CustomGateInput`/`Output` JSON contract · builder
- [ ] T3 — `customgate.go` runner (bounded timeout, env-scrubbed) · builder

### Wave 3 — Pipeline integration
- [ ] T4 — `gates.custom` config + pipeline integration (warn/error) · builder
- [ ] T5 — Test: 7 core gates unchanged with/without custom gates · verifier
- [ ] T6 — Document the stdin/env/stdout contract · builder
- [ ] T7 — Review: no Go plugin loading, no network · reviewer

---

## 🟡 replay-spec-diff (D3) — Replay & Spec Diff

### Wave 1 — Audit-source recon
- [x] T1 — Inventory on-disk audit records · investigator

### Wave 2 — Replay
- [ ] T2 — `replay.go` event collector + stable ordering · builder
- [ ] T3 — `specd replay` command (text + JSON) · builder

### Wave 3 — Diff
- [ ] T4 — `specd diff --from --to` over artifact git history · builder
- [ ] T5 — Test: deterministic output, read-only, no panic on gaps · verifier
- [ ] T6 — Review: no LLM, no mutation · reviewer

---

## 🟡 ide-dashboard (A3) — IDE Extension + Live Dashboard

### Wave 1 — Report data reuse
- [x] T1 — Map the report data path · investigator

### Wave 2 — Read-only server
- [ ] T2 — `specd serve` read-only HTTP server · builder
- [ ] T3 — 404 + no-panic on missing spec/root · builder

### Wave 3 — Parity + extension
- [ ] T4 — Test: served view == static report · verifier
- [ ] T5 — VS Code extension webview (separate package) · builder
- [ ] T6 — Review: read-only, no mutating routes · reviewer

---

## 🟡 distributed-state-backend (C2) — Distributed State Backend

### Wave 1 — Contract recon
- [x] T1 — Document the exact current lock + CAS contract · investigator

### Wave 2 — Interface + conformance net
- [ ] T2 — Extract `StateBackend` interface; file backend behind it · builder
- [ ] T3 — Backend-agnostic conformance test suite · verifier

### Wave 3 — git backend + remote adapters
- [ ] T4 — git-native backend (no Go dep) · builder
- [ ] T5 — Redis/Postgres adapters behind build tags · builder
- [ ] T6 — Test: default binary links no DB/redis driver · verifier
- [ ] T7 — Review: integrity spine unweakened · reviewer

---

## 🟡 cost-telemetry-ledger (C3) — Cost & Telemetry Ledger

### Wave 1 — Clock + record recon
- [x] T1 — Map status transitions + clock injection points · investigator

### Wave 2 — Capture
- [ ] T2 — Add `Telemetry` to TaskState (omitempty, back-compat) · builder
- [ ] T3 — Capture duration/retries/verify-duration via injectable clock · builder
- [ ] T4 — `--tokens`/`--cost` annotation flags (stored, not computed) · builder

### Wave 3 — Aggregate + render
- [ ] T5 — Per-wave/per-spec roll-up in report (+ JSON) · builder
- [ ] T6 — Review: no cost computation / pricing API · reviewer
