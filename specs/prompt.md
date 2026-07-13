# prompt.md — single-wave implementation instruction

Paste this as the standing instruction for any coding agent (fresh context) implementing the
`specs/progress.md` build. It makes every dispatch identical: coordinator locates the next eligible
wave → dispatches exactly one subagent → subagent implements exactly one wave completely,
task-by-task, under TDD + evidence gates → verifies green → reviews, marks, and commits the wave →
returns control. Coordinator then selects the next wave without waiting for user approval.

`specs/progress.md` is the map; each `specs/0N-.../` is the territory; `CLAUDE.md` holds the
invariants. Read all three before your first edit.

**One subagent, one wave, one commit.** Coordinator may have only one implementation subagent
active. Subagent implements one stage wave — e.g. `09 W1 — 09b-successor-link-kinds` — and must
verify, mark, and commit it before returning. Only after coordinator confirms that commit may it
dispatch a subagent for next eligible wave. Never batch waves or run implementation subagents in
parallel.

---

## 0. Non-negotiables (never violate)

- **The agent reasons; the harness enforces.** No LLM/network/model call in any gate, DAG, or
  report path. Deterministic core only. (`CLAUDE.md` invariants.)
- **Evidence integrity.** A task completes only against a passing `verify` record — exit 0 pinned
  to a real git HEAD. There is no bypass flag; do not add one.
- **Test before contract.** Write the failing black-box/conformance/unit test *before* the public
  contract code it pins. RED → GREEN, never GREEN-only.
- **Additive & compatible.** Old `.specd/` state, task files, and CLI output load/migrate or fail
  with an actionable migration error — never silent reinterpretation. Zero new runtime deps
  (`go mod tidy` must stay clean). Never edit `reference/` (frozen v1 museum).
- **Subtractive bias.** When unsure, cut or defer and record the decision in the spec's tasks.
- **Committed completion.** Never flip a `tasks.md` checkbox or a `progress.md` row to `[x]` until
  wave verification is green. Never report wave complete or dispatch next subagent until current
  wave is committed.

## 1. Coordinator: locate and dispatch

1. Read `specs/progress.md` top to bottom for the first unchecked row.
2. Determine the **current stage** (P0 → P1 → P2): the lowest stage with any unchecked row.
3. Find the **one eligible wave** — a wave `W` in spec `S` is eligible when:
   - every earlier wave in `S` is `[x]` in `S`'s `tasks.md`, **and**
   - `W`'s local `depends-on` task evidence has passed, **and**
   - every cross-domain "needs" note next to `W` in `progress.md` is already checked.
4. If a wave is mid-implementation (some rows `[x]`, some not) from a prior turn, resume **that**
   wave. Otherwise pick the first eligible unchecked row in `progress.md`'s order.
5. If nothing is eligible, state the blocker (which cross-domain wave it needs) and stop.

State chosen wave in one sentence: `Dispatch target: <spec>/<wave> — <result>`. Pass that wave,
its domain spec, `specs/progress.md`, this protocol, and relevant repository instructions to one
subagent. Do not edit product code in coordinator while subagent runs. Do not dispatch any other
implementation subagent until current one returns with a verified commit. Ignore every later wave
until then.

## 2. Implement the wave completely (task by task, in DAG order)

Process the wave's `tasks.md` rows respecting `depends-on`, applying software engineering best
practices throughout (small diffs, no speculative abstraction, no dead code, idiomatic Go). For
each row honor its `role`:

- **scout** (read-only): produce the inventory/finding the row names; its `verify` is `printf ok`.
  Record findings in the spec, do not edit product code.
- **craftsman** (write + verify): exactly one atomic task per unit of work —
  1. Write/extend the failing test named in the row's `files` first; run it, watch it fail (RED).
  2. Edit only the declared `files`. A needed file not listed → record the deviation in `tasks.md`
     before editing (cross-wave rule), then proceed.
  3. Run the row's `verify` command until it passes at the current HEAD (GREEN) — but see gotcha (a).
  4. Run `gofmt -w` on touched Go files only.
- **validator** (read-only): run the row's `verify` on a freshly built binary; fix the craftsman
  task, never the test, to make it pass.
- **auditor** (read-only, if present): audit the wave diff against the row's acceptance IDs.

Keep the change the smallest that satisfies the row's `acceptance` (the mapped `R<n>` IDs).
Implement the **entire wave**, not a partial slice — but **only** this wave.

## 3. Verify the wave (gate before requesting review)

Run, in order, and require all green:

```bash
go build -o specd .
go test ./... -race -count=1
go test ./... -count=2            # catch iteration-order flakiness
gofmt -l .                       # must be empty
go vet ./...
go mod tidy && git diff --exit-code go.mod   # (no go.sum exists — zero deps)
./scripts/test-lint.sh
./scripts/docs-lint.sh           # if any CLI verb/flag or doc changed
./scripts/regress-domains.sh
./scripts/regress-all.sh         # confirm no earlier wave regressed (exit 0)
```

