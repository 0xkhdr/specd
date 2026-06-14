# Stage 04 — CLI, Output & Exit-Code Consistency

## Scope

Cross-cutting consistency: argument parsing (`internal/cli/args.go`), the
`main.go` dispatch table, exit-code constants, JSON output shape (nil-slice
handling), and a centralized output helper. This standardizes what every
command — including the Stage 03 refactors — emits.

## Current state & findings

### F1 — [MEDIUM] Bare `return 0/1` literals instead of exit constants
Grep finds ~40 literals across `internal/cmd/*` (e.g. `check.go:228,246`,
`next.go:55`, `program.go:136,193`, `dispatch.go:54`, `status.go`,
`approve.go:71`, `task.go:226`, `verify.go:153`). The review (prompt §6) calls
for `core.ExitOK`/`ExitGate`/`ExitUsage`/`ExitNotFound` everywhere. Magic `1`
is ambiguous: sometimes it means "gate failed" (should be `ExitGate`),
sometimes "verification failed" (a distinct semantic). Inconsistent codes break
scripted callers that branch on exit code.

**Intent:** audit every literal. Map:
- gate/violation failure → `core.ExitGate`
- usage error → `core.ExitUsage`
- not-found → `core.ExitNotFound`
- success → `core.ExitOK`
Where `1` currently means "check found violations" or "verify failed", decide a
single documented convention (recommend: keep `ExitGate` for both, since both
are enforcement failures) and apply uniformly. Document the exit-code contract
in `docs/command-reference.md`.

### F2 — [MEDIUM] JSON nil-slice handling is inconsistent
`check.go:215-223` explicitly rewrites `nil` violations/warnings to `[]T{}`
before marshaling so JSON shows `[]` not `null`. Other commands
(`approve.go`, `dispatch.go`, `program.go`, `next.go`) build ad-hoc
`map[string]interface{}` and do **not** all normalize nil slices — some emit
`null`. A JSON consumer (the agent) then must handle both `null` and `[]`.

**Intent:** centralize JSON emission in one helper:
```go
func PrintJSON(v any) error // MarshalIndent, "\n", to stdout
```
and adopt a rule: every list field in a JSON response is a non-nil slice.
Prefer typed result structs with `json:"...,omitempty"` *only* where truly
optional; for arrays the agent parses, always emit `[]`. Replace ad-hoc
`map[string]interface{}` literals with named structs per command response where
practical (improves the contract and removes the nil-normalization dance).

### F3 — [MEDIUM] Output is scattered across fmt.Print/Printf/Fprintf
Commands mix `fmt.Println`, `fmt.Printf`, `fmt.Fprintf(os.Stderr, …)`, and the
`core.Info/Warn/Error/Success/Header` helpers (`ui.go`). No single rule for
"results to stdout, diagnostics to stderr". `helpers.go` has `printlnErr` but it
is barely used.

**Intent:** define and document the convention:
- Machine/result output → stdout (`fmt.Print*` or `PrintJSON`).
- Human diagnostics, gate failures, warnings → stderr via `core.Error/Warn`.
Route all `fail …` / `✗ …` lines through `core.Error`/a stderr helper. Keep
`core.*` UI helpers as the single styled-output surface; remove `printlnErr`
duplication or make it the canonical stderr path.

### F4 — [LOW] `args.go` parser limitations
`cli/args.go`:
- `--flag=value` form is **not supported** — only `--flag value`
  (`args.go:22`). Agents writing `--status=complete` get `status="true"`
  silently. Either support `=` or document its absence loudly.
- Boolean flags are a hardcoded allowlist (`args.go:10-12`); a new bool flag
  silently consumes the next token as its value if forgotten. Fragile.
- A value that legitimately starts with `--` (rare) cannot be passed.

**Intent:** add `--key=value` parsing (split on first `=`). Keep the boolean
allowlist but add a test that every boolean flag used in `internal/cmd` is
registered (guard against the silent-consume footgun). Document the grammar in
`docs/command-reference.md`. Do **not** adopt Cobra — see decision below.

### F5 — [INFO] Cobra/urfave comparison (prompt §Benchmarks)
Decision: **stay custom.** specd's parser is ~30 lines, zero-dependency
(go.mod minimal — a stated value), deterministic, and the command set is small
and stable. Cobra adds a large dependency tree and its help/flag magic fights
the "harness enforces" determinism goal. Record this as an ADR
(`specd decision` / `docs/`) so it is a conscious choice, not drift. Adopt one
Cobra-ish idea only: a small dispatch *table* (map) instead of the `switch` in
`main.go:97-142` to make adding commands declarative.

### F6 — [LOW] `main.go` dispatch is a manual switch
`main.go:97-142` is a 16-case switch; help text (`core/render.go` /
`help.go`) must be kept in sync by hand (prompt §6 help-sync concern).

**Intent:** replace with a `map[string]Command` registry where each entry
carries the handler + summary + usage, and have `RenderHelp` iterate the same
registry. Single source of truth → help can never drift from dispatch.

## Non-goals
- Adopting Cobra/Viper/urfave (explicitly rejected, F5).
- Changing flag *names* or command names (user-facing contract).

## Acceptance criteria
1. Zero bare `return 0/1` in `internal/cmd` (enforced by a grep test or review);
   all use `core.Exit*`. Exit-code contract documented.
2. All JSON responses emit `[]` (never `null`) for list fields, via `PrintJSON`.
3. stdout/stderr convention documented and applied; `fail`/`✗` go to stderr.
4. `--key=value` supported and tested; boolean-flag registration test added.
5. `main.go` dispatch + help share one command registry.
6. `go test -race ./...` green; existing golden output unchanged except
   intentional nil→[] normalizations (update goldens deliberately).
