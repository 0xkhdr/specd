# Contributing to specd

> **Read `PROJECT.md` and `docs/architecture.md` before writing a line of code.**
> The guardrails in this file enforce the same invariants the harness enforces — subtractive
> bias, evidence integrity, determinism first.

---

## 1. Prerequisites

- **Go 1.22+** (see `go.mod`; currently targets `go 1.22` or later).
- **git** — required at runtime for `gitHead()` and the advisory lock.
- No other runtime dependencies. `go.mod` has **no `require` stanza**.

Verify your environment:

```bash
go version     # go1.22.x or later
git --version  # any modern version
```

---

## 2. Build

```bash
# Build the binary
go build -o specd .

# Zero-dep check (must stay empty)
go mod tidy && grep -c "^require" go.mod || echo "zero deps — ok"

# Vet (zero warnings expected)
go vet ./...

# Run all tests
go test ./...

# Run tests with race detector (required before any PR)
go test -race ./...

# Run fuzz test (parser round-trip; smoke only, not CI-blocking)
go test -fuzz=FuzzParseSerialize ./internal/core/ -fuzztime=10s
```

---

## 3. Project structure (quick map)

```
main.go                    entry point
internal/
  cli/                     arg parser (~40 lines, no Cobra)
  core/                    the harness spine — all business logic lives here
    gates/                 pluggable gate registry (ADR-4)
    verify/                verify executor + sandbox opt-in
  cmd/                     thin command handlers — no business logic
  context/                 context manifest engine
  mcp/                     stdio JSON-RPC 2.0 MCP server
  integration/             host adapter interface
  orchestration/           Brain/Pinky — opt-in, inert by default
docs/                      derived reference files (you are here)
specs/                     authored per-domain specs (spec.md + tasks.md)
reference/                 FROZEN v1 — read-only museum
scripts/                   regression scripts (regress-all.sh, regress-lint.sh …)
```

> **`reference/` is a museum.** Never import from it, build it, or copy files wholesale.
> Read it to understand what v1 did and why.

---

## 4. Non-negotiable guardrails

Before submitting any change, verify all of these hold:

### Determinism first
No LLM call may sit inside the harness's decision path. Gates, DAG computation, reports,
and Brain decisions are **pure functions of on-disk state**. No network in any
render/decide path.

```go
// ✅ correct — pure function over CheckCtx
func (g myGate) Run(ctx gates.CheckCtx) []gates.Finding { … }

// ❌ wrong — LLM/network call inside a gate
func (g myGate) Run(ctx gates.CheckCtx) []gates.Finding {
    resp, _ := http.Get("https://api.openai.com/…") // never
}
```

### Evidence integrity
No task completes without a passing verify record (exit code + git HEAD). The
`--unverified --evidence` escape hatch for read-only roles is documented in PROJECT.md
§3 and referenced in `docs/adr-log.md` (ADR-8) but is not yet implemented (see F14).

### Atomic writes + CAS
Every state mutation goes through `core.AtomicWrite` + `core.SaveStateCAS` inside
`core.WithSpecLock`. Writing state outside the lock is a defect.

```go
// ✅ correct — mutation under lock+CAS
core.WithSpecLock(root, func() (struct{}, error) {
    state, _ := core.LoadState(path)
    // … modify state …
    return struct{}{}, core.SaveStateCAS(path, state.Revision, state)
})

// ❌ wrong — unlocked write
core.SaveState(path, state)
```

### Parser byte round-trip
`Serialize(Parse(x)) == x` for `tasks.md`. If you touch `tasksparser.go`, the fuzz test
**must still pass**.

### Zero runtime dependencies
`go.mod` must never gain a `require` stanza. If a capability seems to need a library,
either write the minimal stdlib version or defer the feature.

### Fail-loud posture
Corrupt state, truncated YAML, malformed env — loud error, never silent coercion.

```go
// ✅ correct — loud error on corrupt state
if err := state.Validate(); err != nil {
    return fmt.Errorf("load state: %w", err)
}

// ❌ wrong — silent default on parse error
if err != nil {
    state = State{} // never
}
```

### Subtractive bias
When unsure whether something belongs in the harness, default to **CUT/DEFER** and record
the reasoning via `specd decision`. The target is the minimal accurate surface, not
feature parity with v1.

---

## 5. Adding a gate

Gates are the heart of the harness. Adding one requires exactly **one registration call**
and zero edits to the check runner:

```go
// 1. Create internal/core/gates/mygate.go
package gates

type myGate struct{}

func (g myGate) Name() string { return "my-gate" }

func (g myGate) Run(ctx CheckCtx) []Finding {
    // Pure function over ctx — no IO, no network.
    if someViolation(ctx) {
        return []Finding{{Gate: g.Name(), Severity: Error, Message: "explanation"}}
    }
    return nil
}

// 2. Register in CoreRegistry() (internal/core/gates/core.go) or
//    in the command handler for opt-in gates.
func CoreRegistry() *Registry {
    r := &Registry{}
    r.Register(myGate{})
    // … other gates …
    return r
}
```

Gate rules:
- Gate bodies are **pure** — `CheckCtx` is the only input; no fs/net handles.
- Severity: `warn` (report, exit 0) or `error` (block, exit 1). Gates may pin a minimum
  floor; config can raise, not lower.
- Evidence gate (gate 5) severity is **always `error`** and can never be opt-out.
- External custom gates: subprocess contract (stdin/stdout JSON, scrubbed env, bounded timeout).

---

## 6. Adding a command

New commands require changes in **two places only**:

1. **`internal/core/commands.go`** — add a `Command{}` entry to `Commands[]`.
   This is the single source of truth for help, MCP tools, and the dispatch registry.

