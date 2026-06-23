# specd — Directory Structure Analysis & Restructure Plan

> Date: 2026-06-23
> Scope: Go project layout review against community best practices (Standard Go
> Project Layout, Effective Go package design, `internal/` visibility rules).

## 1. Current Layout

```
specd/
├── main.go                 # entrypoint (thin)
├── main_test.go
├── go.mod                  # module github.com/0xkhdr/specd, stdlib-only
├── Makefile
├── .golangci.yml
├── .goreleaser.yml
├── .github/workflows/      # ci.yml, release.yml
├── docs/
├── scripts/
└── internal/
    ├── cli/            (1 src)   flag/arg parsing
    ├── cmd/            (36 src)  command dispatch + brain subsystem
    ├── core/           (80 src)  EVERYTHING ELSE — god package
    ├── integration/    (11 src)
    ├── mcp/            (13 src)
    ├── obs/            (1 src)
    ├── testharness/    (9 src)
    └── worker/         (4 src)
```

## 2. Assessment

### What already follows best practice ✓

- **`internal/` for private packages** — correctly prevents external import.
- **Thin `main.go`** — logic lives in packages, entrypoint only wires version +
  dispatch. (Root `main.go` is idiomatic for a single binary; `cmd/specd/` is
  only needed when shipping multiple binaries.)
- **stdlib-only `go.mod`** — zero external deps, clean dependency surface.
- **Tooling present** — Makefile, golangci-lint config, goreleaser, CI/release
  workflows, issue templates, SECURITY.md, CONTRIBUTING.md.
- **Build artifacts gitignored** — `specd` binary and `*.out` coverage files are
  untracked (verified via `git ls-files`). Root clutter is local-only.

### Problems

| # | Severity | Problem |
|---|----------|---------|
| 1 | **High** | `internal/core` is a god package: 80 source files, all `package core`. High coupling, slow compile, no enforced boundaries, hard to navigate. Filenames already encode the missing package seams (`acp_*`, `backend_*`, `orchestration_*`, …). |
| 2 | Medium | `internal/cmd` (36 files) mixes command dispatch with the multi-file `brain` subsystem (`brain.go`, `brain_commands.go`, `brain_policy.go`, `brain_worker.go`). |
| 3 | Low | 28 `*_cov_test.go` files in core are coverage-padding tests — a smell, not structural. |
| 4 | Low | Coverage `.out` files clutter the working root (gitignored, so cosmetic only). |

## 3. Root Cause: `internal/core`

One flat namespace holds at least 12 distinct concerns. Cohesive clusters,
derived from existing filenames:

| Target package | Source files |
|---|---|
| `internal/acp` | `acp.go`, `acp_archive.go`, `acp_cursor.go`, `acp_lease.go`, `acp_store.go` |
| `internal/backend` | `backend.go`, `backend_git.go`, `backend_postgres.go`, `backend_redis.go` |
| `internal/orchestration` | `orchestration*.go` (9), `cost_brake.go`, `frontier.go`, `blockers.go` |
| `internal/program` | `program.go`, `program_decide.go`, `program_lease.go`, `program_orchestration.go`, `program_session.go`, `program_snapshot.go`, `program_status.go`, `program_step.go` |
| `internal/pinky` | `pinky.go`, `pinky_brief.go`, `pinky_context.go`, `pinky_report.go` |
| `internal/pack` | `pack.go`, `pack_apply.go`, `pack_resolve.go` |
| `internal/gate` | `gates.go`, `customgate.go`, `ears.go`, `dag.go` |
| `internal/schema` | `schema.go`, `schema_validate.go` |
| `internal/report` | `report.go`, `report_metrics.go`, `prsummary.go`, `telemetry.go` |
| `internal/contextpkg` | `context_estimate.go`, `context_manifest.go`, `context_slice.go` |
| `internal/replay` | `replay.go`, `session_replay.go` |
| `internal/runner` | `runner.go`, `runner_sandbox.go` |
| `internal/core` (shared base, keep) | `paths.go`, `runtime_paths.go`, `slug.go`, `io.go`, `exit.go`, `env.go`, `lock.go`, `state.go`, `md.go`, `output.go`, `ui.go`, `render.go`, `help.go`, `embed.go`, `mode.go`, `mode_recommend.go`, `phases.go`, `agents.go`, `authoring.go`, `commands.go`, `commitlink.go`, `initplan.go`, `manifest_tools.go`, `scaffold.go`, `specfiles.go`, `tasksparser.go`, `taskview.go`, `task_complete.go` |

