# Discrepancy Log — Analysis Plan vs. Live Repository

Produced per the action prompt's Validation Requirement: all `internal/` packages,
the security surface, the performance-critical paths, and the CI/CD/docs surface
were re-inspected against the live repository before any spec was written. Every
entry below is evidence-backed (file:line) and changes scope or design relative to
`specd-optimization-analysis-plan.md`.

## Critical — changes scope

### D1. R4's acceptance signal ("never shell string interpolation") is wrong as written

- **Claim (plan R4 / §6 Security):** "Command execution uses `exec.Command` with
  explicit args (never shell string interpolation)."
- **Observed:** `internal/runner/runner.go:58` runs `exec.CommandContext(ctx,
  spec.Shell, "-c", spec.Command) //nolint:gosec`. `verify:` lines and custom-gate
  commands are deliberately interpolated into a `sh -c` string — the same pattern
  is reused for the bwrap (`runner_sandbox.go:61`) and container
  (`runner_sandbox.go:113`) backends. `SECURITY.md` documents this explicitly:
  "`specd verify` runs each task's `verify:` line via `sh -c` … as the invoking
  user — real code execution," because `verify:` commands need real shell
  semantics (pipes, redirects, `&&`). Mitigations are env scrubbing and NUL-byte
  rejection (`internal/cmd/verify.go`), not argv isolation.
- **Impact:** Banning `sh -c` would break the `verify:` contract entirely. This is
  a documented design decision, not an oversight.
- **Resolution:** S1 (security-hardening) drops "eliminate shell interpolation"
  as a goal. It instead hardens the things that *are* gaps: MCP-boundary schema
  validation (see D2) and closes any unvalidated string concatenation outside the
  documented `verify:`/custom-gate execution path.

### D2. `scripts/install.sh` already has checksum verification — F2 is refuted

- **Claim (plan F2):** "`scripts/install.sh` uses `curl | bash` pattern" with no
  checksum verification; security risk.
- **Observed:** `scripts/install.sh:48-69` `verify_checksum()` downloads
  `SHA256SUMS` for the release and runs `sha256sum -c`/`shasum`, calling `die`
  (exit 1) on mismatch or missing tool. It runs unconditionally at line 217
  unless `--no-verify` is passed, which prints a loud `warn`. The only unverified
  path is build-from-source (`git clone` + `go build`, lines 118-141), which
  fetches no pre-built binary and so has nothing to checksum.
- **Impact:** None — already meets the intent of F2/R3's acceptance signal.
- **Resolution:** S1 drops "add SHA-256 checksum verification" as a task. It adds
  only a regression test asserting `--no-verify` requires explicit opt-in and a
  doc note in `SECURITY.md` cross-referencing the existing mechanism (already
  partially documented).

### D3. Path-traversal hardening (D3/R3) is already complete, not a gap

- **Claim (plan D3/R3):** File paths need traversal-safety hardening across
  `internal/cmd/` file operations.
- **Observed:** `internal/core/slug.go:8-23` defines `SlugRE =
  ^[a-z0-9][a-z0-9-]*$` and `ValidateSlug`, which is called before every
  filesystem operation that derives a path from a spec slug — confirmed across
  18 call sites (`orchestration.go:314,398,482,533`, `pinky.go:303`,
  `specfiles.go:425`, `acp.go:234`, `acp_lease.go:443`,
  `program_lease.go:43,367`, `runtime_paths.go:188,213`,
  `program_state.go:78,86`, `cmd/new.go:27`, `cmd/mcp.go:50`,
  `mcp/resources.go:102`). The remaining `filepath.Join`/`os.*` call sites
  operate on operator-supplied `--root`/`--out` CLI flags, not spec-derived
  strings, which is outside the spec-slug threat model.
- **Impact:** None — no unsanitized spec-slug-to-filesystem call site exists.
- **Resolution:** S1 reframes this from "harden" to "regression-guard": add a
  fuzz/property test pinning `ValidateSlug` behavior so future call sites can't
  silently skip it, rather than writing new sanitization code.

### D4. bwrap/container sandboxing is already fail-closed with no bypass

- **Claim (plan F5/D4):** bwrap sandboxing is fail-closed "but requires external
  dependency"; should "add graceful degradation with warning."
