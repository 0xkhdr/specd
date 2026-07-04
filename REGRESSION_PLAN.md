# specd — Regression Analysis & Action Plan

> **Status:** `specs/progress.md` declares Stage 3 implementation **100% complete (76/76 tasks
> across waves A–H)**. This document is a full regression against that claim. **Finding: the
> claim is false at the product level.** The core Go library is built and unit-tested green, but
> the CLI seam that turns that library into a usable tool was never closed. The spec lifecycle —
> the entire reason the tool exists — cannot be driven from the command line.
>
> **Method:** built the binary, ran the real CLI end-to-end against a fresh workspace, and
> cross-checked every declared verb, handler, and `go run .` verify command against the actual
> dispatch path. All findings below are reproduced from a running binary, not read from source
> alone.

---

## 1. Intent of the fresh start (what "done" was supposed to mean)

From `FRESH_START_BRIEF.md`, `fresh-start/00-roadmap.md`, `00-decisions.md` and the 12 domain
specs, specd's fresh start is a **deterministic, zero-dependency, agent-agnostic spec-driven
development harness**. Its guardrails (ADRs):

- **Determinism first** — no LLM in any decision, gate, or render path.
- **Evidence integrity absolute (ADR-8)** — a task is done only when its `verify:` command
  passes *and* a record (exit code + git HEAD) is written.
- **Zero runtime dependencies**, `go:embed` templates, atomic writes, CAS state, reentrant lock,
  `ParseTasks` round-trip.
- **Subtractive bias** — prefer deletion/consolidation over addition.

The intended product loop is: `init` → `new <spec>` → author requirements/design/tasks → gate
checks (`check`) → human `approve` → `next`/`verify` task execution with evidence → `report`,
optionally driven by the `brain`/`pinky` orchestration tier.

**The Definition of Done in `progress.md` itself (lines 184–189)** requires each task's `verify:`
to pass with a written record, and guardrails to hold. The regression tests that DoD against
reality.

---

## 2. What actually works

Verified green and genuinely reachable from the CLI:

| Area | Evidence |
|---|---|
| Build / vet | `go build ./...` and `go vet ./...` exit 0 |
| Unit tests | `go test ./...` — all 11 packages pass |
| `init` | Writes 4 roles + 6 steering files under `.specd/` (confirmed on disk) |
| `mcp` | `mcp.Serve` wired; stdio server responds |
| `handshake bootstrap` | Emits version + tool list |
| `next` / `status` / `report` / `context` / `verify` | Handlers exist and are registered |
| Core primitives | atomic write, reentrant lock, CAS state, tasks round-trip, evidence ledger, gates, context manifest — all unit-tested |

The **core library is sound.** The defect is entirely at the CLI composition seam.

---

## 3. Findings (severity-ranked, all reproduced from the running binary)

### F1 — CRITICAL: dispatcher fails *open* — unknown/unhandled commands silently exit 0
`internal/cmd/registry.go:53-59`: `Run` returns `nil` when a command has no handler. `main.go`
never validates the command exists. Reproduced:

```
$ specd bogusverb      → exit 0, no output
$ specd new demo       → exit 0, no output, no spec created
$ specd approve demo x → exit 0, no output
$ specd brain start    → exit 0, no output
$ specd triage demo    → exit 0, no output
$ specd help           → exit 0, no output
```

This single bug is why the "100%" claim survived: **every missing command reports success.**
Any `go run . <verb>` verify that doesn't assert a side effect passes trivially.

### F2 — CRITICAL: the spec lifecycle is unreachable; `SaveState` is never called
Grep across non-test code: **`core.SaveState`/`SaveStateCAS` has zero callers.** No command ever
writes `state.json`. Verbs `new`, `approve`, `midreq`, `decision` have **no handler** in
`executable` (`registry.go:25-35`), and `approve`, `midreq`, `triage`, `help` have **zero
implementation functions anywhere in non-test code**. Consequence: you cannot create a spec,
transition a phase, or record an approval from the CLI. Wave B/C lifecycle tasks (T2.1–T2.7)
were "verified" purely by unit tests (`go test -run TestStateCAS` etc.) — the primitives exist,
nothing calls them.

