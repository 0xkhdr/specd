# Domain: CLI Architecture & Foundations

## 1. Purpose & value mapping
- **Principles served:** P1 (the harness is a deterministic, auditable binary), and it is
  the substrate for all others (atomic writes, locks, config).
- **Paper concept realized:** the harness as *tools + sandboxes + orchestration scaffolding*
  delivered as one calibrated artifact (pp.28–30). Zero-dependency and deterministic
  foundations are what make "most failures are configuration failures" (p.30) a claim you
  can actually audit — nothing hidden in a dependency tree.
- **Core use case:** every command dispatches through one flat registry; help can never
  drift from dispatch; every write is atomic; every mutating command holds a reentrant lock;
  config cascades deterministically (global→project→env). The plumbing that makes the whole
  harness trustworthy.
- **If none → CUT:** N/A — this is the floor.

## 2. Current-state analysis (from specd)
- **Reference files read:** `main.go`, `internal/cli/args.go`, `internal/cmd/registry.go`,
  `internal/core/{io.go,lock.go,paths.go,config_loader.go,config_validate.go,
  config_migrate.go,commands.go,help.go,exit.go,env.go,slug.go}`,
  `docs/contributor-guide.md`.
- **What exists today; key contracts/invariants:**
  - **Zero-dep custom parser:** `internal/cli/args.go` (~40 lines) + a flat dispatch table
    (`registry.go`, 29 entries) instead of Cobra/urfave/Viper. Recorded decision
    (`docs/contributor-guide.md`): zero-dep is a product value; the surface (~19–29 commands)
    is small and stable.
  - **Registry↔help single-source guard:** `TestRegistryMatchesHelp` fails if
    `cmd.Registry` (dispatch) and `core.Commands` (help metadata, `commands.go` ~52.9K)
    disagree — "dispatch and help can never drift."
  - **Atomic writes** (`io.go`): `AtomicWrite` = MkdirAll → CreateTemp in same dir →
    WriteString → fsync → Chmod 0644 → Rename; `AppendFile` fsyncs and returns Close error.
  - **Reentrant advisory lock** (`lock.go`): cross-process `O_CREATE|O_EXCL` `.lock`
    (pid+unix-ms) with stale reclaim (`SPECD_LOCK_STALE_MS`, 30s), in-process per-path mutex,
    reentrancy keyed by parsed goroutine id; `WithSpecLock[T]` / `WithProgramLock[T]`;
    acquire timeout `SPECD_LOCK_TIMEOUT_MS` (5s).
  - **Config:** YAML-only as of v0.2.0 (`parseSimpleYAML`, two-space indent, fails loud on
    truncated scalars); layers global→project→env onto `DefaultConfig`; validates +
    secret-scrubs `orchestration` before applying; `effectiveConfigDigest` = sha256.
    `paths.go`: `FindSpecdRoot` walks up → `NotFoundError` exit 3. `env.go`: `EnvInt` clamps
    + warns once. `slug.go`: `^[a-z0-9][a-z0-9-]*$`.
  - **Exit codes:** `ExitOK/ExitGate/ExitUsage/ExitNotFound`.
- **Redundancy / complexity / drift found (evidence):**
  - `commands.go` at ~52.9K is the single largest core file — help metadata for 29 commands.
    Shedding ~13 commands (triage) shrinks it substantially.
  - Config has grown a lot of surface (validate + migrate + scaffold + env + orchestration
    scrubbing); `config_migrate.go`/`config.json` legacy handling is dead weight for a fresh
    tree with no legacy.

## 3. Fresh-start decision
- **Verdict per capability:**
  - Zero-dep custom parser + flat registry — **KEEP** (validated decision; determinism +
    small surface). Re-affirmed: the fresh 16-verb surface makes Cobra even less justified.
  - Registry↔help single-source guard — **KEEP** (the "help can't drift" test is a model
    guardrail; extend it to drive MCP tool registration, domain 07).
  - Atomic writes / reentrant lock / CAS primitives — **KEEP** (hard invariants).
  - Config format — **KEEP YAML** (`parseSimpleYAML` subset, fails loud), but **SIMPLIFY**:
    drop legacy `config.json` runtime handling and `config_migrate.go` (migrate is CUT).
    New fields are added in the loader + validator only (no migration renderer).
  - `commands.go` help metadata — **SIMPLIFY** by construction: fewer verbs, and generate
    help/usage from the same registry entries where possible to shrink the hand-maintained
    table.
  - Program lock — **DEFER** with the program tier (domain 09).
