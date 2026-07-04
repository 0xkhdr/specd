# CLI Architecture & Foundations

This document details the core architecture, configuration system, file lock mechanics, and exit codes of the `specd` (v2) binary.

---

## 1. Zero-Dependency CLI Parser

`specd` is compiled as a single statically linked Go binary. To ensure zero runtime dependencies, it uses a hand-rolled, lightweight argument parser instead of third-party libraries (e.g. Cobra or Viper):

*   **Arg Parsing:** `internal/cli/args.go` parses basic flags and routing.
*   **Command Dispatch:** `internal/cmd/registry.go` matches verbs to commands.

### Registry↔Help Parity Guard
To prevent documentation from drifting out of date, `TestRegistryMatchesHelp` runs during verification. The test asserts that every registered command has matching help metadata and usage examples in [commands.go](file:///var/www/html/rai/up/specd/reference/internal/core/commands.go). A drift causes compilation tests to fail, blocking pull requests.

---

## 2. File Locking & CAS Primitives

Multi-process coordination and state mutations are guarded by a custom reentrant lock engine in [lock.go](file:///var/www/html/rai/up/specd/reference/internal/core/lock.go):

*   **Cross-Process File Lock:** Uses an `O_CREATE|O_EXCL` file named `.lock` containing the active PID and creation timestamp.
*   **Stale Reclaim:** If a lock is held longer than `SPECD_LOCK_STALE_MS` (default 30s), another process can reclaim it.
*   **Acquire Timeout:** Commands wait up to `SPECD_LOCK_TIMEOUT_MS` (default 5s) to acquire a lock before failing.
*   **Goroutine Reentrancy:** Supports lock reentrancy by parsing active goroutine IDs.
*   **CAS Integrity:** Any state mutation under `SaveState` checks the in-memory `revision` against disk, bumping it on match, and failing if they differ.

---

## 3. Configuration System (YAML Subset)

Configuration is managed via `.specd/config.yml`.

```yaml
# config.yml
gates:
  ears: error
verify:
  sandbox: off
orchestration:
  enabled: false
```

### Loading Rules
1.  **Deterministic Cascading:** Config is layered in order: `DefaultConfig` -> Global (`~/.specd/config.yml`) -> Project (`.specd/config.yml`) -> Environment variables.
2.  **Hand-rolled YAML Parser:** Uses `parseSimpleYAML`, a simple, deterministic YAML subset parser.
3.  **Fail-Loud Posture:** To prevent hidden misconfigurations, the parser fails loud immediately on malformed syntax or truncated scalars.
4.  **No Legacy Formats:** All legacy JSON parser paths and config migrations ([config_migrate.go](file:///var/www/html/rai/up/specd/reference/internal/core/config_migrate.go)) are **CUT** from v2.

*Origin:* Simplified config engine from [config_loader.go](file:///var/www/html/rai/up/specd/reference/internal/core/config_loader.go).

---

## 4. Exit Codes & Environment Control

### Exit Codes
`specd` uses standardized exit codes:
*   `0` (`ExitOK`): Action succeeded.
*   `1` (`ExitGate`): Validation gate or verification command failed.
*   `2` (`ExitUsage`): Invalid arguments or flag combinations.
*   `3` (`ExitNotFound`): Path `.specd/` root not found when walking up directories.

### Environment Variable Clamping
Boolean or integer variables (prefixed with `SPECD_`) are parsed using `EnvInt`. If an environment variable is out of range, the harness clamps the value to acceptable limits and prints a **single, non-spammed warning** rather than silently defaulting.
