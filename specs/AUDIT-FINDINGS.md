# Wave Audit — Findings & Action Plan

Audit of `specs/progress.md`, the nine gap-closure specs, and their implementation against
`GAP-ANALYSIS.md`. Date: 2026-07-09. Every finding cites code or a reproducible command.

**Priorities:** P0 = false record / broken CI / violates a stated invariant; P1 = real open
capability gap; P2 = tracking/consistency polish.

---

## 0. Verification baseline (what actually passes)

Ran on a fresh build of `HEAD` (`ce93757`). All green:

| gate | command | result |
|---|---|---|
| build | `go build .` | exit 0 |
| vet | `go vet ./...` | exit 0 |
| gofmt | `gofmt -l .` (non-`reference/`) | 0 files |
| race | `go test ./... -race -count=1` | exit 0 |
| flake | `go test ./... -count=2` | exit 0 (285 tests) |
| docs lint | `./scripts/docs-lint.sh` | exit 0 |
| test lint | `./scripts/test-lint.sh` | exit 0 |
| tidy | `go mod tidy` diff | clean |
| installer | `./scripts/install-scripts-test.sh` + `shellcheck` | exit 0 |

**Conclusion:** the *code* for the eight wave specs and the installer is real and passing.
The gaps below are in **tracking accuracy** and in **Part II items that were never
implemented or silently diverged** — not in the code that CI already covers.

---

## 1. Tracking drift (P0 — the record lies)

### 1.1 `progress.md` marks waves `done` on a fraction of the evidence
The completion rules in `progress.md` require recording each task's passing `verify` output.
The Notes cite only:
- Wave 2 "complete": **ESG-T3/T4, OWR-T3** — but wave 2 across the 8 specs has ~15 tasks
  (AWM-T3/T4, CCH-T3/T4, CWI-T3/T4, CMS-T3/T4, IHS-T3/T4, PDC-T3/T4, OWR-T4). 12 unrecorded.
- Wave 3 "complete": **PDC-T5 only** — but wave 3 has 8 validator tasks
  (AWM-T5, CCH-T5, CWI-T5, CMS-T5, ESG-T5, IHS-T5, OWR-T5). 7 unrecorded.

The suite happens to pass, so the *outcome* is right, but the record asserts far more than it
verified. Under the spec's own "mark done only after its verify exits 0 and output is
recorded" rule, most tasks are unproven-on-paper.

### 1.2 The wave board contradicts itself
`progress.md` table row 17: wave 3 `status = todo`. Notes line 48: "Wave 3 complete." Pick one.

### 1.3 `install-scripts` is off the board and mis-specified (P0)
- Not listed in the Wave Board at all (the board names 8 specs; 9 exist).
- Uses a **different schema** (`status`/`dependencies`, tasks `T1..T8`) than the other eight
  (`wave`/`deps`, `<PREFIX>-T#`). The dispatch board only understands the wave schema.
- Every task still says `pending`, yet `scripts/install.sh`, `scripts/uninstall.sh`, and
  `scripts/install-scripts-test.sh` **exist, pass shellcheck, and run green**, and CI already
  invokes them (`.github/workflows/ci.yml`, `release.yml`).
- **Name mismatch:** spec T5 declares `scripts/install-test.sh`; the real, CI-referenced file
  is `scripts/install-scripts-test.sh`. The spec points at a file that does not exist.

### 1.4 Dogfooding gap — the harness does not track its own specs (P1, headline)
These nine specs live in **top-level `specs/`** as hand-maintained markdown, not in
`.specd/specs/` under specd's own evidence-gated pipeline. There is no `state.json`, no
verify record, no CAS, no lock — exactly the drift-prone manual tracking the product exists to
replace. `progress.md` drifting from reality (§1.1–1.3) is the predicted failure mode of not
eating your own dog food. Consider running the gap-closure work *through* `specd` itself.

---

## 2. Part II gaps never implemented or silently diverged

`GAP-ANALYSIS.md` Part II defines **Waves 4–6** (`enforcement-integrity`,
`orchestration-reachable`, `context-and-execution-truth`). `progress.md` only models Waves
1–3. The nine specs reorganized Part II by domain, but several Part II tasks have no
corresponding code. Confirmed by grep against `HEAD`:

