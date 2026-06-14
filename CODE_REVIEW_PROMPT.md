# Comprehensive Code Review Prompt for `specd` CLI

## Context

You are reviewing the **specd** codebase — a spec-driven coding harness CLI written in Go. It enforces a structured workflow (requirements → design → tasks → evidence-gated execution) for AI coding agents. The tool is agent-agnostic, deterministic, and has zero runtime dependencies on LLMs.

**Repository:** `https://github.com/0xkhdr/specd`  
**Language:** Go  
**Domain:** CLI tool, spec-driven development, project management, AI agent orchestration  

---

## Review Scope

Review **EVERY** Go source file in the project:
- `main.go` (entry point)
- `internal/cli/` (argument parsing)
- `internal/core/` (domain logic, state management, DAG, validation, I/O, templates, config, etc.)
- `internal/cmd/` (command implementations)
- `internal/testutil/` (test helpers)
- `scripts/` (install/uninstall shell scripts)
- `Makefile`
- `.github/workflows/` (CI/CD)

Also review any `.md` documentation files that describe architecture or contracts.

---

## Review Dimensions

For each file and across the entire codebase, evaluate and report findings in these categories:

### 1. Architecture & Design
- **Package structure**: Is the `internal/` layout clean? Are packages cohesive and loosely coupled? Does the `cmd/` vs `core/` split make sense?
- **Separation of concerns**: Are commands thin and delegating to core? Is business logic separated from CLI presentation?
- **Interface design**: Are there clear interfaces between layers? Is the `cli.Args` abstraction sufficient?
- **State management**: Is the `state.json` + `tasks.md` dual-write approach robust? Is the CAS (compare-and-swap) revision logic correct?
- **Concurrency model**: Is the advisory lock (`WithSpecLock`) + in-process mutex + reentrancy design sound? Are there race conditions?
- **Error handling strategy**: Is the `SpecdError` with typed codes (`ExitGate`, `ExitUsage`, `ExitNotFound`) used consistently?
- **Template system**: Is the template loading and variable substitution approach maintainable?
- **Config management**: Is `config.json` handling sensible? Are defaults well-defined?

### 2. Code Quality & Readability
- **Naming**: Are variables, functions, types, and packages named idiomatically in Go? (e.g., `DagTask` vs `DAGTask`, `NextResult` clarity)
- **Comments**: Are exported items documented? Are complex algorithms (DAG, cycle detection, wave grouping) explained?
- **Function length**: Are functions too long? (e.g., `RunCheck`, `RunTask`, `RunVerify` are very long — should they be refactored?)
- **Magic numbers/strings**: Are constants extracted? (e.g., timeout values, regex patterns, file permissions)
- **String handling**: Is string concatenation efficient? Are `strings.Builder` used where appropriate?
- **Error messages**: Are they actionable and user-friendly?
- **Code duplication**: Is there duplicated logic across commands? (e.g., gate checking, status derivation, JSON output formatting)
- **Dead code**: Are there unused functions, variables, or imports?

### 3. Performance
- **Memory allocation**: Are there unnecessary allocations in hot paths? (e.g., `DagTasksFromState` creates a new slice every call)
- **File I/O**: Is atomic write (`AtomicWrite`) efficient? Is `fsync` used appropriately? Is temp file cleanup robust?
- **Regex compilation**: Are regexes compiled once at init or per-call? (Check `ears.go`, `render.go`, `report.go`, `decision.go`, `memory.go`)
- **JSON marshaling**: Is `json.MarshalIndent` called unnecessarily? Could `json.Encoder` with indentation be used for streaming?
- **Map lookups**: Are map lookups optimized? Is `byID()` called repeatedly in loops?
- **Slice pre-allocation**: Are slices pre-allocated with `make(..., len(...))` where possible?
- **Lock contention**: Is the advisory lock held for the minimum time? Are there opportunities for finer-grained locking?

### 4. Security
- **Path traversal**: Is `ValidateSlug` sufficient? Are there any other user-input paths that could escape `.specd/`? (Check `install.sh`, `update.go`, `boot.go`)
- **Shell injection**: In `verify.go`, `exec.CommandContext(ctx, "sh", "-c", command)` runs arbitrary strings from `tasks.md`. Is this acceptable given the threat model? Could it be hardened?
- **File permissions**: Are created files/directories using correct permissions? (`0o755` for dirs, `0o644` for files)
- **Temp file security**: Are temp files created with restrictive permissions? Is the `defer` cleanup in `AtomicWrite` race-safe?
- **Lock file security**: Is the lock file content (PID + timestamp) safe? Could it leak sensitive info?
- **Update mechanism**: In `update.go`, downloading and replacing the running binary — is this safe? Is checksum verification missing?
- **Install script**: Does `install.sh` verify downloaded artifacts? Is `curl | bash` pattern safe enough for this use case?
- **Environment variables**: Are env vars like `SPECD_VERIFY_TIMEOUT_MS` validated? Are there injection risks?

