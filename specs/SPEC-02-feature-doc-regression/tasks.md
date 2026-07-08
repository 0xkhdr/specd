# SPEC-02 Tasks: Feature ↔ Doc Regression

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-02-01 | Build verb→handler→doc map | Enumerate all 23 verbs from `commands.go`; match each to a `registry.go` handler and a `command-reference.md` entry. | A complete table with zero unmatched rows; any mismatch flagged for T-02-05. | Medium | completed |
| T-02-02 | Deferred-verb regression | Add a test asserting `triage` prints a deferral notice and exits 0. | Test passes; fails if `triage` silently no-ops or exits non-zero. | Small | completed |
| T-02-03 | Fail-closed regression | Add tests asserting an unknown verb exits 2 and a bad flag value exits 2. | Both tests pass and fail if exit code changes. | Small | completed |
| T-02-04 | Slug-position regression | Assert each verb reads the slug from the correct argv index (`brain`→argAt(1), others→argAt(0)). | Test passes; fails if any verb's slug index is wrong. | Medium | completed |
| T-02-05 | Orphan sweep | Flag handlers with no doc entry and documented behavior with no handler (incl. brain_worker.go/dispatch.go sub-behaviors); resolve each. | Every handler documented or recorded as intentionally internal; no documented behavior lacks a handler. | Medium | completed |
| T-02-06 | Normalize gate count to 14 | Replace all "12 core gates" (README:15, README:74, elsewhere) with 14 per authoritative `validation-gates.md`; file a drift-guard request for SPEC-07. | `grep -rn "12 core"` returns nothing; README internally consistent at 14; docs-lint green. | Small | completed |

## Task Dependency Graph

```
T-02-01 ─→ T-02-05
T-02-02 (parallel)
T-02-03 (parallel)
T-02-04 (parallel)
T-02-06 (parallel)
```
T-02-01 must precede the orphan sweep (T-02-05). The four regression tests and the gate-count
fix are independent and can run concurrently.

## Evidence Log

Verified against working tree at parent HEAD `4753f1b` (committed as this spec's completion commit).
All local gates green; PR/CI push deferred by user choice (see progress.md).

```
go build -o specd .                # Success
go test ./... -race -count=1       # 257 passed / 13 packages
go test ./... -count=2             # 514 passed (F4 iteration-order flake catch)
gofmt -l . (excl. reference/)      # empty
go vet ./...                       # No issues
go mod tidy                        # no diff (zero deps, no go.sum)
./scripts/test-lint.sh             # ok
./scripts/docs-lint.sh             # ok (CHEATSHEET ↔ command-reference identical)
grep -rn "12 core" (excl. ref/)    # nothing in shipped docs
```
golangci-lint deferred: binary not installed locally; gofmt + go vet clean. SPEC-01 config in `.golangci.yml` runs in CI.

### T-02-01 — verb → handler → doc map (23 verbs, zero unmatched)

23 verbs in `internal/core/commands.go`; 22 executable handlers in `internal/cmd/registry.go`
+ 1 deferred (`triage`). All 23 present as `### \`verb\`` entries in `docs/command-reference.md`
(pinned both directions by `TestSurfaceMatchesADR`; verb↔handler by `TestEveryCommandHasHandler`).

| Verb | Handler | SpecSlugArg | Doc | Verb | Handler | SpecSlugArg | Doc |
|------|---------|-------------|-----|------|---------|-------------|-----|
| help | runHelp | nil | ✓ | handshake | runHandshake | nil | ✓ |
| version | runVersion | nil | ✓ | brain | runBrain | argAt(1) | ✓ |
| init | runInit | nil | ✓ | report | runReport | nil | ✓ |
| new | runNew | nil | ✓ | link | runLink | nil | ✓ |
| approve | runApprove | nil | ✓ | unlink | runUnlink | nil | ✓ |
| midreq | runMidreq | nil | ✓ | review | runReview | argAt(0) | ✓ |
| decision | runDecision | nil | ✓ | submit | runSubmit | argAt(0) | ✓ |
| next | runNext | argAt(0) | ✓ | status | runStatus | nil | ✓ |
| task | runTask | nil | ✓ | check | runCheck | nil | ✓ |
| verify | runVerify | argAt(0) | ✓ | memory | runMemory | nil | ✓ |
| context | runContext | argAt(0) | ✓ | mcp | runMCP | nil | ✓ |
| triage | *deferred* | nil | ✓ | | | | |

Zero unmatched rows. Only `brain` reads its slug from argAt(1) (subcommand precedes slug);
every other slug-phase-checked verb reads argAt(0). Verbs with nil SpecSlugArg resolve no
fixed-position slug and are skipped by the phase gate — pinned by `TestSpecSlugArgPositions`.

### T-02-05 — orphan sweep: no orphans

- **Verb-level parity**: enforced both directions by `TestSurfaceMatchesADR` — no handler
  lacks a doc entry, no doc entry lacks a handler.
- **`brain` subcommands** (`start|step|run|status|cancel|resume`): all six documented at
  `command-reference.md` (`specd brain <start|step|run|status|cancel|resume>`). No orphan.
- **`brain_worker.go`**: exposes no CLI surface — internal helpers reached only from
  `runBrain`/`brain_run.go`. Intentionally internal.
- **`dispatch.go`** (`checkFlagEnums`, `checkPhase`): internal enforcement invoked by `Run`;
  their behavior is documented conceptually via the exit-code semantics in
  `command-reference.md`. Intentionally internal.

No dead sub-behavior to delete; no undocumented behavior to surface.
