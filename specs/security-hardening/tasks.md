# Tasks — Security Hardening (S1)

## Wave 1

- [ ] T1 — Audit MCP argv construction and existing per-command validation coverage
  - why: confirm exact shape of `buildArgv()` and which commands already validate their own inputs, before adding a second validation layer that could duplicate or conflict (Requirement 1)
  - role: investigator
  - files: internal/mcp/server.go, internal/mcp/transport.go, internal/mcp/transport_http.go
  - contract: read `buildArgv()` (~line 387-417) and `enforceBoundedToolCall` (~line 377-384) in full; list every MCP tool name and its expected argument keys/types by cross-referencing `internal/core/commands.go` `CommandMeta`; report which tools have zero downstream validation today. Do NOT write or modify code.
  - acceptance: a written list (in the task's evidence/PR description) of all MCP tools, their argument shapes, and which ones lack any validation today
  - verify: N/A
  - depends: —
  - requirements: 1

- [ ] T2 — Confirm current slug-validation and sandbox fail-closed test coverage
  - why: Requirement 2 adds regression tests only where coverage is missing; must not duplicate existing tests (discrepancy D3, D4 already confirmed these behaviors are correct in production code, but test coverage was not separately audited)
  - role: investigator
  - files: internal/core/slug_test.go, internal/runner/runner_sandbox_test.go, internal/runner/runner_test.go
  - contract: grep for existing tests covering `ValidateSlug` edge cases (`..`, `/`, leading non-`[a-z0-9]`) and `SelectRunner` fail-closed behavior for missing bwrap/container deps; report exact gaps, if any. Do NOT write or modify code.
  - acceptance: written list of which Requirement 2 acceptance criteria already have a passing test today vs. which are genuinely missing
  - verify: N/A
  - depends: —
  - requirements: 2

## Wave 2

- [ ] T3 — Add MCP argument-shape validation gate
  - why: close the one confirmed security gap — MCP tool-call arguments are not validated against a declared shape before argv construction (Requirement 1)
  - role: builder
  - files: internal/mcp/argschema.go (new), internal/mcp/server.go
  - contract: create `validateToolArgs(tool string, args map[string]any) error` in a new file `internal/mcp/argschema.go`, with one schema entry per MCP tool listed in T1's findings (unknown-key and type-mismatch both reject). Call it at the top of `buildArgv()` before the existing `[]any` check. Reuse the existing JSON-RPC error envelope/code already used by `buildArgv()` for its current type check — do not invent a new error code. Do NOT change behavior for any already-valid request shape, and do NOT touch downstream per-command validation (e.g. `ValidateSlug`) — this is an additive, earlier gate only.
  - acceptance: a request with an undeclared argument key, or a type-mismatched value, is rejected before any command dispatch; all previously-valid requests still succeed
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/mcp/... -race -count=1
  - depends: T1
  - requirements: 1

- [ ] T4 — Add regression tests for slug validation and fail-closed sandboxing (only where T2 found gaps)
  - why: pin the already-correct security properties confirmed by this review so a future refactor can't silently reintroduce a path-traversal or sandbox-bypass regression (Requirement 2.1, 2.2)
  - role: builder
  - files: internal/core/slug_test.go, internal/runner/runner_sandbox_test.go
  - contract: using T2's gap list, add only the missing test cases. For slug validation: table-driven cases covering `..`, `/`, leading digit/uppercase/symbol, empty string. For sandbox: assert `SelectRunner("bwrap", ...)` and `SelectRunner("container", ...)` return errors (not a usable runner) when their required binary/env is absent, by temporarily manipulating `PATH` in the test (not modifying production code). Do NOT modify `internal/core/slug.go` or `internal/runner/runner_sandbox.go` — production behavior is already correct, only tests are added.
  - acceptance: `go test ./internal/core/... ./internal/runner/... -race -count=1` passes, including the new cases; deleting the fail-closed check in `runner_sandbox.go` locally (manual sanity check, not committed) makes the new test fail
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/core/... ./internal/runner/... -race -count=1
  - depends: T2
  - requirements: 2

- [ ] T5 — Add install.sh checksum-enforcement regression test
  - why: pin the existing `verify_checksum()` behavior (discrepancy D2) so a future edit to `scripts/install.sh` can't silently drop checksum enforcement (Requirement 2.3)
  - role: builder
  - files: scripts/install.sh, scripts/install_test.sh (new, or existing shell-test location per TESTING.md convention)
  - contract: add a shell-level test that (a) stubs a release with a deliberately wrong `SHA256SUMS` entry and asserts `install.sh` exits non-zero without installing, and (b) asserts `install.sh --no-verify` skips the check and proceeds. Check `TESTING.md` and existing `scripts/*_test.sh` (if any) for the established shell-test convention before choosing file location/framework — do NOT invent a new test framework if one is already in use. Do NOT modify `scripts/install.sh`'s logic, only add the test.
  - acceptance: the new test fails if `verify_checksum()`'s `die` call is commented out (manual sanity check), and passes against current `install.sh`
  - verify: cd /var/www/html/rai/up/specd && bash scripts/install_test.sh
  - depends: T2
  - requirements: 2

- [ ] T6 — Add sandbox-unavailability diagnostic to `specd doctor`
  - why: surface a friendly, specific diagnostic before a user hits a raw `SelectRunner` error, without changing fail-closed behavior (Requirement 3)
  - role: builder
  - files: internal/cmd/doctor.go
  - contract: add a check that, when `verify.sandbox` resolves to `bwrap` or `container` and the required binary is missing from `PATH` (or `SPECD_SANDBOX_IMAGE` is unset for `container`), reports a `doctor` finding naming the missing dependency and an OS-appropriate install hint. Must not change `SelectRunner`'s error/return behavior — `doctor` only reads configuration and probes `PATH`, it does not alter runner selection. `doctor` must still exit 0/complete normally when this finding is present (findings are advisory).
  - acceptance: running `specd doctor` with `verify.sandbox: bwrap` configured and `bwrap` absent from `PATH` shows the new finding; `specd verify` behavior is unchanged (still fails closed with the existing error)
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/cmd/... -race -count=1 -run TestDoctor
  - depends: T1
  - requirements: 3

## Wave 3

- [ ] T7 — Update SECURITY.md to cross-reference new validation gate
  - why: keep SECURITY.md's documented threat model accurate after T3/T6 land (action-prompt rule: every config/behavior must be documented)
  - role: builder
  - files: SECURITY.md
  - contract: add one bullet under the existing MCP section describing the new argument-shape validation gate (T3) and one under the sandbox section describing the new `doctor` diagnostic (T6). Do NOT rewrite existing SECURITY.md sections that T1-T2 confirmed are already accurate (checksum verification, fail-closed sandboxing, slug validation).
  - acceptance: SECURITY.md accurately describes the post-T3/T6 state; `scripts/docs-lint.sh` still passes
  - verify: cd /var/www/html/rai/up/specd && bash scripts/docs-lint.sh
  - depends: T3, T6
  - requirements: 1, 3

- [ ] T8 — Review wave: confirm no regression in existing security behavior
  - why: final gate before this spec is marked done — verify T3-T6 didn't weaken any already-correct property (action-prompt rule: validate each wave)
  - role: reviewer
  - files: internal/mcp/argschema.go, internal/mcp/server.go, internal/core/slug_test.go, internal/runner/runner_sandbox_test.go, scripts/install.sh, internal/cmd/doctor.go, SECURITY.md
  - contract: re-read every file changed in T3-T7 against this spec's Requirements 1-3 and the discrepancy log's D1-D4 findings; confirm `sh -c` execution in `internal/runner/runner.go` is untouched, confirm no fallback/bypass was added to sandbox selection, confirm slug validation call sites are unchanged. Report any deviation.
  - acceptance: written confirmation that all three already-correct properties (fail-closed sandbox, slug validation, checksum-required install) are bit-for-bit unchanged in production code, and only the new gate (T3) plus tests/diagnostics (T4-T6) were added
  - verify: N/A
  - depends: T7
  - requirements: 1, 2, 3

- [ ] T9 — Full verification run
  - why: gate G1 (action-prompt/plan) requires security review complete, validated, before performance work (S2) begins
  - role: verifier
  - files: N/A
  - contract: run the full project test suite and confirm zero regressions from this spec's changes
  - acceptance: `make test` and `make lint` both pass with zero new failures attributable to S1 changes
  - verify: cd /var/www/html/rai/up/specd && make test && make lint
  - depends: T8
  - requirements: 1, 2, 3
