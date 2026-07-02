# specd Production Regression — Progress

**Overall status:** Wave 1 baseline complete; Wave 2 implementation complete; Wave 3 complete; Wave 4 implementation complete locally.
**Current wave:** Wave 4 complete locally → pending remote PR CI confirmation.
**Wave 1 evidence:** `specs/wave1-baseline.md`.

## Requirement → Spec coverage (R1–R15)

| Req | Description | Spec(s) |
|-----|-------------|---------|
| R1  | Deterministic exit codes | S1, (S15 robustness) |
| R2  | State atomicity (CAS + lock) | S2 |
| R3  | DAG frontier/cycle/wave | S3 |
| R4  | Verify shell/bwrap/container fail-closed | S4, S10 |
| R5  | MCP stdio/HTTP/SSE + auth | S5, S10 |
| R6  | `init` idempotency / packs / host detection | S6 |
| R7  | Report Markdown/HTML/PR summary | S7 |
| R8  | Install checksum fail-closed | S8 |
| R9  | Cross-platform builds | S9 |
| R10 | Path/env/auth/isolation boundaries | S10 |
| R11 | Custom gates scrubbed env + timeout | S4 |
| R12 | Docs match behavior | S1, S14 |
| R13 | Performance baselines ±10% | S11 |
| R14 | Coverage floors maintained | S12 |
| R15 | CI reliable & complete | S13 |

All 15 requirements covered by ≥1 spec. ✅

## Spec status

| Spec | Directory | Deps | Wave | Status | Validation |
|------|-----------|------|------|--------|-----------|
| S1  | `regression-cli-commands` | — | 1 | Authored | `go test ./internal/cmd/... -count=2` |
| S2  | `regression-state-atomicity` | — | 1 | Authored | `make stress` |
| S4  | `regression-verify-sandbox` | — | 1 | Authored | `go test ./internal/runner/...` |
| S8  | `regression-install-integrity` | — | 1 | Authored | `bash scripts/install_test.sh` |
| S15 | `regression-fuzz-parsers` | — | 1 | Authored | `go test ./internal/core/... -run Fuzz` |
| S3  | `regression-dag-scheduling` | S2 | 2 | Authored | `go test ./internal/core/... -run 'DAG\|Frontier'` |
| S5  | `regression-mcp-server` | S1 | 2 | Authored | `go test ./internal/mcp/...` |
| S10 | `regression-security-boundaries` | S2,S4,S5 | 2 | Authored | `golangci-lint run && govulncheck ./...` |
| S6  | `regression-onboarding` | S5 | 3 | Authored | `make perf-gate` |
| S7  | `regression-reporting` | S3 | 3 | Authored | `go test ./internal/core/... -run Report` |
| S9  | `regression-cross-platform` | — | 4 | Authored | `GOOS=windows go build ./...` |
| S11 | `regression-performance-baselines` | S1–S10 | 4 | Authored | `make bench` |
| S12 | `regression-coverage-floors` | S1–S10 | 4 | Authored | `make cover-check` |
| S13 | `regression-ci-pipeline` | S1–S12 | 4 | Authored | `make ci` |
| S14 | `regression-documentation-accuracy` | S1 | 4 | Authored | `make docs-lint` |

## Waves

- **Wave 1 (no deps):** S1, S2, S4, S8, S15
- **Wave 2:** S3 (←S2), S5 (←S1), S10 (←S2,S4,S5)
- **Wave 3:** S6 (←S5), S7 (←S3)
- **Wave 4:** S9, S11, S12, S13, S14

## Baselines / targets

| Metric | Source | Current |
|--------|--------|---------|
| Coverage floors | `scripts/coverage-check.sh` | overall 79, core 80, cmd 71, worker 88, mcp 88, harness 80, spec 99, context 91, runner 92, pack 86, schema 83 |
| Coverage targets | `TESTING.md` | 85 overall; core 90→95 |
| Perf baselines | `docs/agent-harness-baselines.md` | read locally; refresh via `make bench` |
| Schema version | `internal/core/state.go:16` | 5 |
| Go | `go.mod` | 1.22 (toolchain go1.22.0), stdlib-only |

## Decisions & deviations (from the analysis plan)

- **D1 — Command set corrected.** Plan lists 13 commands incl. `migrate` +
  `doctor`. Verified `registry.go` has **17** dispatchable commands (`init`,
  `handshake`, `new`, `approve`, `decision`, `midreq`, `memory`, `brain`,
  `pinky`, `next`, `verify`, `task`, `status`, `context`, `check`, `report`,
  `waves`) + `mcp`/`help`/`version` in `main.go`. `migrate` and `doctor` do not
  exist. Affects S1, S14.
- **D2 — No prior specs.** Plan claims `specs/` holds 7 optimization specs.
  `specs/` was empty; these 15 regression specs are the first. No prior-spec
  regression check needed (U1 resolved).
- **D3 — Coverage floors corrected.** worker floor is **88** (not 50); floors
  exist for mcp/harness/spec/context/runner/pack/schema. Plan's F6 ("raise
  worker to 70") already exceeded. Affects S12.
- **D4 — Larger subsystem.** Codebase includes ACP (`acp*.go`), orchestration
  engine (`orchestration*.go`), program lifecycle (`program*.go`), and
  brain/pinky workers (`brain*.go`, `pinky*.go`) not in the plan's architecture.
  Their state paths fold into S2/S3; stress via `stress-acp/orchestration/
  program/brain-recovery/checkpoint-fault.sh`.
- **D5 — More CI/stress jobs.** CI has `stress`, `stress-brain-recovery`,
  `stress-checkpoint-fault`; `make ci` additionally runs `stress-acp`,
  `stress-orchestration`, `stress-program` (not yet dedicated CI jobs). Parity
  gap flagged in S13.
- **D6 — Fuzz already partial.** `internal/mcp/host_caps_fuzz_test.go` exists;
  S15 adds fuzz for the *core parsers* that lack it. Affects S5, S15.
- **D7 — `mcp` routing.** `mcp` is handled in `main.go` (pre-dispatch), not the
  registry. Affects S1.

## Remaining work

- [x] Execute Wave 1 baseline tasks (`specs/wave1-baseline.md`).
- [x] Execute Wave 2 implementation tasks.
- [x] Execute Wave 3 implementation tasks.
- [x] Execute Wave 4 implementation tasks.
- [x] Final local gate: `make ci` green, floors held/raised, stress green.
- [ ] Remote PR CI green confirmation.
