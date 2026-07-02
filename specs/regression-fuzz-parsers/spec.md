# S15 — Fuzz & Parser Regression

## 1. Purpose and requirement coverage

Guarantee parsers survive malformed input without panics or data corruption.
Contributes to **R1–R5** robustness (crash-free command/state/protocol paths).

## 2. Verified current state

- Parsers with no fuzz coverage today:
  - Tasks parser: `internal/core/tasksparser.go` (`tasksparser_test.go`).
  - State loader: `internal/core/state.go` `LoadState` (`state_test.go`,
    `state_cas_test.go`).
  - EARS requirements linter: `internal/core/ears.go` `LintEars`
    (`ears_test.go`).
  - Spec files parser: `internal/core/specfiles.go` (`specfiles_test.go`).
  - Config loader: `internal/core/config_loader.go`
    (`config_corruption_test.go` exists — good precedent).
- Existing fuzz: only `internal/mcp/host_caps_fuzz_test.go` (host capability
  negotiation). No fuzz on the core parsers above.

## 3. Proposed design and end-to-end flow

Add Go native fuzz targets (`func FuzzXxx(f *testing.F)`) for `ParseTasks`,
`LoadState`, `LintEars`, and the spec-files parser. Seed each corpus with valid
+ known-tricky inputs. The invariant under fuzz: the parser returns an error or a
valid structure — it never panics, never hangs, never emits a half-written
state. Time-box each fuzz run; file issues for non-critical findings.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** parser function signatures (`ParseTasks`, `LoadState`,
  `LintEars`); the "error, don't panic" contract.
- **Dependencies:** none (Wave-1 root; parsers are leaf functions).

## 5. Invariants, security, errors, observability, compatibility, rollback

- **Invariant:** malformed input → error, never panic/crash/corruption.
- **Security:** a crashing parser is a DoS surface on untrusted spec files.
- **Rollback:** additive fuzz tests; revert by deletion.

## 6. Acceptance criteria and validation commands

- `go test ./internal/core/... -run Fuzz -count=1` passes (seed corpus).
- `go test ./internal/core/ -fuzz FuzzParseTasks -fuzztime 60s` finds no crash.
- Same for `FuzzLoadState`, `FuzzLintEars` (time-boxed ~60s each).

## 7. Open decisions and deviations

- Deviation F8/D6: analysis plan says "no fuzz testing"; repo already has
  `internal/mcp/host_caps_fuzz_test.go`. This spec adds fuzz for the *core
  parsers*, which genuinely lack it.
- Deviation U5: fuzz is time-boxed; deep findings become tracked issues, not
  blockers for this spec.
