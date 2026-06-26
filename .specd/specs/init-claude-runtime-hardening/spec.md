# Spec — Init hardening: CLAUDE.md, runtime gitignore, spec-scoped mission files

**Slug:** `init-claude-runtime-hardening`
**Status:** draft
**Owner:** init / scaffold subsystem

---

## 1. Problem

Three independent gaps in the `specd init` scaffold and the orchestration runtime:

1. **Claude Code does not auto-load `AGENTS.md`.** Claude Code loads `CLAUDE.md`
   (with `@import` support); it ignores `AGENTS.md`. A user who runs
   `specd init --agent claude-code` gets the full agent prompt pack in
   `AGENTS.md` but Claude Code never reads it, so the harness contract is
   invisible to the agent.

2. **`.specd/runtime/` is not git-ignored.** `git check-ignore .specd/runtime`
   returns NOT-IGNORED. The root `.gitignore` has no `.specd` entry and `init`
   writes no gitignore. Machine-local orchestration state (sessions, leases,
   cursors, events, missions) is therefore stageable and gets committed/pushed.

3. **No persistent, spec-scoped mission file.** Missions are written today only
   to an ephemeral system-temp file `specd-mission-*.json`
   (`internal/worker/shell_runner.go:38`) — random-unique but spec-agnostic and
   deleted after the run. There is no canonical on-disk mission record keyed by
   spec, so concurrent specs cannot be told apart on disk and a task can be
   re-issued/duplicated across runs without a stable, inspectable artifact.

---

## 2. Current architecture (analysis)

### Init flow
- `RunInit` (`internal/cmd/init.go:92`) → `runInitWithRuntime` →
  `core.PlanInit(options, core.DefaultScaffoldManifest(), core.ReadTemplate)`
  → `core.ExecuteInitPlan`.
- **Single source of truth for scaffold files:** `DefaultScaffoldManifest()`
  (`internal/core/scaffold.go:28`). Each entry is a
  `ScaffoldAsset{Template, Target, Policy, Required, Refresh}`.
  - `Policy = ScaffoldCreate` → write if absent (skip if present unless
    `--force`/`--refresh`+`Refresh`).
  - `Policy = ScaffoldMarkerMerge` → idempotent managed-marker merge (only
    `AGENTS.md` today), preserves user content outside the markers.
- Templates are embedded under `internal/core/embed_templates/`, read via
  `core.ReadTemplate` (`internal/core/embed.go:12`).
- **Agent selection** (`claude-code`, `cursor`, …) resolves in
  `runInitWithRuntime` *after* the plan is built (`resolveInitSelection`,
  `selected.Selected []string`). Precedent for post-plan mutation exists:
  the orchestration block already rewrites `plan.Actions` after planning
  (`internal/cmd/init.go:294-323`).
- `.claude/agents/pinky-*.md` are already written **unconditionally** by the
  manifest (`scaffold.go:66-73`) — i.e. external (non-`.specd`) targets are
  already a supported, exercised path in `executeFreshInitPlan`
  (`initplan.go:366-394`).
- Fresh init stages all `.specd/`-prefixed actions into a temp dir then renames
  atomically; non-`.specd` targets are written directly
  (`initplan.go:339-397`). Any new `.specd/...` asset is staged automatically.

### Runtime paths
- `ACPRuntimePaths` (`internal/core/runtime_paths.go`) is the validated path
  helper rooted at `.specd/runtime`. It exposes `SessionsDir`, `WorkerDir`,
  `EventPath`, `ArtifactsDir`, `ProgramSessionsDir`, etc., each guarding against
  traversal and symlinks. There is **no** `MissionsDir`/`MissionPath` helper.
- Session-scoped artifacts are keyed by a 32-char opaque session ID and are
  already collision-free. Missions are the one orchestration artifact with no
  canonical runtime home.

### Mission writes (full trace)
| Location | Path | Lifetime | Spec-scoped? |
|---|---|---|---|
| `internal/worker/shell_runner.go:38` | `os.CreateTemp("", "specd-mission-*.json")` | deleted post-run | no |
| `internal/testharness/pinky.go:123` | `mission-*.json` (test) | test temp | no |
| `specd dispatch` | stdout only (no file) | n/a | n/a |

---

## 3. Requirements (EARS)

- **R1.1** When `specd init` selects or detects the `claude-code` host, the
  system shall write a project-root `CLAUDE.md`.
- **R1.2** The generated `CLAUDE.md` shall import `AGENTS.md` (`@AGENTS.md`) so
  `AGENTS.md` remains the single source of truth and no prompt content is
  duplicated.
- **R1.3** `CLAUDE.md` shall use an idempotent managed-marker merge so re-running
  init and user edits outside the markers are preserved (parity with
  `AGENTS.md`).
- **R1.4** When `claude-code` is **not** selected/detected, `specd init` shall
  not create `CLAUDE.md`.
- **R2.1** `specd init` shall ensure `.specd/runtime/` contents are git-ignored
  by writing a managed gitignore so runtime state is never staged.
- **R2.2** The gitignore shall ignore all runtime contents while keeping the
  ignore file itself tracked so the policy is visible and stable
  (`*` + `!.gitignore`).
- **R2.3** `specd init --repair`/`--refresh` shall restore the runtime gitignore
  if missing.