| # | GAP item | Part II task | status in code | pri |
|---|---|---|---|---|
| 2.1 | `brain run` ≡ `brain step` (5.3) | W5-T3 | **OPEN** — `brain_run.go:49` is `case "step", "run":`, a shared branch. Alias never resolved or removed. | P1 |
| 2.2 | `mission claim`/`mission report` verbs (5.2) | W5-T2 | **DIVERGED** — no such verbs. Worker round-trip goes through `brain_worker.go` + `pinky-*` subagents. Legitimate, but the spec's named verbs don't exist and no decision is recorded. | P1 |
| 2.3 | Release lease on completion; stress asserts no stale live leases (5.4) | W5-T5 | **OPEN** — no `Release` func in `internal/orchestration/`. `scripts/stress-orchestration.sh` asserts session-revision CAS + single dispatch, **not** lease release. | P2 |
| 2.4 | `verify.timeout_seconds`; timeout recorded as failing evidence (4.2) | W6-T4 | **OPEN** — `verify/exec.go` takes a `ctx` and uses `exec.CommandContext`, but `Options` has no timeout and `config.TimeoutSecs` bounds **submit**, not verify. No deadline reaches the verify command. | P1 |
| 2.5 | Record `git_dirty`; surface dirty counts in `report`; `verify.require_clean` (4.3) | W6-T5 | **DIVERGED** — no `git_dirty`/`require_clean` in `evidence.go`. ESG-T4 instead added a config-driven clean-worktree **gate**. Different contract; not recorded as a divergence. | P1 |
| 2.6 | `specd config [--json]` verb; decide `project.yml` vs `.specd/config.yml` (7.2/7.3) | W6-T6 | **DIVERGED** — no `config` verb (only an `init`/`mcp` flag). PDC-T3 exposes diagnostics via `status`/`check`. The `project.yml`-vs-`.specd/config.yml` location decision is still unrecorded. | P1 |

**Confirmed closed** (spot-checked, so the table above is the real remainder):
- Sandbox requirable by policy (4.4): `gates/security/gate.go:129` fails closed when
  `cfg.Sandbox != "off" && !sandboxActive()`. ✅
- Unknown-verb fail-closed + only `triage` deferred (`commands.go:384`). ✅
- Evidence/`HeadPinned` unification (4.1): covered by ESG-T1 tests, suite green. ✅

---

## 3. Action plan

Ordered by "false record first, then real gaps, then polish." Each item is small.

### Phase A — make the record true (P0, do first, no code risk)
1. **Reconcile `progress.md`.** Either record the actual per-task verify evidence for waves 2–3
   (the suite passes, so back-fill the commands + exit codes) or downgrade the Notes to "suite
   green; per-task evidence not individually recorded." Fix the wave-3 `todo`/`complete`
   contradiction.
2. **Put `install-scripts` on the board** and convert its `tasks.md` to the wave schema (or
   document why it's exempt). Mark T1–T7 `done` (they are), and either **create
   `scripts/install-test.sh`** or **rename the spec reference to `install-scripts-test.sh`** so
   the spec matches the CI-wired reality. (Rename is the one-line fix.)
3. **Record the divergences** from §2 (items 2.2, 2.5, 2.6) as explicit decisions in the owning
   spec — the specs claim `done` while silently taking a different design than GAP-ANALYSIS
   prescribed. A one-paragraph "Decision:" note in each restores traceability.

### Phase B — close or formally defer the real gaps (P1)
4. **`brain run` (2.1):** resolve the alias — either implement loop-until-brake with per-step
   checkpoints, or delete `run` from the usage string and route it to a deferral notice. Record
   the choice. Verify: `go test ./internal/cmd -run Brain`.
5. **Verify timeout (2.4):** add `verify.timeout_seconds` to config, thread a deadline into
   `verify.Run` via `context.WithTimeout`, and record a timeout as a **failing** evidence
   record (not a crash). This is a genuine safety gap — an unbounded verify hangs the pipeline.
   Verify: `go test ./internal/core/verify ./internal/cmd`.
6. **Evidence honesty (2.5):** if the clean-worktree *gate* is the accepted design, say so and
   close 4.3; otherwise add `git_dirty` to the evidence record. Don't leave both half-built.
7. **`specd config` (2.6):** decide — the `status`/`check` exposure may be sufficient; if so,
   mark W6-T6 superseded by PDC-T3 and settle the `project.yml` vs `.specd/config.yml` location
   in docs. Only build a dedicated verb if a consumer needs machine-readable effective config.

### Phase C — polish (P2)
8. **Lease release (2.3):** release the lease on task completion and extend
   `stress-orchestration.sh` to assert no stale live leases after a full run. Low blast radius;
   do it when touching orchestration next.

### Standing invariants (unchanged)
Zero new deps, atomic writes, CAS on state, no LLM in any gate/decision path, docs-lint green,
palette changes update `command-reference.md` + `CHEATSHEET.md` together. `reference/` untouched.

---

## 4. One-line summary
The eight wave specs and the installer are **implemented and CI-green**; the failure is
**bookkeeping** — `progress.md` over-claims completion, `install-scripts` is off-board and
mis-named, and ~6 Part II (Wave 4–6) items are unimplemented or diverged without a recorded
decision. Fix the record first (Phase A, no code risk), then close the verify-timeout and
`brain run` gaps (Phase B).
