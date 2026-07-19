# specd — Deep Review & Action Plan

**Date:** 2026-07-19 · **Branch:** `optimization` · **Method:** full-tree ponytail audit
(over-engineering hunt: every line must exist for a reason) + workflow/CI review.
Scope of this pass: complexity and process. Correctness/security bugs were not hunted here.

---

## 1. Health snapshot

| Check | Result |
|---|---|
| `go build` | OK |
| `gofmt -l .` | clean |
| `go vet ./...` | clean |
| `go test ./... -count=1` | all packages pass (~38s locally) |
| Runtime dependencies | zero (`go.mod` has no requires — invariant holds) |
| Size | ~47,300 lines of Go, 36 CLI verbs, 20 docs files, 20 shell scripts |

The core invariants (determinism, evidence integrity, atomic writes, CAS, byte-stable
parser) look intact and well-tested. The problems found are **surface-area problems**:
code and process that exist without a caller or without a payoff.

---

## 2. Ponytail audit — ranked findings

Format: `<tag> <what to cut>. <replacement>. [path]`

1. `delete:` `orchestration/a2a.go` — `ExportA2A`, `ImportA2A`, `SemanticACP`, `A2ASemanticACP` have **zero production callers**; only their own tests exercise them. Nothing. ~351 lines (impl + tests). [internal/orchestration/a2a.go]
2. `yagni:` adapter execution machinery unreachable from any CLI verb — `adapter.Run`, `NegotiateCapabilities`, `MatchIdentity`, `Historical`, the feedback trio (`ValidateFeedbackCommit`, `FeedbackRequest`, `FeedbackFromRequest`), `MissionRequest`. Production code uses only `Adapter`, `Request`, `SchemaVersion`, `MissionFromRequest`, `ExportOTel`. Since `internal/` can't be imported by external adapter authors either, this is a contract with no consumer. Delete or wire a verb that actually drives it. ~900 lines (impl + tests). [internal/adapter/{runner,feedback,identity,a2a}.go]
3. `yagni:` `docs/CHEATSHEET.md` is an enforced **byte-identical copy** of `docs/command-reference.md` (811 lines), maintained by hand-copying, policed by `scripts/docs-lint.sh` + a CI step. Replace with one file (or generate both from `specd help --json`, which already emits the machine-readable palette). Kills 811 duplicated lines, one script, one CI step, and the "edit then copy" ritual. [docs/CHEATSHEET.md, scripts/docs-lint.sh]
4. `yagni:` verb sprawl — 36 verbs; a large tier (`incident`, `recurring`, `release`, `deploy`, `spike`, `midreq`, `drift`, `eval`, `exception`, `memory`, `link`/`unlink`, `decision`, `handshake`) are thin append-a-ledger-record commands (20–154 lines each), but each one also carries ~25 lines of palette metadata, a docs section ×2 (see #3), tests, and MCP mapping. Consolidate the pure ledger verbs under one `specd record <kind>` verb, or defer the ones without a real user. [internal/core/commands.go, internal/cmd/*]
5. `delete:` deferred verb `triage` — declared, documented, exits 0 with a notice, does nothing. One deferred verb is a stub kept alive in palette, docs, and tests. Ship it or cut it. [internal/core/commands.go:598]
6. `stdlib:` three hand-rolled membership helpers — `contains` (config_validate.go:316), `contains` (cmd/dispatch.go:304), `containsString` (core/authority.go:136), `containsPhase` (core/driver.go:183) → `slices.Contains`. [multiple]
7. `stdlib:` three copies of `sortedKeys` — registry.go:680, core/prometheus.go:164, gates/intake.go:93 → `slices.Sorted(maps.Keys(m))`. Repo imports `"sort"` in 63 files but uses `slices` once; Go 1.26 is required, use it. [multiple]
8. `shrink:` `internal/cmd/registry.go` is 1,386 lines mixing the verb→handler map with ~15 full handlers (`runCheck`, `runInit`, `runStatus`, `runReport`, `runVerify`, …) plus git helpers plus JSON output helpers — contradicting the stated architecture ("one handler per verb lives in `internal/cmd/`"). Split handlers into their per-verb files; registry.go keeps only the map + dispatch plumbing. Same logic, honest layout. [internal/cmd/registry.go]
9. `yagni:` dual observability exporters — **RESOLVED**. Prometheus text format kept and documented as the one metrics export (`docs/observability.md`); the OTel JSON projection and `report --format otel` were deleted, OTel export being an external-adapter concern over the neutral `event/v1` stream (`docs/adapter-contract.md`). [internal/core/prometheus.go]
10. `shrink:` scripts sprawl — six `stress-*.sh` scripts (`stress.sh`, `-acp`, `-orchestration`, `-program`, `-brain-recovery`, `-checkpoint-fault`) with overlapping setup boilerplate. One parameterized `stress.sh <domain>` reduces maintenance and CI YAML. [scripts/]

**net: ≈ −2,600 lines, −1 doc file, −1 lint script, −5 scripts possible. 0 deps to cut (already zero).**

Not flagged: the `Gate` and `Scanner` interfaces (multiple real implementations),
the tasks parser, state/lock/CAS machinery, and the gates registry — all earn their lines.

---

## 3. Workflow & process review

### 3.1 CI pipeline (`.github/workflows/ci.yml`)

Findings:

- **~26 sequential steps in one job.** Lint, shellcheck, staticcheck, govulncheck, the full test suite **twice** (`-race` and `-count=2`), perf-gate, install-script tests, domain regressions, six contention/stress runs, a production smoke, and three cross-compiles — all serialized on every push. Thorough, but the PR feedback loop pays for release-grade validation every time.
- **Recommendation — tier it:**
  - *PR job (fast, parallel):* gofmt, vet, staticcheck, test-lint, `go test -race`, docs check. Target < 5 min.
  - *Merge-to-main job:* `-count=2`, regress-domains, stress/contention suite, perf-gate, install-script tests.
  - *Release workflow (already exists):* cross-compiles, production smoke, upgrade matrix.
- **Docs drift:** `CLAUDE.md` / `CONTRIBUTING` describe CI as "gofmt, go vet, go mod tidy check, and the scripts" — it actually also runs staticcheck, govulncheck, shellcheck, a coverage floor, and perf-gate. Contributors will be surprised by gates they weren't told to run. Sync the description or expose one `scripts/ci-local.sh` that mirrors CI exactly.

### 3.2 Documentation

- **CHEATSHEET duplication** — see finding #3. This is the single highest-leverage process fix: the current loop is *edit reference → manually copy → CI fails if you forgot*. A generated reference (from `specd help --json`) makes the palette the single source of truth and deletes the lint.
- **Contract docs for unreachable code** — `docs/adapter-contract.md`, `delivery-contract.md`, `operating-model-contract.md`, `telemetry-schema.md`, `scale-envelope.md`, `data-classification.md` document surfaces that live under `internal/` (unimportable) and, per finding #2, are partly undriven by any verb. Each contract doc should name the verb or file that proves the contract is live; anything that can't is aspiration and should be marked as such or removed (matches the repo's own "subtractive bias" rule).
- 20 docs files for a single-binary CLI is near the ceiling. `concepts.md` / `user-guide.md` / `README.md` overlap; consider folding concepts into the user guide.

### 3.3 Repo hygiene

- `coverage.out` sits untracked in the repo root (correctly gitignored via `*.out`) — delete the stray artifact; local coverage runs should target the scratch dir.
- `TESTING.md` at root plus `docs/` — one testing doc, one location.

### 3.4 What the workflow gets right (keep)

- Fail-closed dispatch (unknown verb → exit 2) with tests.
- Evidence-gated completion with no bypass flag — verified: no bypass path exists.
- `-count=2` order-dependence check and per-domain black-box regressions are unusual and valuable.
- Zero-dependency discipline enforced by `go mod tidy` diff check.
- Byte-stable parser round-trip tests.

---

## 4. Action plan

### Phase 1 — pure deletion (no behavior change, ~1 day)
1. Delete `internal/orchestration/a2a.go` + test. If the A2A envelope is a planned contract, keep only the doc and record the deferral (subtractive-bias rule).
2. Delete unreachable adapter machinery (`runner.go`, `feedback.go`, `identity.go`, `a2a.go` in `internal/adapter`) **or** file an issue naming the verb that will drive it and a deadline; keep only the symbols production uses today.
3. Cut the `triage` deferred verb (palette entry, docs rows, dispatch stub).
4. Remove stray `coverage.out`.
5. Run: `go build`, full suite, `regress-domains.sh` — all must stay green.

### Phase 2 — docs single-sourcing (~1 day)
6. Add `specd docs` (or a small `go run ./tools/gendocs`) that renders `docs/command-reference.md` from the `specd help --json` palette. Delete `CHEATSHEET.md` (or make it the generated output), repoint `docs-lint.sh` to "generated output matches committed file" — a real gate instead of a copy-check.
7. Sync `CLAUDE.md`/`CONTRIBUTING.md` with the actual CI gate list, or add `scripts/ci-local.sh`.

### Phase 3 — mechanical cleanups (~half day)
8. Replace hand-rolled `contains*` with `slices.Contains`; the three `sortedKeys` with `slices.Sorted(maps.Keys(m))`; migrate straightforward `sort.Strings`/`sort.Slice` call sites to `slices` where it's a drop-in.
9. Split `internal/cmd/registry.go`: keep map + dispatch + shared helpers; move each `run*` handler to its verb's file. No logic changes; diff should be pure moves.

### Phase 4 — CI tiering (~half day)
10. Split `ci.yml` into parallel PR jobs (lint / test / build) and move stress, contention, `-count=2`, perf-gate, and cross-compiles to a main-branch / nightly workflow.
11. Collapse the six stress scripts into one parameterized `stress.sh <domain>`.

### Phase 5 — decisions to record (needs an owner call, not code first)
12. **Verb consolidation:** decide whether the ledger verbs (`incident`, `recurring`, `release`, `deploy`, `spike`, `midreq`, `drift`, `decision`, …) have real users; consolidate under `specd record <kind>` or defer the unused ones. Record the decision either way.
13. **One metrics format:** Prometheus text or OTel JSON, not both.
14. **Contract-doc audit:** every `docs/*-contract.md` must cite its live driver (verb/test) or be marked historical.

### Guardrails for every phase
- Preserve the non-negotiables: determinism, evidence integrity, atomic writes/CAS/lock, byte-stable parser, zero deps, fail-closed dispatch.
- Each deletion lands as its own commit with the full gate set green (`gofmt`, `vet`, `test -race`, `test-lint.sh`, docs check, `regress-domains.sh`).
- Any "keep for later" survivor gets an issue + a dated deferral note — no silent zombies.

---

## 5. Summary

The engine is sound and the invariants are genuinely enforced. The cost center is
**unowned surface**: an A2A layer nobody calls, an adapter contract nothing can reach,
an 811-line hand-copied doc with a CI cop, a 36-verb palette where a third are thin
ledger writers, and a release-grade CI bill paid on every push. Roughly 2,600 lines and
several process steps can go with zero behavior change; the rest are recorded decisions.
The repo's own rule says it best: *when unsure, cut or defer and record the decision.*