2. **`internal/cmd/registry.go`** — add the handler function to `executable` map
   and implement `runMyCommand`.

The handler must:
- Validate argument count first (usage error exits 2, not 1).
- Never contain business logic — delegate to `internal/core`.
- Return `nil` on success, an `error` on failure.

Verify the charter invariant: every new command must map to one harness component and one
principle in `docs/charter.md`.

---

## 7. Testing

### Test naming conventions

- Unit tests live alongside source: `foo.go` → `foo_test.go`.
- E2E tests live in `internal/cmd/e2e_test.go`.
- Parity tests: `internal/core/gates/parity_test.go` — ensures tool result == CLI JSON.
- Registry test: `TestRegistryMatchesHelp` in `internal/cmd/registry_test.go` — CI-blocking.

### CI-blocking tests

These must pass on every commit:

```bash
go build ./...                          # zero-dep build
go vet ./...                            # static analysis
go test -race ./...                     # all tests + race detector
go test -fuzz=FuzzParseSerialize \      # fuzz smoke (10s)
    ./internal/core/ -fuzztime=10s
```

### Writing a verify command for a spec task

The `verify:` field in `tasks.md` must be a self-contained shell command that returns
exit 0 on success and non-zero on failure. It runs in a **scrubbed environment** — do not
rely on env vars outside the allowlist (`PATH, HOME, LANG, LC_ALL, TMPDIR, SPECD_*`).

```markdown
| T3 | craftsman | internal/core/state.go | T2 | go test ./internal/core/ -run TestSaveStateCAS | TestSaveStateCAS passes |
```

---

## 8. Working with the spec lifecycle (dogfooding)

specd is designed to be driven by specd. New features should be driven through the CLI:

```bash
# 1. Initialize (if not already)
specd init

# 2. Create a spec for your feature
specd new my-feature

# 3. Author requirements.md (EARS-shaped)
$EDITOR .specd/specs/my-feature/requirements.md

# 4. Check the gate
specd check my-feature

# 5. Approve requirements
specd approve my-feature requirements

# 6. Author design.md
$EDITOR .specd/specs/my-feature/design.md
specd approve my-feature design

# 7. Author tasks.md (DAG pipe table)
$EDITOR .specd/specs/my-feature/tasks.md
specd approve my-feature tasks

# 8. Get the next task
specd next my-feature

# 9. Get context for a task
specd context my-feature T1

# 10. Verify a task
specd verify my-feature T1

# 11. Complete a task (requires passing verify)
specd task complete my-feature T1
```

---

## 9. Open findings (current position)

The project is on the `fresh-start` branch. Several critical gaps exist before production
readiness (see `PROJECT.md §8` for the full audit). The priority order:

| Wave | Focus | Key findings |
|---|---|---|
| **P0** | Restore truth: re-audit `progress.md`, reset `.specd/`, write missing docs | F1 (falsified completions) |
| **P1** | Close the loop: `task complete` enforcement, ADR-7 mode enum | F2 (no loop closure), F5 (mode enum) |
| **P2** | Seal the trust boundary: MCP deny-list, `brain` fail-closed | F3 (approvals gate nothing), F4 (MCP approve) |
| **P3** | Make records mean something: timestamps, git HEAD, actor on every record | F6 (hollow records) |
| **P4** | Finish gate engine: EARS gate, steering into context manifest | F8 (missing gates), F9 (steering inert) |
| **P5** | Surface & config reconciliation: 16-verb count, `config.yml` seeding | F7 (18 vs 16 verbs), F10 (config filename) |
| **P6** | Hardening & release: CI, static binaries, dogfood gate | — |

> **Do not ship before P0 + P1 + the MCP deny-list land.** A spec-discipline tool whose
> own tracker lies and whose loop can't close teaches users that evidence is theater.

**Finding summary (🔴 blocks a guardrail · 🟠 spec drift · 🟡 quality):**

| ID | Severity | Description |
|---|---|---|
| F1 | 🔴 | `progress.md` falsified completions — verify commands fail; no evidence records exist |
| F2 | 🔴 | Core loop cannot close — no `task complete` verb enforces evidence |
| F3 | 🔴 | `approve`/`next`/`verify` never read `state.json`; phase ratchet writes state nothing consults |
| F4 | 🔴 | MCP `ForbiddenTool` doesn't block `approve`, `init`, `brain`, `mcp` |
| F5 | 🔴 | ADR-7 mode enum unimplemented (`simple`/`orchestrated` don't exist; no `--mode` flag) |
| F6 | 🔴 | `decision`/`midreq` capture no content; approvals record only `{"gate":"..."}`  |
| F7 | 🟠 | 18 verbs shipped vs spec'd 16 (`triage`/`memory` extra) |
| F8 | 🟠 | Missing EARS gate, approval/phase gate, Sync gate |
| F9 | 🟠 | Steering + memory scaffolded but inert; context manifest excludes them |
| F10 | 🟠 | Config reads `project.yml` not `config.yml` (ADR-2); errors swallowed |
| F11 | 🟠 | `brain start` not fail-closed without `orchestration.enabled` |
| F12 | 🟠 | Repo's own `.specd/` has stale `roles/scribe.md`, missing `auditor.md` |
| F13 | 🟡 | `progress.md` `files:` don't match real tree |
| F14 | 🟡 | `--unverified --evidence` escape hatch documented but not implemented |

---

## 10. Regression scripts

Three deterministic scripts under `scripts/` enforce standing regressions:

```bash
./scripts/regress-all.sh       # re-runs every verify: in tasks.md files literally
./scripts/regress-lint.sh      # static smell audit (hollow verify, bad file paths…)
./scripts/regress-domains.sh   # domain invariants black-box against a fresh binary
```

No LLM, no network in any verdict path. Run these before opening a PR.