`internal/core` shrinks to a small shared-primitives package (paths, slug, io,
exit, env, state, render, embed). The domain packages depend on it; it depends
on nothing in the project.

### Target layout

```
internal/
├── core/            # shared primitives only (paths, io, slug, state, render…)
├── acp/
├── backend/
├── orchestration/
├── program/
├── pinky/
├── pack/
├── gate/
├── schema/
├── report/
├── contextpkg/
├── replay/
├── runner/
├── cli/
├── cmd/
│   └── brain/       # extract brain subsystem
├── integration/
├── mcp/
├── obs/
├── testharness/
└── worker/
```

## 4. Action Plan

Incremental. One cluster per step. `go build ./... && go test ./...` between
each. The dominant risk is **import cycles** — split leaf clusters first.

### Phase 0 — Prep
1. Branch: `restructure/core-split`.
2. Baseline: `go build ./... && go test ./... && go vet ./...` green.
3. Build the **actual cross-cluster dependency graph**, not just the API surface.
   Mapping `^func [A-Z]` / `^type [A-Z]` lists exported identifiers but does not
   show coupling *direction*. For each cluster, grep which *other* cluster's
   identifiers it references. The resulting edge list gives a real topological
   order — extract sinks (no outbound intra-project edges) first. This replaces
   the guessed leaf order in Phase 1.
4. **Audit `//go:embed` directives** (`grep -rn "go:embed" internal/core`). Embed
   paths are relative to the file's directory. Any file carrying a `//go:embed`
   that gets moved breaks the embed unless its assets move too. `embed.go` stays
   in `core`; flag every *other* file with an embed directive before moving it.
5. **Check coverage-threshold granularity.** Inspect `.golangci.yml`, `Makefile`,
   `.github/workflows/ci.yml`. If thresholds are enforced **per package**,
   splitting one 80-file package into 12 means each new small package is measured
   alone — a package previously carried by the core aggregate can now fail. Know
   this *before* the split, not in Phase 5.

### Phase 1 — Extract leaf clusters (no inbound deps from siblings)
Order: `schema` → `contextpkg` → `gate` → `pack` → `runner`.
Per cluster:
1. `git mv internal/core/<files> internal/<pkg>/` — include the cluster's
   `*_test.go` files (both white-box `package core` and black-box
   `package core_test`).
2. Change `package core` → `package <pkg>` in moved white-box files; change
   `package core_test` → `package <pkg>_test` in moved black-box test files.
3. Fix references: callers now import `internal/<pkg>` and qualify identifiers.
   Black-box test files (`package <pkg>_test`) import the package by its new path
   too — `go build` will *not* flag these; only `go test` does. Don't skip the
   test build.
