# prompt.md — program implementation instruction

Paste this as the standing instruction for any coding agent implementing the program. It makes
**every turn identical**: locate position → pick one eligible wave → implement it completely,
task-by-task, under evidence gates and best practices → **stop and ask the user to review** →
mark done only after the user confirms. `progress.md` is the map, each `specs/0N-.../` is the
territory.

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
- **No unreviewed completion.** Never flip a `tasks.md` checkbox or a `progress.md` row to `[x]`
  without the user's explicit confirmation for that wave.

## 1. Locate position (start of every turn)

1. Read `specs/progress.md` top to bottom for the first unchecked row.
2. Determine the **current stage** (P0 → P1 → P2): the lowest stage with any unchecked row.
3. Find **eligible waves** — a wave `W` in spec `S` is eligible when:
   - every earlier wave in `S` is `[x]` in `S`'s `tasks.md`, **and**
   - `W`'s local `depends-on` task evidence has passed, **and**
   - every cross-domain "needs" note next to `W` in `progress.md` is already checked.
4. If a wave is mid-implementation (some rows `[x]`, some not) from a prior turn, resume it before
   starting a new one. Otherwise pick the first eligible unchecked row in `progress.md`'s order.
5. If nothing is eligible, state the blocker (which cross-domain wave it needs) and stop. Do not
   start out-of-order work.

State your chosen wave in one sentence before editing: `Turn target: <spec>/<wave> — <result>`.

## 2. Implement the wave completely (task by task, in DAG order)

Process the wave's `tasks.md` rows respecting `depends-on`, applying software engineering best
practices throughout (small diffs, no speculative abstraction, no dead code, idiomatic Go). For
each row honor its `role`:

- **scout** (read-only): produce the inventory/finding the row names; its `verify` is `printf ok`.
  Record findings in the spec, do not edit product code.
- **craftsman** (write + verify): exactly one atomic task per unit of work —
  1. Write/extend the failing test named in the row's `files` first; run it, watch it fail.
  2. Edit only the declared `files`. A needed file not listed → record the deviation in `tasks.md`
     before editing (cross-wave rule), then proceed.
  3. Run the row's exact `verify` command until it passes at the current HEAD.
  4. Run `gofmt -w` on touched Go files only.
- **validator** (read-only): run the row's `verify` command; it must pass on a freshly built
  binary. Do not edit product code to make it pass — fix the craftsman task instead.
- **auditor** (read-only, if present): audit the wave diff against the row's acceptance IDs.

Keep the change the smallest that satisfies the row's `acceptance` (the mapped `R<n>` IDs).
Implement the **entire wave**, not a partial slice of it — do not stop mid-wave unless blocked.

## 3. Verify the wave (gate before requesting review)

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

## 4. Ask the user to review — do not self-mark done

Once §3 is fully green:

1. Summarize what the wave implemented, which files changed, and which acceptance IDs it covers.
2. Present it to the user and ask them to review the implementation against best practices.
3. **Wait for explicit confirmation.** Do not check any box in `tasks.md` or `progress.md` yet.
4. If the user requests changes, apply them, re-run §3, and ask again.
5. Only after the user confirms:
   - Flip each completed row `[ ]` → `[x]` in the spec's `tasks.md`.
   - Check the matching row in `specs/progress.md`.
   - Commit with a message naming the wave and its acceptance IDs, e.g.
     `10c: versioned adapter envelope (R2.1,R2.3)`, ending with the repo's required
     `Co-Authored-By` trailer. Branch first if on `main`. Only commit/push if the user has
     authorized it for this session.

## 5. Leave nothing half-done (turn boundary rule)

A turn ends **only** at a wave boundary — implemented, verified green, and either (a) awaiting the
user's review, or (b) confirmed and marked done. Never end a turn with:

- a written test that is still RED and no implementation,
- a craftsman edit whose `verify` has not passed,
- a failing lint/vet/format/regress gate,
- boxes checked without the user having actually confirmed the wave,
- an uncommitted mix of finished and unfinished work.

If you must stop mid-wave, stop at the last task whose `verify` passed, leave its row unchecked,
and note in your turn summary exactly which task to resume next.

## 6. Definition of done

Per wave and per domain, apply `specs/progress.md` § Definition of done verbatim. A domain is done
when its final validator task is green against a fresh release binary, its README completion claim
is demonstrated, and the user has confirmed it. The program is done when all ten domains are done.
