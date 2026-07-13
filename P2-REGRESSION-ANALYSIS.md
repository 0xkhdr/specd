# Stage P2 Regression Analysis — ecosystem and portfolio scale

_Generated 2026-07-13 · branch `sdlc-specs` · HEAD `b5050f0`_

Regression cycle over Stage P2 (`specs/progress.md` §"Stage P2 — ecosystem and
portfolio scale"): re-ran every gate, lint, and task-verify harness against a
freshly built binary, then audited the P2 code sitting in the working tree.

---

## 1. Health snapshot — all deterministic gates GREEN

| Check | Command | Result |
|-------|---------|--------|
| Build | `go build -o specd .` | ✅ clean, single static binary |
| Vet | `go vet ./...` | ✅ no issues |
| Format | `gofmt -l .` | ✅ empty |
| Deps | `go mod tidy` | ✅ zero runtime deps (no `go.sum`) |
| Unit/integration | `go test ./...` | ✅ **723 pass**, 15 packages |
| Test lint | `scripts/test-lint.sh` | ✅ ok |
| Docs sync | `scripts/docs-lint.sh` | ✅ CHEATSHEET mirrors command-reference |
| Verify-table smells | `scripts/regress-lint.sh` | ✅ clean — no smells |
| Per-domain invariants | `scripts/regress-domains.sh` | ✅ 01→10 hold |
| Every task verify | `scripts/regress-all.sh` | ✅ exit 0 |

**Coverage of P2 packages** (`go test -cover`):

| Package | Coverage |
|---------|----------|
| `internal/core/verify` | 88.3% |
| `internal/core/gates/security` | 85.8% |
| `internal/orchestration` | 81.8% |
| `internal/core` | 79.1% |
| `internal/cmd` | 77.0% |

Codebase: **21,341** non-test LOC vs **16,404** test LOC (≈0.77 test:code ratio).

**Verdict:** no failing gate, no invariant regression, no evidence-integrity
violation. The completed P2 waves are functionally sound.

---

## 2. New P2 code — invariant audit (PASS)

The uncommitted P2 body (04 W4/W5, 05 W5, 06 W8, 07 W7) adds 832 LOC across 7
new source files, each with a sibling `_test.go`:

```
231  internal/orchestration/a2a.go           (05 W5 — agent-to-agent adapter)
123  internal/core/gates/security/regress.go (06 W8 — security regression gate)
119  internal/core/eval_policy.go            (04 W4/W5 — eval aggregation/policy)
109  internal/core/evalset.go                (04 — eval manifest digests)
 95  internal/core/quality_ledger.go         (04 W5 — quality packet ledger)
 84  internal/core/verify/adapter.go         (06 — sandbox adapter validation)
 71  internal/cmd/eval.go                     (04 — `specd eval` verb)
```

Audited against the non-negotiable invariants (CLAUDE.md §"Non-negotiable
invariants"):

- **No bypass flag** — grep for `bypass|--force|skipverify|--no-verify` → none.
- **Determinism** — no `http.`/`net.`/`rand.`/`exec.Command` in any gate,
  policy, or ledger path.
- **No dangling work** — zero `TODO|FIXME|XXX|HACK` in the new files.
- **Wiring complete** — `eval` is registered in `registry.go`, declared in
  `commands.go`, documented in **both** `command-reference.md` and
  `CHEATSHEET.md`, and present in the MCP parity suite.
- **No duplication** — `verify/adapter.go` (sandbox), `eval_policy.go`
  (aggregation), `evalset.go` (manifest digest) have disjoint function
  surfaces; the two "adapter/manifest import" paths do not overlap.

This is clean, well-formed code. The problem is not the code — it is where it
lives (§3).

---

## 3. Findings

### F1 — CRITICAL: the entire confirmed P2 body is uncommitted

`progress.md` marks **04 W4, 04 W5, 05 W5, 06 W8, 07 W7** as `[x]` — meaning,
per the Definition of Done (item 5), the user reviewed and *confirmed* them.
But the git log stops at:

```
b5050f0 feat(03): add remote dispatch proof   ← latest commit
```

The working tree holds **17 untracked + 40 modified files**, none committed.
Every "done + confirmed" P2 wave from domains 04–07 exists **only in the
working tree**.

Impact:
- **Durability** — a `git checkout`/`clean`/disk loss destroys ~5 confirmed waves.
- **Traceability** — DoD item 1 requires each task backed by a verify record
  pinned to a **real git HEAD**; those HEADs were never committed. `regress-all`
  is currently asserting green against an *uncommitted* tree, not a real release.
- **Attribution/bisect** — no per-wave commit means no bisect target and no
  durable link between a `[x]` and the confirmation that earned it.

**Recommendation:** commit now, before any new wave. Prefer per-wave commits to
restore the `feat(0N): …` cadence the history already uses (`feat(04): W4 …`,
`feat(05): W5 …`, etc.). At minimum, one `feat(P2): catch-up 04–07` commit.
This is the single highest-value action in this report.

### F2 — MINOR: empty build log tracked in git

`scripts/regress-all.log` is git-tracked and **0 bytes** — a build artifact
checked into the tree. It will thrash on every regression run.

**Recommendation:** `git rm scripts/regress-all.log` and add `scripts/*.log`
to `.gitignore`.

### F3 — SCOPE: 18 P2 waves remain unimplemented

Remaining Stage P2 work (baseline waves exist; feature waves do not):

| Domain | Pending waves | Count | Notes |
|--------|---------------|-------|-------|
| 07 observability | W8, W9 | 2 | neutral event schema, attested ingestion |
| 08 deployment | W8, W9, W10, W11 | 4 | adapter, canary, CI binding, incident drills |
| 09 maintenance | W1–W11 | 11 | **baseline (W0) only** — dominant remaining block |
| 10 interop | W4, W5 | 2 | ecosystem mappings, release/feedback proof |

Domain 09 (`internal/core/memory.go`, `history.go`, fixtures) has its committed
W0 baseline and nothing above it. **11 of the 18 remaining waves are domain 09.**

### F4 — DoD item 5 is unauditable from git

Because waves aren't committed per-wave (F1), there is no durable record tying
each `[x]` to the user confirmation the DoD requires. Fixing F1 with per-wave
commits also fixes F4.

---

## 4. Critical-path recommendation for remaining P2

The `needs` edges in `progress.md` make ordering matter — several domain-08 and
all of domain-09 depend on 07 and 10:

```
10 W4 (ecosystem mappings)  ──┬─→ 10 W5 (needs 08)
07 W8 → 07 W9 (needs 10)    ──┤
                              ├─→ 08 W8..W11 (need 10 adapter, 07 measurement)
                              └─→ 09 W1..W11 (need 07 measurement, 10 boundary)
```

Suggested order:
1. **Commit the P2 body (F1)** — unblocks honest verify records for everything below.
2. **10 W4 → 07 W8 → 07 W9** — these unblock the most downstream waves.
3. **08 W8–W11** — deployment adapters/canary/CI/incident.
4. **09 W1–W11** — largest block; sequence per its own wave DAG.
5. **10 W5 last** — it needs 08.

---

## 5. Optimization opportunities (codebase)

Deliberately short — the codebase is in good shape; these are the only items worth touching:

1. **Restore per-wave commit cadence** (F1) — biggest lever, pure process.
2. **Untrack build logs** (F2) — one-line `.gitignore` fix.
3. **Keep adapter paths separate** — audited, no duplication to collapse. No action.
4. **No new abstractions warranted** — new files are small (71–231 LOC), single-purpose,
   fully tested. Nothing to refactor.

**Bottom line:** the P2 implementation is healthy and passes every gate; the
real risk is process, not code — an entire confirmed stage of work is one
`git clean` away from gone. Commit first, then proceed down the critical path.