`progress.md` T2.4 verify is `go run . new demo && test -f .specd/specs/demo/state.json`. `new`
is a no-op, so `state.json` is never created and the verify **cannot pass** — yet T2.4 is marked
✅. This is a DoD violation (marked ahead of evidence, contra `progress.md:194`).

### F3 — CRITICAL: `check` is implemented but never registered (orphaned handler)
`runCheck` exists (`registry.go:61`) and is in the `executable` map, but **`"check"` is not in
`core.Commands`**. `buildRegistry` only iterates `core.Commands`, so `check` is never added to
`Registry` → `specd check demo` hits the F1 no-op path and exits 0 doing nothing. Every
gate-engine integration verify (T3.3 `go run . check demo`, T3.4 `--security`, T8.5 budget) is
**false-green**. Mirror defect: `Commands` lists 16 verbs; `executable` covers 9; the two sets
are maintained by hand and have silently diverged.

### F4 — HIGH: orchestration tier (Wave G, 10 tasks) is test-only scaffolding
`cmd/brain.go` exposes `runBrainDispatch` and `cmd/pinky.go` exposes `requirePassingVerify` —
both **unexported, referenced only by `_test.go` files**, wired to no command. `brain` and
`triage` verbs are unimplemented (F1 no-op). The orchestration/driver/lease/session logic in
`internal/orchestration` is unit-tested but cannot be invoked by a user. The most-composed tier
of the system is a library with no entry point.

### F5 — HIGH: the `go run .` integration verifies are structurally unable to pass
The demo-spec creation (`new`) is the linchpin every integration verify depends on. Because
`new` is a no-op, `go run . next demo`, `verify demo T1`, `status demo --json`,
`context demo <task>`, `check demo` either no-op to exit 0 (false green) or error on a missing
spec. **Not one of the spec-scoped integration verifies genuinely exercises the path it claims
to.** They were marked ✅ regardless — the evidence-integrity guardrail (ADR-8) was not enforced
on the harness that enforces it on everyone else.

### F6 — MEDIUM: `help` and `task` verbs no-op
`help` and `task` are in the palette, have no handler, and no-op via F1. `help` is table-stakes
UX; `task` (show task details) is referenced by `progress.md`'s intended flow.

### F7 — MEDIUM: spec ↔ implementation file-path divergence breaks traceability
`tasks.md` declares one-file-per-command (`new.go`, `approve.go`, `status.go`, `check.go`,
`next.go`, `verify.go`, `cmd/context.go`, `cmd/mcp.go`, `dispatch.go`, `verify/capture.go`,
`evidence/ledger.go`). The implementation consolidated these into `registry.go` and renamed
packages (`evidence.go` not `evidence/ledger.go`, no `verify/capture.go`). Consolidation is
legitimate under subtractive bias — but `progress.md`'s `files:` columns are now **fiction**, so
the "it touches only the `files:` it declares" DoD check (line 187) is unverifiable. Either the
tasks were not followed as written or the plan was never reconciled.

### F8 — LOW: no unknown-command UX
Even after fixing F1, users get nothing helpful. Should error with the unknown verb and a
"did you mean" / usage pointer.

---

## 4. Root-cause synthesis

One decision explains all findings: **the build waves adopted `go test -run TestX` as the
definition of done for the majority of tasks.** Unit tests pass against pure core functions and
never touch `main`. The handful of tasks whose verify was a real `go run .` invocation were
silently satisfied by the fail-*open* dispatcher (F1). So the seam between "correct core library"
and "user-invocable product" was never actually built or tested. The result is a well-tested
library wearing a CLI costume: `init`/`mcp`/`handshake` work; the spec lifecycle, gate CLI, and
orchestration do not.

**The single guard that would have caught F1–F4, F6 at once:** a test iterating `core.Commands`
and asserting `Registry[name] != nil` for every verb. It does not exist. Its absence is the
meta-finding — the harness has no test that the harness is wired.

---

## 5. Action plan → regression spec `13-cli-regression`

Proposed remediation, grouped into gated waves. Each task keeps ADR-8 discipline: its verify
must **assert a side effect**, not just exit 0.