If the wave touched CLI verbs/flags, `docs/command-reference.md` and `docs/CHEATSHEET.md` must be
updated **together** — `docs-lint.sh` requires them byte-identical (`cmp -s`).

## 4. Review, mark, and commit before returning

Once §3 is fully green, assigned subagent must:

1. Review its own diff against declared files, acceptance IDs, repository invariants, and best
   practices. Fix findings and rerun §3 before continuing.
2. Flip completed rows `[ ]` → `[x]` in domain `tasks.md` and check matching row in
   `specs/progress.md`.
3. Commit exactly this wave, naming wave and acceptance IDs, e.g.
   `09b: successor link kinds (R2.1,R2.2)`, with repository-required trailer. Do not push unless
   user separately requested pushing.
4. Confirm commit exists and working tree contains no uncommitted wave changes.
5. Return coordinator a 3–6 line report: wave, files, acceptance IDs, gate results, commit hash.

Coordinator must validate returned commit and checklist state. If incomplete, send same subagent
back to finish; do not dispatch another. When complete, coordinator may return to §1 and dispatch
next eligible wave automatically. User approval is not a wave gate.

## 5. Leave nothing half-done (turn boundary rule)

A subagent dispatch ends only at a committed wave boundary: implemented, verified green, reviewed,
marked, and committed. Never end a dispatch with:

- a written test that is still RED and no implementation,
- a craftsman edit whose `verify` has not passed,
- a failing lint/vet/format/regress gate,
- boxes checked before verification is green,
- an uncommitted mix of finished and unfinished work,
- **the next wave started or another implementation subagent dispatched before current wave is
  committed.**

If subagent is blocked mid-wave, it stops at last task whose `verify` passed, leaves row unchecked,
and reports exact blocker and resume task. Coordinator must resume same wave with same subagent (or
explicit replacement after it has stopped); it may not skip ahead.

## 6. Gotchas learned in flight (save yourself the debugging)

a. **Verify lines use `\|`, which RE2 reads as a literal pipe.** `go test -run 'TestA\|TestB'`
   therefore matches **nothing** and exits 0 vacuously. Some rows also name a test that does not
   exist yet. So: **do not trust a row's verify to prove your work.** Run the intended tests with
   real alternation (`-run 'TestA|TestB'`) and make them genuinely pass. Write the named test if
   it is missing.

b. **New strictness must be opt-in / profile-gated.** Many requirements read "the system shall
   require …", but the domain designs say new policy is `production`-profile-gated and `default`
   stays backward compatible. Always refuse genuine defects (unknown reference, bad enum value);
   require full completeness only when an opt-in `CheckCtx` flag / config profile is armed. Default
   `CheckCtx` (zero value) must stay clean so existing tests and legacy artifacts keep passing.

c. **Registering a new gate ripples.** `internal/core/gates/core.go`'s `registry.Register(...)`
   count is pinned by (i) `docs-lint.sh`'s "N core gates" claim across ~4 docs
   (`README.md`, `docs/contributor-guide.md`, `docs/README.md`, `docs/validation-gates.md`) and
   (ii) the expected slice in `internal/core/gates/registry_test.go` (`TestRegistryOrder`). Update
   all of them in the same wave, and add the gate's row to `docs/validation-gates.md`.

d. **Header-indexed table columns are extensible.** The tasks parser looks columns up by header
   name via `headerIndex` (returns -1 when absent → `cell` yields ""). Add optional columns by
   name inside `ParseTasksMd`'s visit closure — no `md.go` edit, fully backward compatible.

e. **Approval records should pin a source digest.** `runApprove` pins `core.Digest(bytes)` of the
   approved artifact into the record. Follow that pattern for any new approved artifact.

f. **MCP is derived from `core.Commands`.** Adding a flag to a command's metadata auto-exposes it
   as an MCP tool property (`internal/mcp/tools_core.go`) — you rarely hand-write MCP surface.

g. **Reading files in this environment:** a token-optimizing proxy may compress large tool outputs
   into a `[N items compressed … hash=…]` marker. Read **narrow line ranges** (small
   `offset`/`limit`) to avoid it. Prefer the dedicated Read/Edit/Grep tools over `cat`/`sed`.

## 7. Definition of done

Per wave and per domain, apply `specs/progress.md` § Definition of done verbatim. A wave is done
only when its rows are all `[x]` (each backed by a passing verify record), its validator command is
green on a fresh binary, the full §3 gate suite is green, and the wave is committed. A domain is
done when its final validator task is green against a fresh release binary and its README completion
claim is demonstrated. The program is done when all ten domains are done — reached **one subagent,
one verified wave, one commit at a time**.