- **Observed:** `internal/runner/runner_sandbox.go:37-43` `newBwrapRunner()`
  calls `exec.LookPath("bwrap")`; if absent, it returns an error ("bubblewrap
  not found on PATH — refusing to run unisolated…") instead of constructing a
  runner — `SelectRunner` (`runner.go:19-30`) propagates that error with no
  fallback to the unisolated `shRunner`. The container backend
  (`runner_sandbox.go:76-91`) is equally fail-closed (requires a docker/podman
  binary **and** `SPECD_SANDBOX_IMAGE`). The only way to get unisolated
  execution is the explicit `--sandbox none` (the default).
- **Impact:** The plan's recommendation ("add graceful degradation with
  warning") would *weaken* INV3 (fail-closed sandboxing) — explicitly the wrong
  direction.
- **Resolution:** S1 drops "graceful degradation" entirely. It adds only a
  `doctor`-surfaced diagnostic so users see *why* verify refuses to run before
  they hit the error, without changing fail-closed behavior.

### D5. R2/D2 — DAG/frontier code is not in `internal/worker/`

- **Claim (plan §3, R2):** DAG computation and frontier dispatch live in
  `internal/worker/`.
- **Observed:** `internal/worker/` (262 LOC) only handles process execution of
  already-dispatched work; the actual dependency-graph and frontier logic is in
  `internal/core/dag.go` and `internal/core/frontier.go`. The frontier is
  **recomputed from scratch** (a fresh O(V+E) scan) on every task-completion
  event rather than maintained incrementally, so a full orchestration run costs
  O(V·(V+E)) rather than O(V+E) amortized.
- **Impact:** Real perf finding, but the affected module is `internal/core`, not
  `internal/worker`. S2's affected-modules list and benchmarks target
  `internal/core/dag.go`/`frontier.go`, not `internal/worker/`.
- **Resolution:** S2 rewritten to target the correct files; adds incremental
  frontier maintenance (track in-degree deltas instead of rescanning) as the
  primary optimization.

### D6. `internal/spec/` does not parse tasks — F-claim about parser location is wrong

- **Claim (plan §6 Affected Modules):** "`internal/spec/` — Parser performance,
  memory allocation optimization."
- **Observed:** `internal/spec/` (275 LOC) holds spec-status types/validation
  only. The actual `tasks.md` parser is `internal/core/tasksparser.go`, which
  reads the whole file into memory (no streaming) and is correctly sized for
  CLI-scale inputs (specs are hand-authored markdown, not multi-MB files) — no
  allocation hotspot found.
- **Impact:** Low priority; no streaming rewrite is justified at this scale.
- **Resolution:** S2 drops parser-streaming work. If profiling later shows a
  real hotspot, it targets `internal/core/tasksparser.go`, not `internal/spec/`.

### D7. Structured logging already exists — F6/R10 narrows to metrics + tracing only

- **Claim (plan F6):** "Observability gap: No clear logging framework"; recommend
  "audit `internal/obs/` for structured logging; add `slog` or similar."
- **Observed:** `internal/obs/log.go` (289 LOC) already implements structured
  `log/slog`-based logging with a tee handler. There is no metrics emission
  (counters/timers/histograms) and no tracing (OpenTelemetry or equivalent)
  anywhere in the repo.
- **Impact:** S5 (observability) drops "integrate `log/slog`" as a task —
  logging is done. Scope narrows to metrics + optional tracing hooks only.
- **Resolution:** S5 rewritten around R10's metrics/tracing gap exclusively.

### D8. `scripts/docs-lint.sh` already exists — R13's acceptance signal is refuted

- **Claim (plan R13):** Acceptance signal is "`scripts/docs-lint.sh` passes,"
  implying the script may not exist (per analysis-plan U1).
- **Observed:** `scripts/docs-lint.sh` exists (60 lines, executable). It checks
  (1) dead/deprecated command references in README.md/AGENTS.md/`docs/*.md`
  against `.specd/specs/cmd-audit/audit.csv` rows marked merge/deprecate, and
  (2) that `docs/command-reference.md`'s cheat-sheet table matches a hardcoded
  20-command list and `.specd/specs/CHEATSHEET.md` has exactly 20 rows.
- **Impact:** No new script needed. The hardcoded 20-command list and the
  `.specd/specs/cmd-audit/audit.csv` dependency are themselves a maintainability
  risk worth flagging (see S7).
- **Resolution:** S7 audits/extends the existing script rather than authoring a
  new one; flags the hardcoded list as a finding.

## Moderate — refines scope, doesn't change direction

### D9. `.golangci.yml` is current, not stale — F12 partially refuted

- **Claim (plan F12):** "May be using outdated linters... Update to latest
  schema; enable additional linters (e.g., `gosec`, `revive`)."
- **Observed:** Schema is already v2 (`version: "2"`, current). `gosec` is
  **already enabled** (`.golangci.yml:20`) with documented exclusions for
  `_test.go` and known G304 false positives. `revive` is **not** enabled. No
  cyclomatic/cognitive-complexity linter (`gocyclo`/`gocognit`) exists at all.
- **Resolution:** S3 narrows to: add a complexity linter (the real gap) and
  evaluate `revive` (the one part of F12 that was correct).

### D10. Complexity hotspots are real and specifically located

- **Observed (spot check, branch-keyword density on 5 longest functions):**
  - `internal/cmd/pinky.go:14` `RunPinky` — 171 lines, 49 branch keywords.
  - `internal/core/orchestration_driver.go:98` `DriveOrchestration` — 155
    lines, 32 branches.
  - `internal/cmd/init.go:136` `runInitWithRuntime` — 282 lines, 27 branches
    (longest function in either package).
  - `internal/cmd/doctor.go:67` `runDoctor` — 146 lines, 27 branches.
  - `internal/core/acp.go:283` `validateACPPayload` — 173 lines, 18 branches
    (mostly a large switch).
- **Resolution:** S3 names these five functions directly as refactor targets
  instead of a generic "reduce complexity" task.

### D11. Package-level doc comments are missing in the largest packages

- **Observed:** Package docs exist in `internal/mcp`, `internal/spec`,
  `internal/worker`, `internal/context`, `internal/testharness`; **absent** in
  `internal/core`, `internal/cmd`, `internal/cli`, `internal/runner`,
  `internal/pack`, `internal/schema` — i.e., absent in the two largest packages.
  Sampled exported symbols without doc comments: `internal/core/state.go:250`
  `LoadState`, `internal/core/orchestration.go:91` `OrchestrationPolicy`,
  `internal/cmd/next.go:26` `RunNext`.
- **Resolution:** S3 lists the 6 packages needing a package doc comment by name.

### D12. MCP input validation is partial, not absent

- **Claim (plan R5):** "JSON Schema validation on all inputs; fuzz testing"
  needed.
- **Observed:** A 1 MiB body size limit is enforced
  (`internal/mcp/transport.go:27`, `transport_http.go:199-206`). There is **no**
  schema-validation layer at the MCP boundary itself —
  `internal/mcp/server.go:387-417` `buildArgv()` converts the `arguments` map
  into a CLI argv with only a type check on `args`, then defers to each
  command's own validation (e.g. `ValidateSlug`).
- **Resolution:** S1 keeps R5 but scopes it precisely: add an argument-shape
  validation gate in `buildArgv()`/`enforceBoundedToolCall`, not a blanket
  "JSON Schema everywhere" rewrite — downstream per-command validation already
  exists and should not be duplicated.

### D13. `.goreleaser.yml` — SBOM already present; `-trimpath` and signing are the real gaps

- **Claim (plan F1/R12):** Generic "reproducible builds; SBOM generation."
- **Observed:** `.goreleaser.yml:44-45` already generates a CycloneDX SBOM via
  syft. `-trimpath` is **absent** — `ldflags` only has `-s -w`
  (`.goreleaser.yml:22-23`). Artifact signing is **absent and explicitly
  deferred**: a comment at lines 40-41 states "Artifact signing remains a
  documented deferral until signing-key management is in place."
- **Resolution:** S6 adds `-trimpath` (low-risk, high-value) and treats signing
  as a flagged-but-deferred item requiring a key-management decision outside
  this review's scope — not a silent gap to fill unilaterally.

### D14. README's Windows statement is narrower than claimed

- **Claim (plan F4):** "POSIX-only on Windows — run under WSL" (blanket).
- **Observed:** `README.md:68`: "Brain/Pinky worker orchestration is
  POSIX-only on Windows and fails fast with: `orchestration requires a POSIX
  shell (sh); not supported on Windows — run under WSL`." General `specd` usage
  works on Windows with a bash-like shell on PATH; only Brain/Pinky
  orchestration is restricted.
- **Resolution:** S7 corrects scope: document the Brain/Pinky-specific
  limitation precisely rather than implying specd is unusable on Windows.

### D15. Stress targets confirmed to lack resource bounds (F3 confirmed correct)

- **Observed:** `Makefile:50-66` `stress*` targets invoke their scripts with no
  `ulimit`/timeout flags; `grep -n "ulimit\|timeout" scripts/stress*.sh` returns
  zero matches across all stress scripts.
- **Resolution:** F3 stands as written. S4 adds `ulimit`/timeout wrappers.

### D16. `go vet ./...` is clean

- **Observed:** Ran clean, no issues. Confirms R1's "no panic in non-init
  paths" baseline is not contradicted by vet output; R1 work in S3 is additive
  hardening, not a fix for an existing vet failure.

## Out-of-band finding (not in original plan)

### D17. `AGENTS.md` contains an unrelated embedded instruction block

- **Observed:** `AGENTS.md:252-293` contains a fenced block of instructions for
  an unrelated third-party tool ("RTK") with no connection to specd. This is
  real committed content, not a tool artifact (verified via direct file read
  and `wc -l`/`od -c`).
- **Impact:** Dead weight in the agent-facing instructions file; risks
  confusing coding agents reading `AGENTS.md` for actual specd conventions.
- **Resolution:** Added to S7 (documentation-hygiene) as a removal candidate —
  flagged for the user's confirmation before deletion since its provenance is
  unclear (out of scope for this review to silently delete without a decision
  gate).

## No discrepancy — confirmed as claimed

- F1 (stdlib-only `go.mod`, ~104 bytes): confirmed (106 bytes), and the
  default-build stdlib-only property is enforced by
  `TestDefaultLinksNoDriver` — optional Redis/Postgres backends are
  build-tag-gated, never linked by default.
- F9 (no root `go.sum`): confirmed and *expected* — stdlib-only module needs
  none. Not a gap.
- F11 (`AGENTS.md`/`TESTING.md` sizes, no TOC): confirmed exactly (293 / 265
  lines, zero `##`-anchored TOC in either).
- No genuinely stale command references found in README.md/AGENTS.md against
  `internal/cmd/` — apparent hits were prose false positives ("specd root
  directory", etc.).
