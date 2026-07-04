# Spec 13 — CLI Seam Regression & Wiring Closure

> **Authoring order:** 13 / 13 (regression) · **Critical path:** yes (blocks trustworthy Stage 3)
> **Sources:** `REGRESSION_PLAN.md`, `specs/progress.md`, `fresh-start/00-roadmap.md`
> **ADRs:** ADR-2, ADR-7, ADR-8 (evidence integrity — the guardrail this spec restores)
> **Reference:** `internal/cmd/registry.go`, `internal/core/{commands,state,phases}.go`,
> `internal/cmd/{brain,pinky}.go`, `internal/orchestration/*.go`

The core library is built and unit-tested green, but the CLI seam that turns it into a usable
product was never closed. The dispatcher fails **open** — unknown and unhandled commands exit 0
silently — so the spec lifecycle, the gate command, and the orchestration tier are all
unreachable while `progress.md` reports 100%. This spec closes the seam and makes the harness
enforce ADR-8 on itself: a verb is done only when a running binary exercises it and asserts a
side effect.

---

## 1. Purpose & principles
- **Principles owned:** P2 (Specs as Source of Truth) and P6 (Human Gates) — both currently
  unenforceable from the CLI because no command writes `state.json` or records approval.
- **Regression thesis:** the build waves used `go test -run TestX` as Definition of Done. Unit
  tests pass against pure core functions and never touch `main`. The few real `go run .` verifies
  were satisfied by the fail-open dispatcher. The correct library therefore ships with a
  non-functional CLI. This spec is the seam test and the missing handlers.

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| Fail-open dispatch (`Run` returns nil for unknown/unhandled verb) | **REDESIGN** → fail-closed, exit 2 | `internal/cmd/registry.go:53-59`; `main.go` no validation |
| Hand-maintained `Commands` ⇄ `executable` maps that silently diverge | **REDESIGN** → parity test is source of truth | 16 verbs declared, 9 handlers, `check` orphaned |
| `check` handler present but absent from `core.Commands` | **KEEP handler, REGISTER** | `registry.go:26,61`; not in `commands.go` |
| Spec lifecycle verbs `new`/`approve`/`midreq`/`decision` | **REDESIGN** → implement, wire to `SaveStateCAS` | zero callers of `SaveState`; verbs no-op |
| Orchestration `brain`/`triage` (unexported test scaffolding) | **REDESIGN** → export + wire, or mark deferred in metadata | `cmd/brain.go`, `cmd/pinky.go` referenced only by `_test.go` |
| `help`/`task` verbs | **KEEP palette, IMPLEMENT** | no-op via fail-open path |
| `progress.md` "100%" + `tasks.md` `files:` columns | **CUT the false claim** → reconcile to evidence | consolidation into `registry.go` un-recorded |

**Minimal accurate surface:** every entry in `core.Commands` resolves to a non-nil handler or an
explicit `deferred` metadata flag; unknown verbs fail closed; one end-to-end golden test drives
`init → new → check → approve → next → verify → report` asserting on-disk side effects.

## 3. Requirements (EARS)
- **R13.1** When a command name is not in the registry, the system shall print the unknown verb
  plus usage to stderr and exit with code 2 — never exit 0.
- **R13.2** The system shall guarantee, by test, that every verb in `core.Commands` resolves to a
  non-nil handler or carries an explicit `Deferred: true` metadata flag; a verb that is neither
  shall fail the build's test suite.
- **R13.3** When `new <name>` is invoked, the system shall create
  `.specd/specs/<slug>/{spec.md,tasks.md,state.json}` with `state.json` at `revision: 0`,
  `mode: "simple"`, status `requirements`, persisted via `SaveStateCAS` under the per-spec lock.
- **R13.4** When `approve <spec> <gate>` is invoked and the current phase readiness gates do not
  pass, the system shall refuse the transition, report failing gates, and leave `state.json`
  unchanged; on pass it shall ratchet the phase and append an approval record.
