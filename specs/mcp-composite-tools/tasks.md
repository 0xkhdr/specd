# Tasks: MCP Composite Tools

## T1 — Add `enum` to schemaProp
- **Deps:** Wave 1 done
- **Files:** `internal/mcp/tools.go`
- **Do:** Add `Enum []string \`json:"enum,omitempty"\`` to `schemaProp` so `view`/`action` advertise allowed values.
- **Verify:** `go build ./internal/mcp/...`; JSON omits empty enum.
- **Satisfies:** schema for R1/R4/R5.

## T2 — `specd_inspect` composite
- **Deps:** T1
- **Files:** `internal/mcp/intent.go` (or new `composite.go`)
- **Do:** Intent tool with required `view` enum `{status,waves,context,check,validate,replay,diff}` + `slug` + view-specific flags (`from`/`to`/`version`/`schema`). Translate to atomic command; validate enum (R6).
- **Verify:** Round-trip test AC1/AC2; enum error AC5.
- **Satisfies:** R1, R6, R8, R9.

## T3 — `specd_read` + `specd_query`
- **Deps:** T1
- **Files:** `internal/mcp/composite.go`
- **Do:** `specd_read{view:report, format}` → `report`. `specd_query{view:next|dispatch, all, json}` → `next`/`dispatch`. Exclude serve/watch (document why).
- **Verify:** Round-trip tests; serve/watch absent.
- **Satisfies:** R2, R3, R8.

## T4 — `specd_orchestrate` composite
- **Deps:** T1
- **Files:** `internal/mcp/composite.go`
- **Do:** `action` enum `{start,step,status,why,pause,resume,cancel}` → `brain <action>` with policy/session defaults reusing existing intent translators.
- **Verify:** Round-trip AC3; readOnlyHint correct per action (R9).
- **Satisfies:** R4, R6, R8, R9.

## T5 — `specd_worker` composite
- **Deps:** T1
- **Files:** `internal/mcp/composite.go`
- **Do:** `action` enum `{claim,heartbeat,progress,query,report,block,release,inbox}` → `pinky <action>`.
- **Verify:** Round-trip AC4.
- **Satisfies:** R5, R6, R8.

## T6 — Filtering integration
- **Deps:** T2–T5
- **Files:** `internal/mcp/tools.go` (`resolveMCPExposure`/`buildTools`)
- **Do:** Composites participate in exposure plan: gate orchestrate/worker on orchestration include (R7); under `essential` prefer composites; under `all` emit composites + atomics.
- **Verify:** AC6, AC7 tests.
- **Satisfies:** R7.

## T7 — Tests + docs
- **Deps:** T6
- **Files:** `internal/mcp/composite_test.go`, `docs/mcp-guide.md`
- **Do:** Full round-trip matrix (plan §7.3); document composites + deprecation note for `brain_*`.
- **Verify:** `go test ./internal/mcp/...`.
- **Satisfies:** AC1–AC7.

**Wave gate:** independent of mcp-resources/mcp-prompts; all three feed Wave 3.
