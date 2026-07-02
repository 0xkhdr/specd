# Wave 1 Baseline — 2026-07-02

Wave 1 scope from `specs/progress.md`: S1, S2, S4, S8, S15.

## Validation results

| Spec | Command | Result |
|------|---------|--------|
| S1 CLI commands | `go test ./internal/cmd/... -race -count=1` | PASS (`github.com/0xkhdr/specd/internal/cmd`, 8.553s) |
| S2 state atomicity | `make stress` | PASS: 16 workers × 20 iterations, 320 committed writes, final turn 320, final revision 321, state.json intact |
| S4 verify sandbox | `go test ./internal/runner/... -race -count=1` | PASS (`github.com/0xkhdr/specd/internal/runner`, 6.080s) |
| S8 install integrity | `shellcheck scripts/install.sh && bash scripts/install_test.sh` | PASS: 17 passed, 0 failed |
| S15 core parsers | `go test ./internal/core/... -race -count=1` | PASS (`github.com/0xkhdr/specd/internal/core`, 24.386s) |

## S1 — CLI command regression baseline

Dispatchable `cmd.Registry` commands: `init`, `handshake`, `new`, `approve`, `decision`, `midreq`, `memory`, `brain`, `pinky`, `next`, `verify`, `task`, `status`, `context`, `check`, `report`, `waves`.

Existing `--json` contract coverage in `internal/cmd/json_contract_test.go`:

| Command/path | Top-level JSON keys asserted |
|--------------|------------------------------|
| `status <spec> --json` | `spec`, `status`, `phase`, `counts`, `next` |
| `context <spec> --json` | `spec`, `status`, `skill`, `load`, `next` |
| `next <spec> --json` | `kind`, `id`, `task` |
| `next <spec> --dispatch --json` | `kind`, `count`, `packets` |
| `status --program --json` | `kind`, `count`, `specs` |
| `check <spec> --json` | `ok`, `violations`, `warnings` |
| `waves <spec> --json` | `waves`, `criticalPath`, `blockers` |
| `approve <spec> --json` | `ok`, `action`, `from`, `status`, `phase` |
| `check <spec> --json` error path | `ok`, `violations` |

Registry commands without explicit `--json` contract in this shared test: `init`, `handshake`, `new`, `decision`, `midreq`, `memory`, `brain`, `pinky`, `verify`, `task`, `report`. Some may have per-command tests; Wave 2 should close or document each gap.

## S2 — State atomicity baseline

Existing CAS/lock assertions:

- `internal/core/state_cas_test.go`: revision bump, stale concurrent write rejected, lock assertion panic/pass, vanished file conflict, corrupt/missing/newer-schema load rejection, nil map normalization.
- `internal/core/lock_test.go`: goroutine id sanity, lock release, reentrant lock, timeout on held lock, stale lock reclaim, schema validity after reclaim, contended writes keep schema valid.
- `internal/core/concurrency_test.go`: 32 goroutines serialize through `WithSpecLock`, every commit lands, final `Turn == committed == workers`, panic releases lock.

Gap for Wave 2: add explicit multi-save revision monotonicity over N writes and a torn-write guard if not already covered by write-failure tests.

## S4 — Verify and sandbox baseline

Current runner behavior from `internal/runner/runner_test.go`:

| Case | Expected behavior |
|------|-------------------|
| success | exit `0`, `TimedOut=false`, stdout captured |
| non-zero | original exit code preserved (`3` in test), `TimedOut=false` |
| stderr | stderr captured separately from stdout |
| timeout | exit `124`, `TimedOut=true` |

Host sandbox/tool availability:

- `bwrap`: `/usr/bin/bwrap`
- `docker`: `/usr/bin/docker`
- `podman`: `/usr/bin/podman`
- `nerdctl`: not found

Existing sandbox tests cover default `none`, unknown backend fail-closed, empty-PATH fail-closed for `bwrap`/`container`, and missing-image container failure message.

## S8 — Install integrity baseline

`verify_checksum` branches in `scripts/install.sh`:

- `NO_VERIFY=true`: warn and skip verification.
- `sha256sum` present: `sha256sum --ignore-missing -c SHA256SUMS`.
- `shasum` present: grep archive line, `shasum -a 256 -c -`.
- neither tool present: fail closed with clear error.
- failed checksum command: fail closed with `Checksum verification failed for <archive>`.

Host checksum tools:

- `sha256sum`: `/usr/bin/sha256sum`
- `shasum`: `/usr/bin/shasum`

Existing `scripts/install_test.sh` covers matching checksum, mismatch fail-closed, and `--no-verify` skip. Wave 2 should add explicit neither-tool PATH simulation if not already covered.

## S15 — Parser/fuzz baseline

Existing fuzz precedent: `internal/mcp/host_caps_fuzz_test.go` (`FuzzParseHostPrefs`) seeds malformed host capabilities and asserts conservative clamps/no panic.

Core parser tests available as seed sources:

- `internal/core/tasksparser_test.go`: valid task docs, annotations, missing keys, dependency parsing, round-trip stability.
- `internal/core/state_test.go` / `internal/core/state_cas_test.go`: valid state, corrupt JSON, missing fields, schema version cases, normalization.
- `internal/core/ears_test.go`: valid EARS forms, invalid criteria, case-insensitive forms, false-positive guards.
- `internal/core/specfiles_test.go`: spec artifact sync/traceability scenarios.

Good seed classes for Wave 2 fuzz targets: empty input, invalid UTF-8/JSON fragments, valid minimal documents, missing required sections/fields, overlong lines, duplicate task IDs, cyclic dependencies, newer schema, null maps/slices, mixed-case EARS, malformed markdown headings.
