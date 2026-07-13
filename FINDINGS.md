# specd — Implementation Findings & Residual Items

_Generated 2026-07-13 on branch `sdlc-specs`, after implementing the ANALYSIS.md
action plan. This file records (1) where the original analysis was **wrong** and
had to be corrected before implementing, (2) what was actually changed, and (3)
the residual low-severity items that remain._

---

## 1. Correction: ANALYSIS.md misdiagnosed the pipe convention (critical)

ANALYSIS.md §G3/§A4 claimed that domains 01–08 "correctly escape" multi-pattern
test selectors as `-run 'TestA\|TestB'`, and that domain 10's raw `-run 'TestA|TestB'`
was the bug to fix by escaping it. **This is backwards**, and applying A4 as written
would have silently broken working evidence. Proven empirically:

```
$ go test ./internal/core/gates -run 'TestEARS\|TestNothingXYZ'   # ESCAPED form
Go test: No tests found        exit 0    ← VACUOUS: runs zero tests, passes anyway

$ go test ./internal/core/gates -run 'TestEARS|TestNothingXYZ'    # RAW form
Go test: 2 passed in 1 packages  exit 0  ← actually runs the tests
```

In Go's regexp, `\|` is an **escaped literal pipe**, so `-run 'TestA\|TestB'` looks
for a single test literally named `TestA|TestB`, finds none, and `go test` exits 0
("no tests to run" is a pass). The `\|` form is therefore a **hollow verify** — the
exact evidence-integrity failure specd exists to prevent.

Scope of the real defect (measured across all 293 completed rows):

| Form | Rows | Behavior | Markdown render |
|------|------|----------|-----------------|
| `\|` escaped | 15 (all domain 01) | **runs 0 tests** (vacuous pass) | renders OK |
| raw `\|` | 130 (domains 02–07, 10) | tests actually run | **breaks GFM table** (cell splits mid-command) |
| single-pattern / already backtick-wrapped | 148 | fine | fine |

Both non-trivial forms are half-broken. The only convention correct on **both**
axes is a **backtick code span containing a raw pipe** — which is exactly what the
original `regress-all.sh` `cell()` splitter and `regress-lint.sh` extractor were
already written to expect. The rows had drifted away from it.

**Verification that the 15 vacuous rows hid no real failure:** every one of the 15
domain-01 verify commands was re-run with a real (raw) pipe; all 15 pass. The
underlying code was always correct — only the recorded evidence was empty. Fixing
`\|` → `|` restores real evidence with **zero code changes**.

---

## 2. What was implemented

Ordered as executed. All gate commands are green after these changes.

### C1 — Normalized the verify-table pipe convention (145 rows)
Converted every pipe-bearing verify cell to a **backtick code span with a raw pipe**:
- 15 domain-01 `\|` cells → `` `... 'TestA|TestB'` `` (fixes the vacuous-verify bug).
- 130 domains 02–07/10 raw-pipe cells → wrapped in a code span (fixes GFM rendering
  **and** gives the harness a single unambiguous field to parse).
- Domains 08/09 were already conformant and were left untouched.

Row-for-row edit (145 insertions / 145 deletions); requirement columns preserved;
`[x]` count unchanged at 293.

### C2 — Rewrote `scripts/regress-all.sh` (fixes G1 — dead no-op)
- Globs `specs/[0-1][0-9]-*/tasks.md` (all ten domains) instead of the nonexistent
  `review-specs/0[0-6]-*/`.
- Parses `| [x] Tnn |` rows; extracts field-6 verify with a backtick- **and**
  escape-aware splitter; runs the first code-span (annotations like ` (GREEN)` may
  follow) or the whole cell.
- Skips self-recursive release-validator rows (verify contains `regress-all.sh`).
- **Fails closed on zero executed rows.** The precise way this harness previously
  lied — matching nothing and exiting 0 — is now itself a hard failure. Proven by
  breaking a verify to `false` and confirming a non-zero exit that names the row.

