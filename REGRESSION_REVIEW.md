# REGRESSION_REVIEW.md — Audit of Spec 13 (CLI Seam Regression) implementation

> **Scope.** `specs/13-cli-regression/` (R13.1–R13.11 / T13.1–T13.15): the spec that
> closes the CLI seam and makes the harness enforce ADR-8 on itself.
> **Method.** Every requirement re-verified by running the built binary
> (`go build -o /tmp/specd .`) in throwaway repos and by executing each task's literal
> `verify:` command — not by trusting `progress.md` (PROJECT.md §8 F1, and the standing
> memory: distrust `progress.md` until re-audited).
> **Date.** 2026-07-04. **Suite state at audit:** `go build`/`go vet` clean, `go test ./...`
> green (102 tests).

---

## 1. Verdict at a glance

The seam is **mostly closed**: the dispatcher fails closed, the parity test guards the
command↔handler maps, and the lifecycle (`init→new→check→approve→next→verify→report`)
runs end-to-end through a real binary. **5 gaps remain**, one of them a genuine
correctness bug (the ADR-7 mode enum), the rest honesty/traceability debt that the spec
itself names as its reason for existing.

| Req / Task | Claim | Verified? | Evidence |
|---|---|---|---|
| R13.1 / T13.1 — fail-closed exit 2 | done | ✅ PASS | `/tmp/specd bogusverb` → exit 2 |
| R13.2 / T13.2 — parity test | done | ✅ PASS | `TestEveryCommandHasHandler` green; `triage` carries `Deferred:true` |
| R13.6 / T13.3 — `check` registered, runs gates | done | ⚠️ WEAK | see **G4** — literal verify hits a non-existent runtime spec |
| R13.3 / T13.4 — `new` writes `state.json` rev0 / **mode simple** / status requirements | done | ❌ GAP | see **G1** — writes `mode:"default"`, not `simple` |
| R13.4 / T13.5 — approve gates | done | ✅ PASS | `TestApproveGatesE2E` green |
| R13.5 / T13.6 — midreq/decision append via CAS | done | ✅ PASS | `TestMidreqDecisionAppend` green |
| R13.10 / T13.7 — status/next/verify/context/report on a real spec | done | ✅ PASS | `TestStatusNextVerifyOnRealSpec` green |
| R13.7 / T13.8 — brain drives driver, fail-closed | done | ✅ PASS | `TestBrainDispatchesFrontierViaCLI` green |
| R13.8 / T13.9 — triage runs or reports deferral | done | ✅ PASS | `triage t` → explicit deferral notice, non-silent |
| R13.9 / T13.10, T13.11 — help + task | done | ✅ PASS | `help new`, `help --json`, `TestTaskShowsDetails` |
| R13.10 / T13.12 — e2e golden through built binary | done | ✅ PASS | `TestLifecycleE2E` green (now also drives `task complete`) |
| — / T13.13 — `verify-progress.sh` | done | ✅ PASS | script present, exits 0 |
| R13.11 / T13.14 — `progress.md` reconciled to reality | done | ❌ GAP | see **G2** — still says `100%`, omits `13-cli-regression` |
| — / T13.15 — record cmd consolidation as ADR | done | ❌ GAP | see **G3** — target file `fresh-start/00-decisions.md` was deleted |

---

## 2. Gaps & improvements

### G1 — ADR-7 mode enum is unimplemented (this is also F5) 🔴 HIGH — correctness

**What R13.3 requires:** `new` writes `state.json` with `mode: "simple"`. ADR-7 (PROJECT.md
§4, canonical) is explicit: the enum is exactly **`simple`** / **`orchestrated`**, default
`simple`, set at `new --mode`, changed only via an auditable `approve --mode` transition;
orchestration eligibility keys off `mode: orchestrated`.

**What the code does:** `internal/core/state.go` defines `ModeDefault = "default"` and
`ModeAgent = "agent"`. `new` has no `--mode` flag; there is no `approve --mode` path.

```
$ /tmp/specd new demo && grep '"mode"' .specd/specs/demo/state.json
  "mode": "default",        # ADR-7 mandates "simple"
```