- **R3.1** The system shall define a canonical, validated runtime missions path
  `.specd/runtime/missions/<slug>-<taskID>-<attempt>.json` via `ACPRuntimePaths`.
- **R3.2** Each segment (`slug`, `taskID`, `attempt`) shall be validated (reuse
  `ValidateSlug` and existing segment validators) and reject traversal/symlinks,
  consistent with the rest of `ACPRuntimePaths`.
- **R3.3** Mission persistence shall write to the spec-scoped path so two specs
  (or two attempts of one task) never share a filename, removing duplicate-task
  ambiguity on disk.
- **R3.4** Mission filenames shall be deterministic given `(slug, taskID,
  attempt)` so a re-issued attempt overwrites its own record rather than
  creating a duplicate.

---

## 4. Design / action plan

### R1 — CLAUDE.md
- Add embed template `internal/core/embed_templates/CLAUDE.md`:
  ```markdown
  <!-- specd:begin -->
  # Project agent guide

  This project uses `specd`. The full harness contract, workflow, and steering
  pointers live in AGENTS.md, imported below.

  @AGENTS.md
  <!-- specd:end -->
  ```
  (Use the exact managed-marker tokens the AGENTS.md merge already recognizes —
  confirm in `internal/core` marker logic before authoring.)
- Make the manifest claude-aware. Two viable approaches:
  - **(A — chosen) Conditional asset.** Append a `ScaffoldAsset{Template:
    "CLAUDE.md", Target: "CLAUDE.md", Policy: ScaffoldMarkerMerge, Required:
    false}` only when `claude-code ∈ selected.Selected` (or detected). Mirror the
    existing post-plan mutation pattern (`init.go:294-323`): build the action and
    splice it into `plan.Actions` after selection resolves, before
    `ExecuteInitPlan`. Satisfies R1.4 cleanly.
  - (B — rejected) Always write CLAUDE.md like `.claude/agents/*`. Simpler but
    violates R1.4 and pollutes non-Claude repos.
- `marker-merge` reuses `MergeAgentsMD`/`ValidateAgentsMD`; verify those are
  filename-agnostic (operate on any marker file) or generalize them.

### R2 — runtime gitignore
- Add embed template `internal/core/embed_templates/runtime.gitignore`:
  ```gitignore
  # specd runtime state — machine-local orchestration data. Do not commit.
  *
  !.gitignore
  ```
- Add manifest entry `ScaffoldAsset{Template: "runtime.gitignore", Target:
  ".specd/runtime/.gitignore", Policy: ScaffoldCreate, Required: true, Refresh:
  true}`. Target is `.specd/`-prefixed → staged + atomically committed by
  `executeFreshInitPlan`; creating the file also materializes
  `.specd/runtime/`.
- `Refresh: true` satisfies R2.3 for `--refresh`; `--repair` already restores
  missing `create` assets.

### R3 — spec-scoped mission path
- Extend `ACPRuntimePaths` (`internal/core/runtime_paths.go`):
  ```go
  func (p ACPRuntimePaths) MissionsDir() (string, error) {
      return p.join("missions")
  }
  func (p ACPRuntimePaths) MissionPath(slug, taskID string, attempt int) (string, error) {
      if err := ValidateSlug(slug); err != nil {
          return "", fmt.Errorf("acp runtime: invalid mission slug: %w", err)
      }
      if err := validateACPRuntimeSegment("task ID", taskID); err != nil {
          return "", err
      }
      if attempt < 1 {
          return "", fmt.Errorf("acp runtime: mission attempt must be >= 1")
      }
      name := fmt.Sprintf("%s-%s-%d.json", slug, taskID, attempt)
      return p.join("missions", name)
  }
  ```
  (Confirm `taskID` charset against `validateACPRuntimeSegment`; if task IDs like
  `T1` contain uppercase, add a dedicated validator/normalizer — the existing
  segment regex is `^[a-z0-9][a-z0-9-]*$`.)
- Update the mission writer to persist to `MissionPath(slug, taskID, attempt)`
  via `AtomicWrite` instead of (or in addition to) the ephemeral
  `shell_runner.go:38` temp file. The temp hand-off to the worker subprocess can
  remain, but the durable, inspectable record lives in runtime under a
  deterministic spec-scoped name (R3.3, R3.4).
- The runtime gitignore from R2 ensures these mission files are never committed.

### Ordering / dependencies
- R2 (runtime gitignore) should land **before/with** R3 so the new mission files
  are born ignored.
- R1 is independent of R2/R3.

---

## 5. Risks & verification

- **Marker reuse:** confirm the AGENTS.md merge engine is filename-agnostic
  before pointing it at `CLAUDE.md`; otherwise generalize (small refactor).
- **Task ID charset:** the runtime segment regex is lowercase-only; real task IDs
  (`T1`) may need a separate validator. Verify before wiring `MissionPath`.
- **Parity tests:** `SortedScaffoldTargets` and any manifest golden/parity test
  must be updated for the two new targets.
- **No double-write regressions:** decide whether the ephemeral temp file is
  replaced or supplemented; keep the worker contract unchanged.
- Gate the whole change behind `specd check` on a throwaway init + `go test
  ./internal/core/... ./internal/cmd/... ./internal/worker/...`.
