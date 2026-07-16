# Testing specd

specd is Go, standard library only, zero runtime dependencies. The whole suite runs with the `go`
toolchain — no frameworks, no fixtures, no network. This guide is the reference `.github/workflows/ci.yml`
points at.

## Running the suite

```bash
go test ./... -race -count=1      # full suite, as CI runs it
go test ./... -count=2            # order/iteration-order dependence catch (F4)
go test ./internal/cmd -run TestLifecycleE2E -count=1   # one test by name
./scripts/production-smoke.sh                           # installed lifecycle lane
```

CI runs `-race -count=1` and `-count=2` on `{ubuntu, macos} × {go 1.26.x, stable}`. Both legs must
be green before merge.

### Installed production lifecycle

`scripts/production-smoke.sh` starts from an empty temporary Git repository and drives an installed
binary through `init`, `new`, approvals, `context`, `verify`, task completion, `review`, and
`submit`. It first attempts an invalid phase jump and requires both a non-zero exit and the
documented next gate. Use `SPECD_BIN=/absolute/path/to/specd` to test an already-built binary;
otherwise the script builds one from the current checkout. `--negative` stops after proving the
fail-closed step.


### The `-count=2` leg (order-dependence)

A second, back-to-back run surfaces tests that leak state or depend on map/iteration order —
golden-output tests are the usual suspects. It is a required CI leg, not a nicety: a suite that
passes once but fails on the second run is flaky and blocks release. Keep it green; if it ever
fails, the offending test is order-dependent and must be fixed (isolate its temp state, sort before
asserting), never retried away.

## Coverage floor

Coverage is enforced by `scripts/coverage-check.sh` (CI job **Coverage floor**), which produces
`coverage.out` and fails if **total** statement coverage drops below the policy floor.

```bash
go test ./... -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out | tail -1     # total
./scripts/coverage-check.sh                    # enforce the floor
```

**Policy floor: 75.0%** (total). It is a **ratchet** — raise it as coverage climbs, never lower it.
Measured total at the floor's last update: **75.7%**, leaving ~0.7% headroom for atomic-mode
run-to-run jitter.

### Per-package coverage (reference snapshot, total 75.7%)

| Package | Coverage | Notes |
|---|---|---|
| `internal/cli` | 96.3% | Arg parsing. |
| `internal/cmd` | 73.9% | Verb handlers; agent-facing surface. |
| `internal/context` | 85.6% | Context manifest + HUD. |
| `internal/core` | 73.0% | State, DAG, evidence, gates plumbing. |
| `internal/core/gates` | 83.5% | Validation gates. |
| `internal/core/gates/security` | 88.5% | Opt-in security gate. |
| `internal/core/verify` | 51.2% | Verify exec/sandbox — much is OS-path guard that only fires without `bwrap`. Lowest package; the standing gap-closing target. |
| `internal/integration` | 100.0% | Role/steering conformance. |
| `internal/mcp` | 88.2% | MCP stdio server + tool-call marshaling contract. |
| `internal/orchestration` | 82.3% | Brain/Pinky leases, ACP ledger, decisions. |

`main.go`, `internal/core/embed_templates`, and `internal/version` carry no tests (embeds and a
version string) and do not move the total meaningfully.

Agent-facing surfaces — the MCP contract (`internal/mcp/`), the help palette (`help --json` schema),
and the gate registry — are all above the floor and pinned by contract tests (`TestSplitArgumentsContract`,
`TestHelpJSON`, the `gates` parity/conformance suites).

## Regression harness (`scripts/regress-domains.sh`)

One harness re-asserts each domain's owned invariant black-box against a freshly built binary in
a throwaway copy of the tree, so probes that mutate `.specd/` never touch the working repo:

```bash
./scripts/regress-domains.sh   # per-domain black-box invariant checks
```

It runs in CI (the `regression` job) and again as a release input gate before GoReleaser
publishes a tag.

## Stress / crash-safety jobs

Cross-process concurrency and crash-safety are proven by dedicated CI jobs (not the unit suite):

| Script | CI job | Proves |
|---|---|---|
| `stress.sh` | Concurrency stress | Cross-process contention on one spec. |
| `stress-acp.sh` | ACP ledger stress | ACP append/replay survives interleaving. |
| `stress-checkpoint-fault.sh` | Checkpoint fault-injection | Crash mid-checkpoint → no double-claim / no orphaned lease. |
| `stress-orchestration.sh` | Orchestration stress | Brain/Pinky contention. |
| `stress-program.sh` | Program scheduler stress | Cross-spec scheduling contention. |
| `stress-brain-recovery.sh` | Brain recovery stress | Retry/reclaim paths. |

Run any locally the same way CI does, e.g. `./scripts/stress-acp.sh`.

## Lint gates

```bash
gofmt -l .            # must be empty
go vet ./...
go mod tidy           # no diff (zero runtime deps; no go.sum)
./scripts/test-lint.sh   # test-suite structural lint
./scripts/docs-lint.sh   # CHEATSHEET ↔ command-reference sync + drift-guard
golangci-lint run        # v2 config in .golangci.yml
```

## Windows note

`specd update` self-replacement is known-limited on Windows (a running binary can't overwrite
itself). The CI **Build** job compiles on Windows and all non-self-update commands work there; the
self-update path is exercised only on Unix.

---

**See also:** [docs/contributor-guide.md](docs/contributor-guide.md) ·
[docs/observability.md](docs/observability.md) · [scripts/README.md](scripts/README.md)
