# specd — Program Analysis & Action Plan

_Generated 2026-07-13 on branch `sdlc-specs` (HEAD `11dc841`). Based on a full re-run of every
gate against a freshly built binary, not on the checkbox state in `specs/progress.md`._

---

## 1. What specd is (foundational model)

`specd` is a **spec-driven coding harness**: a single static Go binary (stdlib only, zero runtime
deps) that moves SDLC process enforcement out of the LLM context window into a deterministic,
tool-gated pipeline — **requirements → design → tasks → evidence-gated execution**. The agent
reasons; the harness enforces.

**Pipeline of the codebase itself.** specd is built _by_ a specd-style process. Ten domain specs
live under `specs/0N-*/` — each with `requirements.md` (EARS), `design.md`, `tasks.md` (a wave
DAG of evidence-gated task rows). `specs/progress.md` is the **cross-domain flat DAG walk order**
(P0 → P1 → P2 stages); `specs/prompt.md` is the per-turn implementation protocol (one subagent,
one eligible wave, verify → mark → commit, strictly serial).

**The ten domains**

| # | Domain | Core deliverable |
|---|--------|------------------|
| 01 | lifecycle & structured intent | requirements/design/tasks contract, EARS gate, task metadata |
| 02 | context, knowledge & skills | typed context manifest v2, static lanes, portable skills, receipts |
| 03 | agent tool-driving & native guidance | truthful paths, scaffold, driver projection, host conformance |
| 04 | verification, evals & quality | evidence envelope, coverage gates, datasets/rubrics, quality packet |
| 05 | orchestration, multi-agent & routing | mission dispatch, worker lifecycle, recovery, routing/limits |
| 06 | security, permissions & governance | operating profiles, sandbox/secret isolation, authority packets |
| 07 | observability, cost & economics | run/telemetry envelope, cost brake, privacy, OTel/trace export |
| 08 | deployment & production assurance | delivery envelopes, release/deploy ledgers, canary/rollback, incidents |
| 09 | maintenance, modernization & operating model | successor links, provenance, drift, memory aging, org adoption |
| 10 | scope boundaries & interoperability | boundary invariant, adapter envelope, offline continuity, A2A/OTel maps |

**Optimization thesis.** The whole design optimizes **determinism + token economy**: gates, the
DAG, and reports are _pure functions of on-disk `.specd/` state_ — no LLM in any gate/DAG/report
path. Evidence integrity is absolute: a task completes **only** against a passing `specd verify`
record (exit 0 pinned to a real git HEAD); no bypass flag exists. Bias is subtractive (cut/defer
and record). This is what lets a small model drive a large process safely — the context window
never holds the rules; the binary does.

---

## 2. Health verification (measured, this session)

Every automated gate was re-run against a fresh `go build`:

| Gate | Result |
|------|--------|
| `go build -o specd .` | ✅ clean |
| `go test ./... -count=1` | ✅ **828 tests pass, 15 packages** |
| `gofmt -l .` (excl. `reference/`) | ✅ empty |
| `go vet ./...` | ✅ no issues |
| `go mod tidy` | ✅ no diff (no `go.sum` — zero deps) |
| `./scripts/docs-lint.sh` | ✅ exit 0 (CHEATSHEET ↔ command-reference in sync) |
| `./scripts/test-lint.sh` | ✅ exit 0 |
| `./scripts/regress-domains.sh` | ✅ all per-domain invariants hold |
| `./scripts/regress-lint.sh` | ✅ "clean — no smells" _(but see G2 — narrow scope)_ |
| `specs/*/tasks.md` task rows | ✅ **293/293 `[x]`**, 0 unchecked |

**Verdict: the product is healthy and the shipped code is real.** The 10-domain program is
functionally complete; nothing below contradicts that. The gaps are in the **regression-harness
and evidence-integrity _tooling_**, which is exactly the layer specd sells as its guarantee — so
they matter more than their small diff suggests.

---

## 3. Gap inventory

### G1 — `scripts/regress-all.sh` is a dead no-op (false green) · **HIGH**

The primary cross-wave regression harness — the one CLAUDE.md and `specs/progress.md` both name
as the post-wave safety net ("_Re-run `./scripts/regress-all.sh` after any wave flips to `[x]`_")
— **executes zero rows and always exits 0.** It is stale from a prior single-domain `P0…P7`
layout. Three independent mismatches, any one fatal:

1. **Wrong directory.** It globs `"$ROOT"/review-specs/0[0-6]-*/tasks.md`. There is no
   `review-specs/` — the specs live in `specs/`. The glob matches nothing → the loop never runs.
2. **Wrong domain range.** Even pointed at `specs/`, `0[0-6]` covers only domains 01–06 and
   misses 07, 08, 09, 10.
3. **Wrong row grammar.** It expects task IDs like `P0.1a` and verify commands wrapped in
   `` `backticks` ``. Real rows use `[x] T01` IDs with **plain** verify cells.

**Proof:** `scripts/regress-all.log` is **0 lines** after a run; exit code 0. The harness that is
supposed to re-execute every task's `verify:` line against a throwaway tree re-executes **none**.

