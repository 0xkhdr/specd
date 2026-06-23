# Remaining Tasks — Close the Level-Up Production Gate

> Continuation of `tasks.md`. T1, T2, T4, T5 are **done**. This file captures
> the unfinished work: T3 (brain ≥80%), T6 (85/90 coverage stretch), and the
> T7 wave flips that depend on them.
>
> Invariants (unchanged): stdlib-only runtime, no floor ever lowered, every task
> leaves `go build ./...`, `go test ./...`, `golangci-lint run ./...`, and
> `make ci` green.
>
> Status legend: `⬜ not started` · `🟡 in progress` · `✅ done`

---

## Snapshot (2026-06-23)

| Metric | Now | Floor | Target | Gap |
|---|---|---|---|---|
| overall | 77.1% | 77 | ≥85 | +~850 stmts |
| internal/core | 79.8% | 79 | ≥90 | +~580 stmts |
| internal/cmd | 69.2% | 69 | — | — |
| `cmd/brain.go` (file) | 74.6% | — | ≥80 | — |
| brain driver (aggregate) | ≈78.5% | — | ≥80 | +~6 stmts |
| internal/worker | 90.8% | 90 | ≥90 | ✅ |
| internal/mcp | 88.9% | 88 | — | — |
| internal/testharness | 80.0% | 80 | — | — |

Done already: lint exit 0, `make ci` exit 0, recovery tests green `-race -count=2`,
floors ratcheted up, `progress.md`/`tasks.md` flipped honestly.

---

## R1 — Close `cmd/brain.go` ≥80% (finishes T3 / W1.7)

**Why:** aggregate brain driver code is ≈78.5%, ~6 covered statements short of
the literal 80% bar. The single biggest hole is `brainRunBootstrap` (33%): its
spec-creation success branch (`--bootstrap` → `RunNew` → `continue`) is never hit.

**Root cause:** `brain run <slug>` errors `spec '<slug>' not found` (ExitGate)
**before** preflight bootstrap fires — the spec is loaded up front, so the
`item.Kind == "spec" && --bootstrap` branch in `brainRunBootstrap` is dead via
the CLI today.

**Do (pick one):**
1. **Flow fix (preferred):** in `brainRun`/`brainRunSession`, run
   `OrchestrationPreflight` + `brainRunBootstrap` **before** the spec-load that
   errors not-found, so `--bootstrap` can scaffold a missing spec and the drive
   continues. Then add `TestBrainRunBootstrapCreatesSpec` (inited workspace,
   missing spec, `--bootstrap --title` → `core.SpecExists` true).
2. **Direct-unit fallback:** export `brainRunBootstrap` to `cmd_test` via
   `export_test.go` and drive it directly with a synthetic
   `[]core.PreflightItem{{Kind:"spec",...}}` and `--bootstrap` args under a
   harness root (harness `os.Chdir`'s to `h.Root`, so `RunNew` resolves it).
3. Also nudge `brain.go`'s lower funcs (the 50–55% ones) with one or two
   assertions if option 1/2 alone doesn't clear 80%.

**Done when:** aggregate brain driver coverage ≥80%; `brainRunBootstrap` ≥80%;
W1.7 + W1 exit gate flip ✅ in `specs/progress.md`.

---

## R2 — Drive `internal/core` 79.8% → ≥90% (the bulk of T6)

**Why:** A6 requires core ≥90. ~580 uncovered statements remain, concentrated in
the program-orchestration runtime error/edge branches. Cheap pure/0%-func
coverage is already exhausted — what's left needs fixture-driven failure
injection.

**Target files (largest uncovered blocks first):**
- `program_lease.go` — `ReleaseProgramChildLease` (0%), `ensureProgramChildSession`
  (44%), `saveLease`/lease-validation error paths.
- `program_session.go` — `ResumeProgramOrchestration` (0%), status-transition guards.
- `program_step.go` — paused/cancelling/failed branches, mark-status error paths.
- `orchestration_engine.go` — `ActiveOrchestrationSessionForSpec` (0%),
  `IsOrchestrationSessionNotFound` is done; pause/cancel/complete transitions.
- `acp_*.go` — remaining validation-failure branches (`validateACPPayload`,
  `validateACPLease`, `decodeACPStrict`, `parseACPEventFilename`,
  `writeImmutablePrivate`/`atomicWritePrivate` error paths).

**Do:**
1. Build a reusable program-orchestration fixture helper (multi-child program,
   real `.specd` runtime dir) in `helpers_test.go` or a new `*_test.go`.
2. Add lifecycle tests: start → child dispatch → lease claim → **crash holding
   lease** → resume reclaims → complete; plus paused/cancelling/failed sessions.
3. Add ACP error-path tests: corrupt/oversized payloads, sequence gaps, lease
   rollback, malformed event filenames, immutable-rewrite rejection.
4. Re-measure after each batch; stop at ≥90.

**Hygiene:** seed randomness; tests pass under `go test -race -count=2 ./internal/core/`.

**Done when:** `internal/core` ≥90%; `CORE_MIN` ratcheted to the new measured value.

---

## R3 — Drive overall 77.1% → ≥85% (rest of T6)

**Why:** A6 requires overall ≥85. ~850 uncovered statements package-wide.

**Do:**
1. R2 lands most of it (core is the largest package).
2. Then top up `internal/cmd` (69.2%) — the next-largest gap — via the brain
   driver + program command paths using the recording-runner seam.
3. Check `internal/mcp` (88.9%) and `internal/integration` for cheap remaining
   handler/error branches.
4. Re-measure; bump `OVERALL_MIN` to the new measured value.

**Done when:** overall ≥85%; `OVERALL_MIN` ratcheted; never lower a floor.

---

## R4 — Flip waves green + final CI evidence (finishes T7)

**Depends on:** R1, R2, R3.

**Do:**
1. `make ci` end-to-end; capture green output as exit-gate evidence.
2. `golangci-lint run ./...` exit 0 (keep clean as new tests land — gosec is
   excluded for `_test.go`, but watch `gocritic`/`unused` in non-test helpers).
3. In `specs/progress.md`: flip W1 🟡→✅ (after R1), W2 🟡→✅ (after R2+R3 floors
   hit targets). W3/W4 already ✅.
4. Tick the corresponding boxes in each spec's `tasks.md` and in this file.

**Done when:** `make ci` exits 0; `progress.md` shows W1–W4 ✅; all program exit
criteria in `progress.md` green.

---

## Verification commands

```bash
export PATH=$PATH:$(go env GOPATH)/bin

# coverage by package + overall
go test ./... -coverprofile=cov.out && go tool cover -func=cov.out | tail -1
go test ./internal/core/... -cover
go tool cover -func=cov.out | grep 'cmd/brain.go'

# recovery / race
go test -race -count=2 ./internal/cmd/ ./internal/core/

# gate + lint + full CI
bash scripts/coverage-check.sh
golangci-lint run ./...
make ci
```