**Why it matters:** the mode enum is the ADR-7 seam that gates the entire orchestration
tier (`brain start` eligibility). Wrong values mean the string an operator or `brain`
would test against (`orchestrated`) can never appear, and R13.3's on-disk contract is not
met. It is a real divergence from the binding ADR, not doc drift.

**Action plan:**
1. `internal/core/state.go` — rename the enum to the ADR-7 values:
   `ModeSimple Mode = "simple"`, `ModeOrchestrated Mode = "orchestrated"`; `InitialState`
   defaults to `ModeSimple`; add a `ValidMode` check to `State.Validate()`.
2. `internal/cmd/registry.go` (`runNew`) — add a `--mode` flag (`simple|orchestrated`,
   default `simple`, fail-loud on any other value) and thread it into `InitialState`.
3. `internal/cmd/lifecycle.go` (`runApprove`) — add the `approve <spec> --mode <target>`
   transition: append a stamped `mode` record and persist via `SaveStateCAS` (auditable,
   forward-only; no silent mutation).
4. Update the `brain start` precondition to key off `ModeOrchestrated` (grep the current
   check — commit db20eb3 added orchestration preconditions; point it at the renamed const).
5. Sweep `ModeDefault`/`ModeAgent` references (`grep -rn ModeDefault ModeAgent internal/`)
   and fix tests + `state.json` golden fixtures.
6. **Verify:** `new` (no flag) → `mode:"simple"`; `new --mode orchestrated` →
   `mode:"orchestrated"`; `new --mode bogus` → non-zero, fail-loud; `approve --mode`
   writes a mode record and bumps revision. Add `TestModeEnumEndToEnd`.

> Guardrail note: this straddles Spec 13 R13.3 and BUILD_REVIEW F5 / PROJECT.md Wave P1.
> Doing it here closes both. `state.json` `SchemaVersion` stays `1` (ADR-2): the enum
> string changes, the schema shape does not.

---

### G2 — `progress.md` still reports falsified completion (R13.11 / T13.14) 🔴 HIGH — honesty

**What T13.14 requires:** `grep -q '13-cli-regression' specs/progress.md && ! grep -q '100%' specs/progress.md`.

**Current state:** `specs/progress.md` still contains `100%` and has **no** `13-cli-regression`
row. The acceptance predicate fails. This is the exact falsified-tracker problem
(F1/F13) that Spec 13 exists to end, and the standing memory flags `progress.md` as
untrustworthy pending re-audit.

**Action plan:**
1. Re-audit `progress.md` the way this review was done: for each task run its integration
   `verify:` literally; demote every task whose verify does not genuinely pass to its real
   status; delete the absolute `100%` claim.
2. Add the `13-cli-regression` rows reflecting **this** audit (G1/G3/G4 open, the rest
   green).
3. **Verify:** the T13.14 grep predicate passes.

> This overlaps review-specs **W0 (restore-truth, R0.1/R0.3)**. Do it once, here, and let
> W0 consume the result rather than re-auditing twice.

---

### G3 — cmd-consolidation ADR has no live home (T13.15) 🟠 MED — traceability

**What T13.15 requires:** record the `internal/cmd/*.go` → `registry.go` consolidation as
an ADR so the Definition-of-Done file-scope check (`touches only its declared files:`) is
meaningful again — verify: `grep -qi 'consolidat' fresh-start/00-decisions.md`.

**Current state:** `fresh-start/` was intentionally removed during the fresh-start
consolidation (per project memory, 2026-07-04). The verify target no longer exists, so the
task can never pass as written, and the consolidation is unrecorded in the canonical
`PROJECT.md` §4 ADR list.

**Action plan:**
1. Add a short ADR to `PROJECT.md` §4 (the canonical, surviving location) recording that
   per-command `cmd/*.go` files were consolidated into `registry.go`/`lifecycle.go`, and
   what that means for the DoD file-scope guard (declared `files:` now name the
   consolidated files).
2. Fix the stale verify in `specs/13-cli-regression/tasks.md` T13.15 to point at
   `PROJECT.md` instead of the deleted `fresh-start/00-decisions.md`.
3. **Verify:** `grep -qi 'consolidat' PROJECT.md`.