- **Minimal accurate surface:**
  - `main.go` + `internal/cli/args.go` (parser) + `internal/cmd/registry.go` (dispatch,
    16 entries) + `internal/core/{io,lock,paths,config_loader,config_validate,commands,
    help,exit,env,slug}.go`.
- **Architecture & flexibility improvements:**
  - **One registry, three consumers:** dispatch, help, and MCP tool list all derive from a
    single `[]Command` table — kills two more drift classes.
  - **Config = pure function + boundary:** `LoadConfig(paths, env) → (Config, []Diagnostic)`
    is pure over its inputs; only `paths.go` touches the filesystem. Testable without disk.
  - **Keep the fail-loud posture everywhere** (loud on corrupt state, truncated YAML,
    malformed env) — it is a determinism and safety property, not an inconvenience.

## 4. Requirements (EARS-shaped) — seed for requirements.md
1. The system shall build with zero runtime Go module dependencies and parse its own CLI
   arguments without a third-party library.
2. The system shall fail its test suite if the dispatch registry and the help metadata
   disagree on the set of commands.
3. When writing any file, the system shall write atomically via temp-file + fsync + rename.
4. When executing any mutating command, the system shall hold the reentrant per-spec
   advisory lock for the entire read-modify-write.
5. When the `.specd/` root cannot be found by walking up, the system shall exit with the
   not-found code (3).
6. When loading configuration, the system shall layer global→project→env deterministically,
   validate and secret-scrub the orchestration block, and fail loud on a truncated scalar.
7. When a `SPECD_*` integer variable is malformed, the system shall clamp to range and warn
   once rather than silently defaulting.

## 5. Design notes — seed for design.md
- **Module boundaries:** `internal/cli` (parse), `internal/cmd` (dispatch + verbs),
  `internal/core` (io/lock/paths/config/exit/env/slug primitives).
- **Key types:** `Command{Name,Summary,Handler,Positional,Flags}` (single source);
  `Config` + `Diagnostic`; `SpecdError`/exit codes; lock handles.
- **Data/on-disk contracts:** `.specd/config.yml`; `.lock` files (0644, pid+unix-ms);
  YAML two-space-indent subset only.
- **Invariants to preserve:** zero-dep; help↔dispatch parity; atomic write; reentrant lock +
  stale reclaim; CAS (domain 02); fail-loud config; env clamping.
- **External interfaces:** exit codes; `SPECD_*` env vars; the `Command` table (drives
  domain 07 tool list).

## 6. Proposed task DAG — seed for tasks.md

### Wave 1 — primitives
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T10.1 | craftsman | `internal/core/io.go` | — | `go test ./internal/core -run TestAtomicWrite` | temp→fsync→rename; append fsyncs |
| T10.2 | craftsman | `internal/core/lock.go` | — | `go test ./internal/core -run TestReentrantLock` | reentrant; stale reclaim; timeout |
| T10.3 | craftsman | `internal/core/paths.go`, `internal/core/slug.go` | — | `go test ./internal/core -run 'TestFindRoot|TestSlug'` | walk-up NotFound(3); slug grammar |
### Wave 2 — parser, registry, config
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T10.4 | craftsman | `internal/cli/args.go`, `main.go` | — | `go test ./internal/cli -run TestArgs` | zero-dep parser; usage on error |
| T10.5 | craftsman | `internal/cmd/registry.go`, `internal/core/commands.go` | T10.4 | `go test ./internal/core -run TestRegistryMatchesHelp` | dispatch↔help parity |
| T10.6 | craftsman | `internal/core/config_loader.go`, `config_validate.go` | — | `go test ./internal/core -run TestConfigCascade` | global→project→env; fail-loud; scrub |
| T10.7 | validator | `internal/core/config_test.go` | T10.6 | `go test ./internal/core -run TestConfigNoLegacyJSON` | legacy config.json not parsed at runtime |

## 7. Risks, open questions, cross-domain dependencies
- **Risk:** hand-maintained help metadata drifts. Mitigation: single `Command` table drives
  dispatch + help + MCP; the parity test is CI-blocking.
- **Open question:** generate help text fully from the registry (shrinking `commands.go`) or
  keep rich hand-written help? Proposed: registry-generated usage + short summary; long-form
  help only where a verb needs examples.
- **Cross-domain deps:** every domain sits on these primitives; domain 02 (CAS uses
  io+lock), domain 07 (registry drives tools), domain 09 (config authority block + file
  backend).