### 5. Go Idioms & Best Practices
- **Error handling**: Are errors checked? Is `errors.As` used correctly? Are sentinel errors used where appropriate?
- **Context usage**: Is `context.Context` used properly in `verify.go`? Should it be used more broadly?
- **Interfaces**: Are interfaces defined by consumers (Go idiom) or by producers?
- **Pointer vs value receivers**: Are methods using the right receiver type?
- **Package naming**: Do package names follow Go conventions? (e.g., `cli` is fine, but `cmd` as a package name conflicts with `cmd/` directory convention)
- **Testing**: Are table-driven tests used? Is `t.Parallel()` used correctly? Is the test helper pattern (`internal/testutil`) clean?
- **Build tags**: Are there any build constraints needed? (e.g., Windows vs Unix path handling)
- **Module dependencies**: Is `go.mod` minimal? Are there unnecessary external dependencies?
- **Vendoring**: Should dependencies be vendored for a security-critical CLI?

### 6. Consistency & Maintainability
- **Exit codes**: Are `core.ExitOK`, `core.ExitGate`, `core.ExitUsage`, `core.ExitNotFound` used consistently across ALL commands? Are there any `return 1` or `return 0` literals that should use constants?
- **JSON output**: Is the JSON output format consistent across commands? Are nil slices handled consistently (some commands set `[]Type{}` explicitly, others don't)?
- **User output**: Is the mix of `fmt.Print`, `fmt.Printf`, `fmt.Fprintf(os.Stderr, ...)` consistent? Should there be a centralized output helper?
- **Help text**: Is the `USAGE` string in `core/render.go` (or wherever help is generated) kept in sync with actual command signatures?
- **State mutations**: Is `SaveState` called after every mutation? Are there paths where state is modified but not saved?
- **Logging**: Is there a consistent logging approach? (`core.Info`, `core.Warn`, `core.Error`, `core.Header`, `core.Success` — are these sufficient?)

### 7. Domain-Specific Logic (specd's core value)
- **DAG correctness**: Is cycle detection correct? Is wave violation detection correct? Is the critical path algorithm correct?
- **EARS validation**: Are the regex patterns comprehensive? Do they handle edge cases?
- **Task parser**: Is the line-based parser robust? Does it handle malformed input gracefully?
- **Phase transitions**: Is the planning ratchet (`PlanningAdvance`) correct? Are there missing transitions?
- **Evidence gating**: Is the verification → complete flow bulletproof? Are there ways to bypass the evidence gate?
- **Boot detection**: Is `AnalyzeBoot` deterministic and safe? Does it handle edge cases (empty repos, monorepos, etc.)?
- **Enrichment**: Is the enrichment contract clear? Is freshness checking robust?

### 8. Testing & CI/CD
- **Test coverage**: Is the 100% coverage target for critical functions actually enforced? Are there gaps?
- **Race detection**: Is `-race` used in CI? Is the concurrency stress test (`scripts/stress.sh`) comprehensive?
- **Golden files**: Are report golden files maintained? Are they deterministic?
- **CI workflow**: Is the GitHub Actions workflow robust? Does it test on multiple OS/arch combinations?
- **Release process**: Is the release binary signing/checksum process documented? Is it automated?

---

## Specific Files to Deep-Dive

Pay special attention to these high-risk / high-complexity files:

1. **`internal/cmd/check.go`** — Very long function, mixes many gates, complex control flow. Should be refactored into smaller gate functions.
2. **`internal/cmd/task.go`** — The evidence gate. Any bug here compromises the entire integrity model. Check the `deriveStatus` logic, the dual-write to `tasks.md` + `state.json`, and the blocker handling.
3. **`internal/cmd/verify.go`** — Runs arbitrary shell commands. Security critical. Check timeout handling, output capture, and the criterion recording logic.
4. **`internal/core/lock.go`** — Concurrency primitive. Check the reentrancy logic, stale lock reclamation, and the `goID()` parsing robustness.
5. **`internal/core/state.go`** — State machine. Check migration logic, CAS correctness, and the `Clock` abstraction.
6. **`internal/core/dag.go`** — Graph algorithms. Verify cycle detection, wave grouping, and critical path correctness. Check for off-by-one errors.
7. **`internal/cmd/update.go`** — Self-updating binary. Check the download+replace logic, error handling, and security (no checksum verification!).
8. **`scripts/install.sh`** — Shell script run by `curl | bash`. Check for POSIX compliance, error handling, and PATH manipulation safety.
9. **`main.go`** — Entry point. Check the `--json` flag stripping logic and the dispatch table. Is it scalable for adding new commands?
10. **`internal/cmd/approve.go`** — Complex state machine transitions. Check the gate clearing, verification acceptance, and planning ratchet logic.

---

## Comparison Benchmarks

Compare `specd` against these well-known Go CLI tools in the same domain:
- **Cobra** (spf13/cobra) — How does specd's custom CLI parsing compare to Cobra's flag handling, subcommands, and help generation? Would adopting Cobra improve maintainability?
- **Viper** (spf13/viper) — How does specd's custom config handling compare to Viper? Is the JSON-only config a limitation?
- **urfave/cli** — Similar comparison for CLI framework adoption.
- **Task** (go-task/task) — Another task runner with DAG. How does specd's DAG compare? What can be learned?
- **Mage** (magefile/mage) — Go-based make alternative. How does specd's verification approach compare to Mage's target caching?
- **GitHub CLI** (cli/cli) — A large Go CLI. How does specd's architecture compare? What patterns should be adopted or avoided?

---

## Deliverables

For each finding, provide:
1. **Severity**: `critical` | `high` | `medium` | `low` | `info`
2. **Category**: Architecture | Quality | Performance | Security | Idioms | Consistency | Domain
3. **Location**: File and line number (or function name)
4. **Description**: What is the issue?
5. **Impact**: Why does it matter?
6. **Recommendation**: How to fix it, with code examples where helpful.
7. **References**: Link to relevant Go best practices, security guidelines, or comparable tools.

Organize findings by file, then by severity. Provide a summary at the top with:
- Total number of findings by severity
- Top 5 most critical issues
- Overall architectural assessment (score 1-10)
- Overall security assessment (score 1-10)
- Overall performance assessment (score 1-10)
- Overall maintainability assessment (score 1-10)
- Recommended priority order for fixes

---

## Constraints

- Be **specific** and **actionable**. Avoid vague advice like "improve error handling" — point to exact lines and suggest exact refactors.
- Be **constructive**. The codebase is well-intentioned and has strong design principles. Acknowledge what's done well before criticizing.
- Consider **backward compatibility**. Changes to `state.json` schema or CLI interface may break existing users.
- Consider **testability**. Every refactor should make testing easier, not harder.
- Consider **the project's philosophy**: "The agent reasons. The harness enforces." Any change should strengthen, not weaken, the harness's enforcement capability.

---

## Example Finding Format

```markdown
### [CRITICAL] Shell Injection in `verify.go`
- **File**: `internal/cmd/verify.go`
- **Line**: ~145
- **Description**: `exec.CommandContext(ctx, "sh", "-c", command)` executes the `verify:` command from `tasks.md` verbatim. A malicious or compromised `tasks.md` could inject shell commands.
- **Impact**: Arbitrary code execution with the privileges of the user running `specd`.
- **Recommendation**: 
  - Option A: Parse the `verify:` line into command + args and use `exec.CommandContext(ctx, cmd, args...)` instead of `sh -c`.
  - Option B: If `sh -c` is required for complex pipelines, add a `--allow-shell` flag and warn users. Document the risk in `AGENTS.md`.
  - Option C: Sanitize the command string before execution (whitelist approach).
- **References**: [CWE-78](https://cwe.mitre.org/data/definitions/78.html), Go `os/exec` best practices
```

---

## Final Instructions

1. Read all source files carefully.
2. Run `go vet ./...`, `gofmt -l .`, and `go test -race ./...` if possible.
3. Check for any `TODO`, `FIXME`, or `XXX` comments.
4. Verify that all exported functions have documentation comments.
5. Check that the `go.mod` file has no unnecessary dependencies.
6. Ensure the `README.md` and `AGENTS.md` accurately describe the current implementation.
7. Produce the review in a single markdown document.

**Begin the review now.**