### Wave R1 — Dispatcher integrity (fail-closed) — *do first, unblocks honest verification*
- **R1.1** `Run`/`main` return a non-nil error + usage for any command absent from the registry;
  exit 2. *Verify:* `specd bogusverb; test $? -eq 2`.
- **R1.2** Registry parity test: iterate `core.Commands`, assert every verb has a non-nil handler
  (or an explicit, documented "metadata-only" allowlist). *Verify:* `go test ./internal/cmd -run
  TestEveryCommandHasHandler`. **This test is the linchpin — it fails today and must stay green
  forever.**
- **R1.3** Register `check` (add to `core.Commands`) or delete `runCheck` if `check` is folded
  into another verb. *Verify:* `specd check <real-spec>` exits per gate result, not silently.

### Wave R2 — Lifecycle wiring (depends R1)
- **R2.1** `new <name>` → creates `.specd/specs/<slug>/` with initial `state.json` via
  `SaveStateCAS`, seeds `spec.md`/`tasks.md` from embedded templates. *Verify:* `specd new demo
  && test -f .specd/specs/demo/state.json`.
- **R2.2** `approve <spec> <gate>` → phase ratchet + evidence-backed approval record. *Verify:*
  approve advances phase and is rejected when gates are red.
- **R2.3** `midreq` / `decision` → append scoped requirement/decision records. *Verify:* record
  written and reloads.
- **R2.4** `status`/`next`/`verify`/`context`/`report` re-verified against a **real** `new`-created
  demo spec (retire the false-green integration verifies from F5).

### Wave R3 — Orchestration wiring (depends R2)
- **R3.1** `brain <start|step|run|status|approve|cancel|resume>` → export and wire the driver;
  fail-closed authority. *Verify:* `specd brain start <spec>` produces a session + evidence,
  `go test -run TestBrainDriverDispatchesFrontier` still green through the CLI path.
- **R3.2** `triage <spec>` → wire the opt-in tier or explicitly mark deferred in `Commands`
  metadata (not silently no-op).

### Wave R4 — UX + surfaces
- **R4.1** `help [command] [--json]` handler backed by `core.Commands` metadata. *Verify:*
  `specd help new` prints usage; `--json` is machine-readable.
- **R4.2** `task <id>` handler (show task details from `tasks.md`).

### Wave R5 — Evidence-integrity harness (the meta-fix)
- **R5.1** End-to-end golden test: `init → new → author → check → approve → next → verify →
  report` in a temp repo, asserting on-disk side effects at each step. *Verify:* `go test
  ./internal/cmd -run TestLifecycleE2E`.
- **R5.2** A `progress.md` audit script: every task whose verify is `go run .` must be executed
  by the e2e harness, not marked by hand.

### Wave R6 — Reconciliation & truth
- **R6.1** Rewrite `progress.md` to reflect verified reality (demote falsely-green tasks F2/F3/F5
  to ⬜ until R1–R5 land).
- **R6.2** Reconcile `tasks.md` `files:` columns with the consolidated implementation, or add an
  ADR recording the consolidation so the DoD file-scope check is meaningful again (F7).

---

## 6. Suggested sequencing

`R1 → R2 → R3` is the critical path (dispatcher must fail closed before any lifecycle verify can
be trusted; lifecycle must exist before orchestration composes it). R4 is parallelizable after
R1. **R5.1 (the parity test + e2e golden) should land inside R1** — it is the cheapest, highest-
leverage guard and converts every subsequent wave from "trust the checkbox" to "trust the
evidence." R6 closes the loop by making `progress.md` a projection of reality again, per its own
stated contract (`progress.md:191-194`).

---

## 7. One-line summary for stakeholders

> The core library passes all unit tests, but the CLI never wired the spec lifecycle, the gate
> command, or the orchestration tier; a fail-open dispatcher made every missing command report
> success, so `progress.md`'s "100% complete" is a measurement artifact, not a working product.
> The fix is small and mechanical (wire handlers + fail closed) and its permanent guard is a
> single registry-parity test plus one end-to-end golden test.
