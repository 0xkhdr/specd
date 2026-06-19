# Tasks: MCP Prompts

## T1 — Advertise prompts capability
- **Deps:** Wave 1
- **Files:** `internal/mcp/server.go` (`initializeResult`)
- **Do:** Add `"prompts": {"listChanged": false}` to capabilities.
- **Verify:** initialize test (AC1).
- **Satisfies:** R1.

## T2 — Prompt registry + embedded content
- **Deps:** none
- **Files:** `internal/mcp/prompts.go` (new), embedded `.md` templates
- **Do:** Define `promptDef{name, description, args, render}`; embed canonical phase/role text via `//go:embed`. Register 4 phase + 2 role prompts.
- **Verify:** Registry completeness unit test.
- **Satisfies:** R2, R6, R7.

## T3 — `prompts/list`
- **Deps:** T2
- **Files:** `internal/mcp/prompts.go`
- **Do:** Emit `{prompts:[{name, description, arguments}]}`, deterministic order.
- **Verify:** AC2.
- **Satisfies:** R2, R7.

## T4 — `prompts/get` + substitution
- **Deps:** T2
- **Files:** `internal/mcp/prompts.go`
- **Do:** Resolve name→`render(root,args)`; inject `slug` context header where declared; unknown ⇒ rpcError.
- **Verify:** AC3, AC4, AC5, AC6 (golden).
- **Satisfies:** R3, R4, R5, R6.

## T5 — Wire into route()
- **Deps:** T3, T4
- **Files:** `internal/mcp/server.go`
- **Do:** Add `prompts/list` + `prompts/get` cases.
- **Verify:** Integration round-trip.
- **Satisfies:** R1–R7 live.

## T6 — Docs
- **Deps:** T5
- **Files:** `docs/mcp-guide.md`
- **Do:** Document prompt names/arguments; note steering files now reachable as prompts.
- **Verify:** Manual read.
- **Satisfies:** plan §B2.

**Wave gate:** parallel with mcp-resources/mcp-composite-tools; feeds Wave 3.
