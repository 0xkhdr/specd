# Stage 04 — Tasks

Branch: `refactor/04-cli-output-consistency`.

## T1 — PrintJSON helper + nil-slice rule (F2)
**Files:** `internal/core/ui.go` (or `output.go`); all `internal/cmd/*` that marshal.

1. Add:
   ```go
   func PrintJSON(v any) error {
       b, err := json.MarshalIndent(v, "", "  ")
       if err != nil { return err }
       fmt.Println(string(b))
       return nil
   }
   ```
2. Replace each `b, _ := json.MarshalIndent(...); fmt.Println(string(b))` block
   (check.go, approve.go, dispatch.go, program.go, next.go, status.go, boot.go,
   enrich.go, waves.go, context.go — grep `MarshalIndent` in `internal/cmd`).
3. Enforce non-nil lists: before emitting, ensure every slice field is non-nil
   (`if x == nil { x = []T{} }`) — or switch the response to a typed struct and
   initialize slices to `[]T{}`. Keep `check.go`'s existing normalization, just
   route through `PrintJSON`.

**Verify:** `go test ./... ` then re-generate/inspect any JSON golden files.

## T2 — Exit-code audit (F1)
**Files:** all `internal/cmd/*.go`.

1. `grep -rn "return 0\|return 1" internal/cmd`. For each:
   - success → `core.ExitOK`
   - usage error → `core.ExitUsage` (most already via `usageExit`)
   - gate/violation/verify-fail → `core.ExitGate`
   - not-found → handled by `specdExit` already
2. For `(int, error)` closures inside `WithSpecLock`, return the constant in the
   int slot (e.g. `return core.ExitGate, nil`).
3. Add `docs/command-reference.md` "Exit codes" section documenting each.

**Verify:** `go build ./... && go test ./...`; add `main_test.go` assertions on
exit codes for a gate failure, usage error, and success path.

## T3 — Output convention (F3)
**Files:** `internal/cmd/helpers.go`, commands emitting `fail`/`✗`.

1. Add `func errLine(format string, a ...any)` in helpers (wraps
   `fmt.Fprintf(os.Stderr, ...)`); remove or repoint `printlnErr`.
2. Route all `fmt.Fprintf(os.Stderr, "fail …")` and `"✗ …"` lines through it /
   `core.Error`. Results stay on stdout.
3. Document the rule at the top of `docs/command-reference.md`.

**Verify:** `go test ./...` (stderr-captured tests must still match).

## T4 — args.go `--key=value` + boolean registration guard (F4)
**Files:** `internal/cli/args.go`, `internal/cli/args_test.go`.

1. In `ParseArgs`, after `key := tok[2:]`, split on first `=`:
   ```go
   if eq := strings.IndexByte(key, '='); eq >= 0 {
       args.Flags[key[:eq]] = key[eq+1:]
       continue
   }
   ```
2. Add test cases: `--status=complete` → `status=complete`; `--json` bool;
   `--force --json` two bools; `--evidence "x y"` value form.
3. Add a registration guard test: a slice of every boolean flag name used across
   `internal/cmd` (maintain manually or grep), assert each is in
   `booleanFlags`. Prevents the silent next-token-consume bug.

**Verify:** `go test ./internal/cli/`

## T5 — Command registry for dispatch + help (F5, F6)
**Files:** `main.go`, `internal/cmd/dispatch.go` or new `internal/cmd/registry.go`, `internal/core/help.go`.

1. Define:
   ```go
   type Command struct {
       Name    string
       Summary string
       Usage   string
       Run     func(cli.Args) int
   }
   ```
2. Build `var Registry = []Command{ {"init", "...", "...", cmd.RunInit}, ... }`
   covering all 16 commands from `main.go:97-142`. Keep ordering for help.
3. `main.go dispatch` looks up by name (map built from Registry); unknown →
   existing error path.
4. `RenderHelp` iterates `Registry` for the command list so help cannot drift.
   Cross-check current help text to preserve wording.
5. Record the "no Cobra" decision: `specd decision` ADR or a note in
   `docs/contributor-guide.md`.

**Verify:** `go test ./... && go run . --help` output matches prior help
(diff intentionally only where registry wording was normalized).

## Done-when
- `grep -rn "return 0\|return 1" internal/cmd` returns only constant-backed or
  intentional cases (ideally zero bare literals).
- All JSON lists emit `[]`; one `PrintJSON` path.
- `--key=value` works; boolean guard test passes.
- Dispatch + help share `Registry`.
- `go vet ./... && gofmt -l . && go test -race ./...` green.
