# prompt.md — single-wave implementation instruction

Paste this as the standing instruction for any coding agent (fresh context) implementing the
`specs/progress.md` build. It makes **every turn identical**: locate position → implement **exactly
one** eligible wave completely, task-by-task, under TDD + evidence gates → verify green → **stop and
ask the user to review and commit** → mark done only after the user confirms. Then stop; the next
wave is a new turn.

`specs/progress.md` is the map; each `specs/0N-.../` is the territory; `CLAUDE.md` holds the
invariants. Read all three before your first edit.

**One wave per turn.** Implement a single stage wave — e.g. `09 W0 — 09a-maintenance-baseline` —
present it, and wait for the user to review and commit **before** you touch the next wave
(e.g. `10 W0 — baseline and boundary invariant`). Do not batch waves. Do not self-advance.

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
- **No unreviewed completion.** Never flip a `tasks.md` checkbox or a `progress.md` row to `[x]`
  without the user's explicit confirmation for that one wave.

## 1. Locate position (start of every turn)

1. Read `specs/progress.md` top to bottom for the first unchecked row.
2. Determine the **current stage** (P0 → P1 → P2): the lowest stage with any unchecked row.
3. Find the **one eligible wave** — a wave `W` in spec `S` is eligible when:
   - every earlier wave in `S` is `[x]` in `S`'s `tasks.md`, **and**
   - `W`'s local `depends-on` task evidence has passed, **and**
   - every cross-domain "needs" note next to `W` in `progress.md` is already checked.
4. If a wave is mid-implementation (some rows `[x]`, some not) from a prior turn, resume **that**
   wave. Otherwise pick the first eligible unchecked row in `progress.md`'s order.
5. If nothing is eligible, state the blocker (which cross-domain wave it needs) and stop.

State your chosen wave in one sentence before editing: `Turn target: <spec>/<wave> — <result>`.
Pick exactly one wave. Ignore every later wave until this one is confirmed and committed.

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

## 4. Stop, ask the user to review and commit — do not self-advance

Once §3 is fully green:

1. Summarize what the wave implemented, which files changed, and which acceptance IDs it covers
   (3–6 lines: wave, files, acceptance IDs, gate results).
2. Present it to the user and ask them to **review the implementation** against best practices.
3. **Wait for explicit confirmation.** Do not check any box in `tasks.md` or `progress.md` yet,
   and do not begin the next wave.
4. If the user requests changes, apply them, re-run §3, and ask again.
5. Only after the user confirms:
   - Flip each completed row `[ ]` → `[x]` in the spec's `tasks.md`.
   - Check the matching row in `specs/progress.md`.
   - **Commit** this wave (branch first if on `main`), naming the wave and its acceptance IDs,
     e.g. `09a: maintenance baseline (R1.1,R1.2)`, ending with the repo's required
     `Co-Authored-By` trailer. Only commit/push if the user has authorized it for this session.
6. **Then stop and end the turn.** The next eligible wave is a fresh turn — return to §1.

## 5. Leave nothing half-done (turn boundary rule)

A turn ends **only** at a wave boundary — implemented, verified green, and either (a) awaiting the
user's review, or (b) confirmed, marked, and committed. Never end a turn with:

- a written test that is still RED and no implementation,
- a craftsman edit whose `verify` has not passed,
- a failing lint/vet/format/regress gate,
- boxes checked without the user having actually confirmed the wave,
- an uncommitted mix of finished and unfinished work,
- **the next wave already started before the current one was confirmed and committed.**

If you must stop mid-wave, stop at the last task whose `verify` passed, leave its row unchecked,
and note in your turn summary exactly which task to resume next.

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
green on a fresh binary, the full §3 gate suite is green, the user has confirmed, and the wave is
committed. A domain is done when its final validator task is green against a fresh release binary,
its README completion claim is demonstrated, and the user has confirmed. The program is done when
all ten domains are done — reached **one confirmed, committed wave at a time**.