### C3 — Rewrote `scripts/regress-lint.sh` (fixes G2 — 2-of-10 coverage)
Now loops **all ten domains** and audits the (well-formed) verify command with
smells A (authoring `specs/`), B (hollow), C (stale verify path), D (compile-only),
plus two new ones that would have caught this whole class:
- **E — vacuous `\|` selector** (the bug ANALYSIS.md called "correct").
- **F — unescaped raw pipe** outside a code span (row splits into ≠ 8 table fields).

Both new smells were proven to fire on injected bad rows. The free-form `files:`
column is intentionally **not** scanned for existence (it carries prose and varies
per domain — the source of false positives); the explicit release-proof tripwires
are retained.

### C4 — Wired the harnesses into automation (fixes G4)
- New `regression` job in `.github/workflows/ci.yml` running `regress-lint`,
  `regress-domains`, and `regress-all` on every PR/push.
- Release gate in `.github/workflows/release.yml` extended with the same three.
So the safety net is enforced, not aspirational, and C2/C3 cannot silently rot back
to a no-op.

### C5 — Fixed a genuinely broken verify surfaced by C2 (relates to G5)
Domain-08 **T43** verify `go mod tidy && git diff --exit-code go.mod go.sum` exited
128 because there is no `go.sum` (`git diff` without `--` treats it as a bad path).
Added the `--` pathspec separator to match CI (`git diff --exit-code -- go.mod go.sum`),
which tolerates the absent file. This latent break was invisible while G1 kept the
harness dead.

### C6 — Corrected the `go.sum` doc claim (fixes G5)
`CLAUDE.md` and `docs/contributor-guide.md` said "keep `go.mod`/`go.sum` tidy";
there is no `go.sum` (zero deps). Reworded to "there is no `go.sum` — nothing to
sum; CI runs `go mod tidy` and fails on any `go.mod` diff."

---

## 3. Residual items (low severity, deliberately not auto-fixed)

### R1 — Nine stale **file-column** declarations in completed rows · **LOW**
Generalizing the stale-target audit surfaced nine `[x]` rows whose `files:` column
names a source file that has since been renamed or split. These are **not** evidence
defects — the verify commands run and pass; the code exists under different names —
so the lint intentionally does **not** flag the prose-heavy `files:` column (doing so
produced false positives on legitimately-renamed files across domains 08/09). Left as
optional documentation cleanup; the accurate mappings are:

| Row | Declared (stale) | Now lives in |
|-----|------------------|--------------|
| 01-T12 | `internal/core/dag_test.go` | `internal/core/tasksparser_test.go` (`TestDAG`) |
| 01-T17 | `internal/core/amendment_test.go` | `internal/core/state_test.go` (`TestAmendment`) |
| 03-T07 | `internal/core/mcpconfig_test.go` | `internal/cmd/integration_polish_test.go` (`TestMCPConfig`) |
| 05-T09/T10/T11 | `internal/cmd/brain.go` | `internal/cmd/brain_{claim,run,report,worker,heartbeat}.go` |
| 09-T24 | `roles.go/scaffold.go` (malformed token) | `internal/core/roles.go`, `internal/core/scaffold.go` |

To make this a lint-enforced invariant one would first need to standardize the
`files:` column into a machine-parseable path list (separate from its prose), which
is a larger, separate change than the tooling repair above — recorded here rather
than smuggled in.

### R2 — `regress-all` is serial and slow · **LOW**
Re-running ~285 verifies sequentially takes minutes and several rows invoke
`regress-domains.sh` (a full fresh-tree build) inside their own chain, so CI does
redundant builds. It is correct and hermetic; parallelizing is a future optimization,
not a defect. (During development, running it concurrently with other `go test`
invocations caused spurious SIGKILL/`rc=152` failures from CPU/disk oversubscription
— it must run without competing heavy load, which CI's dedicated job provides.)

---

## 4. Verification status

| Gate | Result |
|------|--------|
| `go build`, `go test ./...`, `gofmt -l`, `go vet` | green (no Go source changed) |
| `shellcheck -S error` on both rewritten scripts | clean |
| `./scripts/regress-lint.sh` (all 10 domains) | clean — no smells |
| `./scripts/regress-domains.sh` | all invariants hold |
| `./scripts/regress-all.sh` | executes 285 rows, all pass _(see run log)_ |
| E/F smell fire-on-break | proven |
| regress-all fail-on-break | proven |
