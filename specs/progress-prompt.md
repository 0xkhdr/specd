# Program implementation prompt

Paste this as the standing instruction for any coding agent implementing the Google SDLC alignment
program. It makes **every turn identical**: locate position → pick one eligible wave → implement it
task-by-task under evidence gates → mark progress → leave nothing half-done. Follow it exactly;
`progress-plan.md` is the map, each `specs/0N-.../` is the territory.

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
  (`go mod tidy` must stay clean). Never edit `reference/`.
- **Subtractive bias.** When unsure, cut or defer and record the decision in the spec's tasks.

## 1. Locate position (start of every turn)

1. Read `specs/progress-plan.md` §5 (ledger) and §3 (build order).
2. Determine the **current stage** (P0 → P1 → P2): the lowest stage with any `todo`/`wip` cell.
3. Find **eligible waves** — a wave `W` in spec `S` is eligible when:
   - every earlier wave in `S` is `done` (its `tasks.md` rows are `[x]`), **and**
   - `W`'s local `depends-on` task evidence has passed, **and**
   - every **cross-domain "Requires"** listed for `W` in `S`'s README is `done` in the ledger.
4. If a `wip` wave exists, **resume it** — finishing an in-flight wave always precedes starting a
   new one (see §5). If none, pick the eligible wave earliest in the §3 build order; break ties by
   lowest domain number, then earliest wave letter.
5. If nothing is eligible, record the blocker in §5's blocked-wave log with the exact
   cross-domain prereq, and stop with that finding. Do not start out-of-order work.

State your chosen wave in one sentence before editing: `Turn target: <spec>/<wave> — <result>`.

## 2. Implement the wave (task by task, in DAG order)

Process the wave's `tasks.md` rows respecting `depends-on`. For each row honor its `role`:

- **scout** (read-only): produce the inventory/finding the row names; its `verify` is `printf ok`.
  Record findings in the spec (e.g., baseline inventory), do not edit product code.
- **craftsman** (write + verify): **exactly one atomic task per unit of work** —
  1. Write/extend the failing test named in the row's `files` first; run it, watch it fail.
  2. Edit only the declared `files`. A needed file not listed → record the deviation in `tasks.md`
     before editing (cross-wave rule), then proceed.
  3. Run the row's exact `verify` command until it passes at the current HEAD.
  4. Run `gofmt -w` on touched Go files only.
- **validator** (read-only): run the row's `verify` command; it must pass on a freshly built
  binary. Do not edit product code to make it pass — fix the craftsman task instead.
- **auditor** (read-only, if present): audit the wave diff against the row's acceptance IDs.

Keep the change the smallest that satisfies the row's `acceptance` (the mapped `R<n>` IDs). Do not
implement future waves' scope.

## 3. Verify the wave (gate before marking done)

Run, in order, and require all green:

```bash
go build -o specd .
go test ./... -race -count=1
go test ./... -count=2            # catch iteration-order flakiness
gofmt -l .                        # must be empty
go vet ./...
go mod tidy                       # then: git diff --exit-code go.mod go.sum
./scripts/test-lint.sh
./scripts/docs-lint.sh            # if any CLI verb/flag or doc changed
./scripts/regress-domains.sh
./scripts/regress-all.sh          # confirm no earlier wave regressed
```

If the wave touched CLI verbs/flags, `docs/command-reference.md` and `docs/CHEATSHEET.md` must have
been updated together (docs-lint enforces byte-identical mirroring).

## 4. Mark progress (same change, atomically)

Only after §3 is fully green:

1. Flip each completed row `[ ]` → `[x]` in the spec's `tasks.md`.
2. If the wave's validator row passed, update the domain's stage cell in `progress-plan.md` §5
   (`wip` → `done`, or `todo` → `wip` if the wave is mid-stage). Clear any matching blocked-wave
   log entry.
3. Commit with a message naming the wave and its acceptance IDs, e.g.
   `10c: versioned adapter envelope (R2.1,R2.3)`. End the commit body with the repo's required
   `Co-Authored-By` trailer. Branch first if on `main`.

The checkboxes in `tasks.md` are the fine-grained truth; the ledger is the rollup. They must agree
at the end of every turn.

## 5. Leave nothing half-done (turn boundary rule)

A turn ends **only** at a wave boundary or a clean task boundary where §3 is fully green and §4 is
recorded. Never end a turn with:

- a written test that is still RED and no implementation,
- a craftsman edit whose `verify` has not passed,
- `tasks.md` checkboxes or the ledger out of sync with reality,
- a failing lint/vet/format/regress gate,
- an uncommitted mix of finished and unfinished work.

If you must stop mid-wave, stop at the last task whose `verify` passed, mark exactly those rows
`[x]`, set the domain cell to `wip`, and note the next task id in the blocked-wave log so the next
turn resumes deterministically. The next turn's §1 will resume that `wip` wave before anything else.

## 6. Definition of done

Per wave and per domain, apply `progress-plan.md` §6 verbatim. A domain is `done` when its final
validator task is green against a fresh release binary and its README completion claim is
demonstrated. The program is `done` when all ten domains are `done` and the L0–L7 ladder in
`progress-plan.md` §4 is satisfied end-to-end.