---

### G4 — R13.6's `check` verify does not exercise a real spec (T13.3) 🟠 MED — evidence quality

**What R13.6 requires:** `check <spec>` runs the gate registry **against a real spec** and
exits non-zero iff a gate errors.

**The weakness:** T13.3's literal verify is `/tmp/specd check 01-product-philosophy-core`.
Authored domain specs live in `specs/` (authoring dir); `check` reads `.specd/specs/`
(runtime dir). So that command hits a spec that is not present at runtime and exits 1 on
"not found" — it satisfies "non-zero, not silent" **without ever running the gate
registry over real content**. The evidence is hollow in exactly the way Spec 13 §7 warns
against ("require each verify to assert a real side effect").

**Mitigation already in place:** `TestLifecycleE2E` now drives `check` against a spec
created by `new` (real gate run, real exit), so R13.6 *is* genuinely covered — by T13.12,
not T13.3.

**Action plan:**
1. Rewrite T13.3's verify to create the spec first:
   `... new demo && printf '<green tasks.md>' > .specd/specs/demo/tasks.md && /tmp/specd check demo` and assert exit 0, then a deliberately-broken tasks.md → assert non-zero.
2. Or, if keeping it a one-liner, point it at a scaffolded `.specd/specs/` fixture.
3. **Verify:** the rewritten command runs gates over real content and its exit tracks
   findings.

---

### G5 — 18 registered verbs vs the 16-verb target (F7) 🟡 LOW — cross-wave (W5), noted not owned

`help` lists 18 verbs (`triage` + `memory` beyond the intended 16). PROJECT.md P5 resolves
this by **cutting `triage`** and deciding `memory` via a superseding ADR after P4 makes
memory functional. Spec 13's R13.8 only requires triage to *report deferral explicitly*
(satisfied by the `Deferred:true` stub). The final subtraction is **W5 / review-specs
R5.1**, not Spec 13. Recorded here so it is not lost; **no action taken in this pass.**

---

## 3. Proposed execution order

```
G2 (honesty) ─┐
G1 (mode enum) ┼─► independent, do first (G1 is the only code-correctness gap)
G4 (check verify) ┘
G3 (ADR record) ──► after G1 lands (the mode work is itself an ADR-touching change worth recording alongside)
G5 ──► deferred to W5, not this pass
```

- **G1** is the highest-value fix: it is the one genuine bug and it also closes F5 / Wave P1.
- **G2** is the cheapest honesty win and unblocks trustworthy re-verification (and W0).
- **G3/G4** are traceability/evidence-quality tightening.
- Each fix lands with its own runnable check; nothing merges until `go build ./... && go vet ./... && go test ./...` stays green (Spec 13 §5, ADR-8 evidence integrity).

## 4. Guardrails honored

- No new packages/dependencies; every fix lands in existing files
  (`state.go`, `registry.go`, `lifecycle.go`, `commands.go`, `PROJECT.md`, the two spec docs).
- ADR-8 hard invariants untouched: mode transitions go through `SaveStateCAS` under the
  per-spec lock; `SchemaVersion` stays `1`; no LLM enters any decision/gate path.
- Subtractive bias: G5 stays a *cut* (triage), not a feature restoration.
- Dogfood: once `task complete` + gates are trusted (they now are, post-W4), these fixes
  should themselves be driven as a spec under this repo's own `.specd/specs/`.

## 5. What I am asking you to confirm before I implement

1. **G1 rename** touches the `Mode` enum repo-wide and the on-disk `mode` string. Green-light
   renaming `ModeDefault→ModeSimple` / `ModeAgent→ModeOrchestrated` and adding `new --mode` +
   `approve --mode`? (This is the ADR-7 contract; I recommend yes.)
2. **G2** — should I re-audit and rewrite `specs/progress.md` now, or leave it for review-specs
   W0 to avoid double work? (I recommend doing it here and having W0 consume it.)
3. **G3/G4** edit two files inside `specs/13-cli-regression/` (fixing stale verify targets).
   OK to amend the spec's own tasks, or should those corrections be recorded as a spec
   amendment/midreq instead?