**Blast radius.** `./scripts/regress-all.sh` is chained into the release-validator `verify:` line
of **every** domain (01 T26, 02 T24, 03 T21, 04 T22, 05 T24, 06 T31, 07 T32, 10 T17). Each of
those "full domain release evidence" tasks passed a no-op as one link in its `&&` chain. The
program's "done" claim rests on a gate that never ran. _(The other links in those chains — `go
test -race`, `vet`, `docs-lint`, `regress-domains` — are real and do pass, which is why the
product is still healthy. But the per-task re-run guarantee does not exist.)_

### G2 — `scripts/regress-lint.sh` covers only domains 02 and 04 · **MEDIUM**

The verify-table smell audit (its stated smell "A" catches verify lines that target the wrong
`specs/` vs `.specd/specs/` tree) hardcodes `specs/02-*/tasks.md` and one `specs/04-*/tasks.md`
path. It scans **2 of 10 domains** and reports "clean — no smells" while never looking at the
other 8. This is why G3 below survived undetected.

### G3 — Domain 10 verify cells use unescaped `|` · **MEDIUM**

Domain 10's `tasks.md` writes multi-pattern test selectors as `-run 'TestEnvelopeReject|TestExitClass'`
with a **raw pipe** inside a pipe-delimited markdown table. Domains 01–08 correctly escape these
as `\|` (e.g. `'TestRequirements\|TestEARS'`). Any table parser — `regress-all.sh`'s backtick-aware
`cell()` splitter, `regress-lint.sh`, or a docs renderer — mis-splits these cells mid-pattern.
The commands still work when a human runs them, but they **violate the table contract the
regression tooling depends on.** Latent because G1 (dead) and G2 (skips domain 10) both hide it.

### G4 — No `regress-*.sh` runs in CI or release automation · **MEDIUM**

`grep regress .github/workflows/*.yml` → **0 matches.** CI (`ci.yml`) and release (`release.yml`)
enforce `go test -race`, `go test -count=2`, `gofmt`, `vet`, `test-lint`, `docs-lint`, and
staticcheck — solid — but **none of the three regression harnesses.** So the domain-level
black-box invariants (`regress-domains`) and the per-task re-run (`regress-all`) are manual-only.
Nothing in automation catches a domain-invariant regression; the harnesses rely entirely on a
human remembering to run them, and G1 means one of them lies when they do.

### G5 — CLAUDE.md `go.sum` claim is inaccurate · **LOW**

CLAUDE.md: "_keep `go.mod`/`go.sum` tidy — CI runs `go mod tidy` and fails on a diff_." There is
no `go.sum` (zero deps); `git diff go.sum` errors "no such path in the working tree." Harmless
doc drift, but it's in the invariants section that's supposed to be authoritative.

---

## 4. Action plan

Ordered by leverage. G1 is the one that undermines the program's core claim; do it first.

### A1 — Rewrite `scripts/regress-all.sh` for the current layout _(fixes G1)_ · **HIGH**

Rebuild the harness to actually walk today's specs. Concretely:

- Glob `"$ROOT"/specs/[0-1][0-9]-*/tasks.md` (all ten domains).
- Parse rows matching `| [x] Tnn |`; extract the **verify** column (field 6 of the
  unescaped-pipe split) and un-escape `\|` → `|`.
- **Exclude the release-validator rows** whose verify contains `./scripts/regress-all.sh`
  (self-recursion) — the same guard the old script applied to its W7 wave.
- Run each unique verify with `sh -c` in the repo root, aggregate by exit code, write the log,
  exit non-zero iff any fails. (The existing `cell()`/`run_row()` scaffolding is reusable; only
  the glob, the ID regex, and the backtick assumption need replacing.)
- **Verify the fix by breaking it on purpose:** temporarily flip one task's verify to `false`,
  confirm the harness now exits non-zero and names that row. A regression harness with no failing
  case proving it can fail is just G1 again.

_Ponytail note:_ this is repair, not redesign — keep it a `sh` script, keep the aggregate-by-exit-code
contract, don't grow it into a framework.

### A2 — Wire the working harnesses into CI _(fixes G4, guards G1)_ · **MEDIUM**

Add a CI step (after the test job) running `./scripts/regress-domains.sh` and the repaired
`./scripts/regress-all.sh`. This makes the safety net real instead of aspirational and ensures A1
can never silently rot back to a no-op. Gate release on them too, alongside the existing suite.

### A3 — Generalize `regress-lint.sh` to all ten domains _(fixes G2)_ · **MEDIUM**

Replace the hardcoded `02`/`04` paths with a `for tasks in "$ROOT"/specs/[0-1][0-9]-*/tasks.md`
loop so the smell audit covers the whole program. Running it after A3 will immediately surface G3.

### A4 — Escape the pipes in domain 10 `tasks.md` _(fixes G3)_ · **LOW-MEDIUM**

Change every `-run 'TestA|TestB'` in `specs/10-*/tasks.md` to `-run 'TestA\|TestB'` to match the
table contract used by 01–08. Pure text edit; the commands' behavior is unchanged. After A3 this
becomes a lint-enforced invariant instead of a manual fix.

### A5 — Correct the `go.sum` line in CLAUDE.md _(fixes G5)_ · **LOW**

Drop the `go.sum` reference (or note "no `go.sum` — zero deps; `go mod tidy` keeps `go.mod`
minimal"). One-line doc edit.

---

## 5. Suggested sequence

```
A1 (repair regress-all)  →  A3 (widen regress-lint)  →  A4 (fix domain-10 pipes, now lint-caught)
                         →  A2 (CI-enforce both harnesses)  →  A5 (doc fix)
```

A1 first restores the program's headline guarantee. A3 before A4 so the lint _finds_ the pipe bug
rather than trusting this report. A2 last-but-important so none of the above can silently rot again.

**Bottom line:** the specd product is complete, tested, and green on every gate that actually runs.
The defect is that specd's own regression-tooling layer — the part that embodies its "evidence,
not judgment" promise — was left pointing at a spec layout that no longer exists, and CI never
noticed because it never ran it. Small diff, high symbolic and practical cost. Fix A1 + A2 and the
guarantee is real again.