4. Rename in-package unqualified uses that broke.
5. **Move co-located fixtures.** If a moved test reads from `core/testdata/...`,
   move those fixtures into `internal/<pkg>/testdata/` (paths are relative to the
   test file's directory).
6. `go build ./... && go test ./...`.
7. Commit per cluster (small, reviewable, bisectable).

### Phase 2 — Extract mid-tier clusters
`acp` → `backend` → `report` → `replay` → `pinky`.
Same procedure. If a cycle appears: push the shared **type** down into
`internal/core` (base) rather than creating a sideways dependency. But when the
coupling is **behavior, not a type** (cluster A *calls* cluster B and vice
versa), pushing a type down does not break it — define a **consumer-side
interface** in A and have B satisfy it implicitly. Type-push fixes data cycles;
interface seams fix behavioral cycles.

### Phase 3 — Extract heavy clusters
`orchestration` → `program` (these likely cross-reference each other and the
mid-tier; do last when the rest is stable).

### Phase 4 — Split `cmd/brain`
0. **Check `brain_worker.go` deps first.** If `brain` imports `internal/worker`
   AND `worker` imports `internal/cmd` → cycle once `brain` is its own package.
   Resolve with an interface seam before the move.
1. `git mv internal/cmd/brain*.go internal/cmd/brain/`.
2. `package brain`; export the dispatch entrypoints `cmd` needs.
3. Build + test.

### Phase 5 — Cleanup
1. Fold meaningful assertions from `*_cov_test.go` into behavior tests; delete
   pad-only files.
2. Re-run full suite + coverage; confirm thresholds in CI still pass.
3. `golangci-lint run` clean (watch for `revive` package-naming: avoid stutter
   like `contextpkg.Context` — pick a non-stuttering name, e.g. `ctxslice`).

### Optional (defer unless multi-binary planned)
- Move `main.go` → `cmd/specd/main.go`. Only worthwhile if a second binary
  (e.g. a standalone worker) is on the roadmap.

## 5. Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Import cycles between `orchestration`/`program`/`core` | Extract leaves first; push shared types down into base `core`; never import sideways. |
| Identifier renames cause large diffs | One cluster per commit; rely on compiler to surface every break. |
| Coverage thresholds drop after test cleanup | Do Phase 5 last; measure before/after; keep real assertions. |
| `internal/` import paths change | All consumers are in-repo; compiler catches every stale import. No external breakage (paths are `internal/`). |
| **`//go:embed` paths break** when a file carrying an embed directive moves | Audit embeds in Phase 0; keep `embed.go` in `core`; move embedded assets together with any other embedding file. Fails at build or, worse, embeds wrong tree silently. |
| **Black-box tests (`package *_test`) miss the build sweep** — 33 such files in the tree | `go build ./...` ignores `_test.go`; run `go test ./...` after every cluster move. Update their import paths + `package <pkg>_test` rename. |
| **`testdata/` fixtures orphaned** when their test moves | Move fixtures with the test; paths are relative to the test file's dir. |
| **Per-package coverage thresholds fail** after split | Confirm threshold granularity in Phase 0; if per-package, budget coverage work into each cluster commit, not just Phase 5. |
| **Behavioral cycle** `orchestration`↔`program` (`program_orchestration.go` already couples them) | Interface seam on the consumer side; type-push alone won't break a call-cycle. |

## 6. Test Layout — beside implementation (not a separate dir)

**Decision: keep tests beside the code they test. Do not introduce a `tests/`
directory.** This is idiomatic Go, enforced by the toolchain:

- `go test` discovers `*_test.go` per package directory. A separate test tree
  fights the tool.
- `_test.go` files are excluded from the build automatically — zero binary cost
  to living beside source.
- File-level coverage maps test → source within the same package.
- Two test styles, both **in the same directory** as the source:
  - white-box → `package <pkg>` (tests unexported internals).
  - black-box → `package <pkg>_test` (tests only the exported API). The repo
    already uses this — 33 files. Keep both styles co-located.
- The only directory Go reserves for tests is `testdata/` (build-ignored fixture
  subdir). The repo has two (`internal/cmd/testdata`, `internal/mcp/testdata`).
  That convention stays; fixtures move with their tests (see Phase 1, step 5).

A separate `tests/` tree is a Python/JUnit habit, not Go. No change to current
co-located layout. The restructure moves tests *with* their source package; it
does not centralize them.

## 7. Definition of Done

- [ ] `internal/core` ≤ ~30 files, shared primitives only.
- [ ] Each domain cluster is its own `internal/<pkg>`.
- [ ] `brain` extracted to `internal/cmd/brain`.
- [ ] No import cycles (`go build ./...` clean).
- [ ] `go test ./...` green, coverage thresholds met.
- [ ] `golangci-lint run` clean, no package-name stutter.
- [ ] Each step committed separately for bisectability.
- [ ] No broken `//go:embed` paths (`go build` clean + spot-check embedded tree).
- [ ] `go test ./...` green, including all `package *_test` black-box files.
- [ ] `testdata/` fixtures moved with their tests; tests beside source (no
      separate `tests/` dir introduced).