- **R13.5** When `midreq <spec>` or `decision <spec>` is invoked, the system shall append the
  scoped record to state and persist via CAS without altering unrelated core fields.
- **R13.6** When `check <spec>` is invoked, the system shall run the gate registry against a real
  spec and exit non-zero iff a gate emits an error; `check` shall be a registered verb.
- **R13.7** When `brain <subcommand> <spec>` is invoked, the system shall drive the deterministic
  orchestration controller (fail-closed authority) and write session + evidence; no LLM in the
  decision path.
- **R13.8** When `triage <spec>` is invoked, the system shall either run the opt-in tier or, if
  deferred, report deferral explicitly — never silently no-op.
- **R13.9** When `help [command]` is invoked, the system shall render usage from `core.Commands`
  metadata, with `--json` machine-readable; when `task <id>` is invoked it shall print that
  task's parsed details.
- **R13.10** The system shall provide an end-to-end test that drives the full lifecycle through a
  built binary in a temp repo and asserts on-disk side effects at each step; every task whose
  verify is `go run .` shall be covered by it.
- **R13.11** The system shall update `progress.md` to a projection of verified reality, demoting
  every task whose integration verify does not genuinely pass until this spec's tasks are green.

## 4. Design

### Module boundaries
- `internal/cli/args.go`, `main.go` — dispatch entry; add fail-closed unknown-verb path (R13.1).
- `internal/cmd/registry.go` — the `executable` map; add `new`/`approve`/`midreq`/`decision`/
  `brain`/`triage`/`help`/`task` handlers; register `check`. Handlers stay thin, delegating to
  `internal/core` and `internal/orchestration` (no business logic leaks into `cmd`).
- `internal/core/commands.go` — add `Deferred bool` to `Command`; single source of truth for the
  parity test (R13.2).
- `internal/cmd/brain.go`, `internal/cmd/pinky.go` — promote the test-only helpers into real,
  exported command entry points wired to `internal/orchestration`.

### Key types / interfaces
- Reuse `core.State`, `core.SaveStateCAS`, `core.PhaseReadiness`, `core.CompleteTask` — all
  already unit-tested; this spec only supplies their missing callers.
- `Command{ …, Deferred bool }` — a deferred verb is registered, prints a deferral notice, exits
  0; the parity test treats it as satisfied.

### On-disk contracts
- `new` writes the Spec 02 contract exactly: `state.json` (0644), forward-only status, monotonic
  revision, byte-identical for a Simple spec that never opts into orchestration.
- No new on-disk formats introduced — this spec is wiring, not schema.

### External interfaces
- CLI verbs become genuinely invocable; MCP/handshake surfaces unchanged.

## 5. Invariants preserved (ADR-8)
Evidence integrity is the invariant this spec **restores**: no verb is done without a running
binary exercising it and asserting a side effect. CAS-on-revision, SaveState-must-be-locked,
atomic write, forward-only ratchet, determinism (no LLM in decision/gate/render) all hold —
handlers call the existing guarded primitives, adding no bypass.

## 6. Cross-domain dependencies
- Depends on: Spec 02 (state/phases), Spec 03 (gate registry), Spec 04 (frontier), Spec 05
  (evidence), Spec 09 (orchestration driver), Spec 10 (dispatch/args). All already implemented as
  libraries — this spec supplies their entry points.
- Consumed by: every user of the CLI; unblocks honest re-verification of waves B, C, D, G.

## 7. Risks & open questions
- **Risk:** re-wiring leaks business logic into `cmd`. → handlers stay thin; logic stays in
  `core`/`orchestration`; enforce by review + keeping handler bodies small.
- **Risk:** parity test forces stub handlers just to pass. → require each new handler's verify to
  assert a real side effect (file created, phase advanced, non-zero exit), not exit 0.
- **Open (ADR):** record the consolidation of one-file-per-command into `registry.go` as an ADR
  so the DoD file-scope check is meaningful again, or restore per-command files. Resolve in T13.11.
