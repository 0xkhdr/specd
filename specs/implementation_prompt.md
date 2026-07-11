# implementation_prompt.md — standing instruction for the implementing agent

Paste this as the standing instruction for any coding agent (fresh context) that continues the
`specs/progress.md` build. It makes **every turn identical**: locate position → implement one
eligible wave completely under TDD + evidence gates → verify green → mark done → continue. When
the whole plan is done, tell the user to review.

Read these three files before your first edit: `specs/progress.md` (the map), `specs/prompt.md`
(the per-turn contract), and `CLAUDE.md` (the invariants). This file adds the **operating mode**
and the **hard-won gotchas** on top of `prompt.md` — where they overlap, both apply.

---

## 0. Operating mode (decided with the user)

- **Autonomous through the plan.** Do **not** stop for per-wave human review. Implement each
  eligible wave, verify it green (§3 below), then **self-mark it `[x]`** in both the spec's
  `tasks.md` and `specs/progress.md`, and move to the next eligible wave. Keep going across as
  many turns as it takes.
- **Do not commit** unless the user explicitly authorizes it for the session. Leave changes in
  the working tree. (If they do authorize: branch first if on `main`, name the wave + acceptance
  IDs in the message, end with the repo's `Co-Authored-By` trailer.)
- **Never leave a turn mid-wave.** A turn ends only at a wave boundary — implemented, green, and
  marked. If you must stop, stop at the last task whose `verify` passed, leave its row unchecked,
  and say exactly which task resumes next. Never end with a RED test and no implementation, a
  craftsman edit whose verify has not passed, or a failing lint/vet/format/regress gate.

## 1. Locate position (start of every turn)

1. Read `specs/progress.md` top-to-bottom for the first unchecked row.
2. Current **stage** = lowest of P0 → P1 → P2 with any unchecked row. Do stages in order.
3. A wave `W` in spec `S` is **eligible** when: every earlier wave in `S` is `[x]` in `S`'s
   `tasks.md`, **and** `W`'s `depends-on` evidence has passed, **and** every cross-domain "needs"
   note beside `W` in `progress.md` is already checked.
4. If a wave is mid-implementation from a prior turn, **resume it** before starting a new one.
   Otherwise pick the first eligible unchecked row in `progress.md` order.
5. If nothing is eligible, state the blocking cross-domain wave and stop.

State your target in one line before editing: `Turn target: <spec>/<wave> — <result>`.

## 2. Implement the wave completely (task by task, DAG order)

Process the wave's `tasks.md` rows respecting `depends-on`. Honor each row's `role`:

- **scout** (read-only): produce the inventory/finding the row names; record it in the spec, edit
  no product code. Its verify is `printf ok`.
- **craftsman** (write + verify), one atomic task per unit of work:
  1. Write/extend the failing test named in the row's `files` **first**; run it, watch it fail (RED).
  2. Edit only the declared `files`. If you truly need a file not listed, **record the deviation
     in that wave's `tasks.md` first** (see §4), then edit it.
  3. Run the row's `verify` until it passes at the current HEAD (GREEN) — but see gotcha (a).
  4. `gofmt -w` the touched Go files only.
- **validator** (read-only): run the row's verify on a freshly built binary; fix the craftsman
  task, never the test, to make it pass.
- **auditor** (read-only): audit the wave diff against the row's acceptance IDs.

Keep each change the smallest that satisfies the row's acceptance (`R<n>` IDs). Implement the
**entire wave**, not a slice.

## 3. Verify the wave (gate before marking done)

Run, in order, all green:

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

## 4. Mark done and continue

Once §3 is fully green:
1. Flip each completed row `[ ]` → `[x]` in the spec's `tasks.md`.
2. Check the matching row in `specs/progress.md`.
3. Append a short **cross-wave deviations** note under the wave's table for any file edited beyond
   the declared lists, any declared file you did **not** need (subtractive), and why.
4. Give a 3–6 line turn summary (wave, files, acceptance IDs, gate results), then start the next
   eligible wave — same turn if budget allows, otherwise next turn.

## 5. Non-negotiable invariants (never violate — from `CLAUDE.md` / `prompt.md` §0)

- **The agent reasons; the harness enforces.** No LLM / network / model call in any gate, DAG, or
  report path. Deterministic core only.
- **Evidence integrity.** A task completes only against a passing verify record (exit 0 pinned to
  a real git HEAD). There is no bypass flag — never add one.
- **Test before contract.** RED → GREEN, never GREEN-only.
- **Additive & compatible.** Old `.specd/` state, task files, docs, and CLI output must load or
  migrate with an actionable error — never silent reinterpretation. **Zero new runtime deps**
  (`go mod tidy` stays clean). Never edit `reference/` (frozen v1 museum).
- **Subtractive bias.** When unsure, cut or defer and record the decision in the wave's `tasks.md`.
- **Byte-stable parsers.** Markdown/tasks parsers round-trip without reformatting author bytes.

## 6. Gotchas learned in flight (save yourself the debugging)

a. **Verify lines use `\|`, which RE2 reads as a literal pipe.** `go test -run 'TestA\|TestB'`
   therefore matches **nothing** and exits 0 vacuously. Some rows also name a test that does not
   exist yet. So: **do not trust a row's verify to prove your work.** Run the intended tests with
   real alternation (`-run 'TestA|TestB'`) and make them genuinely pass. Write the named test if
   it is missing. (A future cleanup wave should `s/\\|/|/` the verify cells and add a lint; until
   then, treat the row's verify as a naming hint, not a gate.)

b. **New strictness must be opt-in / profile-gated.** Many requirements read "the system shall
   require …", but the domain designs say new policy is `production`-profile-gated and `default`
   stays backward compatible (R7.1-style). Pattern that worked: **always** refuse genuine defects
   (unknown reference, bad enum value); require full completeness only when an opt-in
   `CheckCtx` flag / config profile is armed. Default `CheckCtx` (zero value) must stay clean so
   existing tests and legacy artifacts keep passing.

c. **Registering a new gate ripples.** `internal/core/gates/core.go`'s `registry.Register(...)`
   count is pinned by (i) `docs-lint.sh`'s "N core gates" claim across ~4 docs
   (`README.md`, `docs/contributor-guide.md`, `docs/README.md`, `docs/validation-gates.md`) and
   (ii) the expected slice in `internal/core/gates/registry_test.go` (`TestRegistryOrder`). Update
   all of them in the same wave, and add the gate's row to `docs/validation-gates.md`.

d. **Header-indexed table columns are extensible.** The tasks parser looks columns up by header
   name via `headerIndex` (returns -1 when absent → `cell` yields ""). Add optional columns by
   name inside `ParseTasksMd`'s visit closure — no `md.go` edit, fully backward compatible.

e. **Approval records should pin a source digest.** `runApprove` pins `core.Digest(bytes)` of the
   approved artifact into the record — needed later for amendment/staleness (R5). Follow that
   pattern for any new approved artifact.

f. **MCP is derived from `core.Commands`.** Adding a flag to a command's metadata auto-exposes it
   as an MCP tool property (`internal/mcp/tools_core.go`) — you rarely hand-write MCP surface.

g. **Reading files in this environment:** a token-optimizing proxy may compress large tool
   outputs into a `[N items compressed … hash=…]` marker. Read **narrow line ranges** (small
   `offset`/`limit`) to avoid it, or retrieve the full content by its `hash`. Prefer the dedicated
   Read/Edit/Grep tools over `cat`/`sed`.

## 7. Definition of done — and the termination signal

Apply `specs/progress.md`'s "Definition of done" per wave and per domain: a domain is done when
its final validator task is green against a fresh release binary and its README completion claim
is demonstrated. **The plan is done when every row in `specs/progress.md` (all stages P0, P1, P2,
across all ten domains) is `[x]`** and the full §3 gate suite is green on a fresh build.

When — and only when — that is true, do **not** start another wave. Instead reply to the user
with exactly this outcome:

> **No more turns needed.** All waves in `specs/progress.md` (P0 → P1 → P2, domains 01–10) are
> implemented, verified, and marked done. The full gate suite is green on a fresh build. It is
> time to review the specs implementation.

Then summarize (per stage/domain: waves completed, notable deviations, and anything the user should
scrutinize), and stop.
