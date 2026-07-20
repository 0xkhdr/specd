# Workflow feedback log

Append-only log of friction hit while *using* specd to build specs. Input for
improving the harness so agents work with specd as native knowledge.

Do not delete entries. Do not rewrite history — resolved entries get
`Status: resolved (<commit/spec>)`, they stay.

Two entry kinds: **friction** (something blocked or misled the agent) and
**improvement** (workflow succeeded, but a concrete change would make it
faster, clearer, or harder to get wrong).

## Friction format

```markdown
### <YYYY-MM-DD> — friction — <short title>
- **Context:** spec slug, phase, role, exact command run
- **Expected:** what the agent believed would happen and why
- **Actual:** exact output / exit code / blocker (shortest decisive line)
- **Root cause:** harness bug | missing guidance | ambiguous docs | agent error
- **Recommendation:** concrete change (verb/flag/gate/doc/message text)
- **Status:** open
```

## Improvement format

```markdown
### <YYYY-MM-DD> — improvement — <short title>
- **Context:** spec slug, phase, role, command sequence that worked
- **Observation:** what cost time or attention despite succeeding
- **Cost:** turns, re-reads, redundant commands, or context tokens burned
- **Recommendation:** concrete change (verb/flag/gate/doc/message text)
- **Tradeoff:** what it costs — invariant risk, added surface, or none
- **Status:** open
```

Improvement entries must clear a bar: an agent-hours or correctness win a
future run can feel. "Nice to have" is not an entry. Anything that would weaken
determinism, evidence integrity, or add a bypass gets logged with the tradeoff
stated plainly and stays a proposal — never a self-applied change.

---

### 2026-07-20 — friction — `specd help link` omits the allowed `--kind` values
- **Context:** authoring spec `agent-driver-protocol`, phase perceive, role craftsman. `specd link agent-driver-protocol agent-protocol-clarity --kind depends-on --reason "..."`
- **Expected:** `depends-on` accepted, or the allowed set visible in `specd help link` before the call. Help printed `--kind  Link kind (default: follows).` with no enumeration.
- **Actual:** `usage: flag --kind="depends-on" not allowed; expected one of [follows regresses maintains supersedes]` (exit 2)
- **Root cause:** missing guidance — the enumeration exists in the validator but not in the palette help text.
- **Recommendation:** render enumerated flag values in `specd help <verb>`, e.g. `--kind  Link kind: follows|regresses|maintains|supersedes (default: follows).` Palette is the single source of truth, so this is one field, and `docs-lint.sh` keeps the reference in sync.
- **Status:** open

### 2026-07-20 — friction — steering templates never load into the machine manifest
- **Context:** fresh `git init` + `specd init` + `specd new demo` in a throwaway tree, specd 1.0.0. `specd context demo T1 --json`
- **Expected:** the five shipped steering templates appear in `items`, since `specd context demo T1` lists all six.
- **Actual:** all five in `omissions` with reason `missing explicit applicability metadata`; `grep -rl specd-context internal/core/embed_templates/steering/` returns nothing.
- **Root cause:** harness bug — no shipped steering template carries the `specd-context` block `SelectSteering` requires, so every driver runs with no project constitution and nothing reports an error.
- **Recommendation:** ship a permissive `specd-context` block in each steering template, and make total steering omission a warning-severity `specd check` finding. Tracked as spec `template-conformance` R1.
- **Status:** open

### 2026-07-20 — friction — requirements scaffold cannot pass `ParseRequirements`, and the error blames tasks.md
- **Context:** same throwaway tree; scaffolded `requirements.md` emits `## Requirement R1 — <name>` and `- **R1.1** When …`
- **Expected:** filling the scaffold as written yields parseable requirement IDs.
- **Actual:** `RequirementIDSet` is empty because the parser wants `## R1 —` and `- R1.1:`; the failure surfaces as `error task-trace: T1 references unknown requirement R1.1` — against the wrong file, for a reference that is correct.
- **Root cause:** harness bug — template/consumer contract with no test asserting it, plus displaced error attribution.
- **Recommendation:** correct the template to the parsed format, and when `RequirementIDSet` is empty say so against `requirements.md` instead of reporting an unknown reference against `tasks.md`. Tracked as spec `template-conformance` R2.1/R3.1.
- **Status:** open

### 2026-07-20 — friction — tasks template teaches `evidence=tests`, which the quality-declaration gate rejects
- **Context:** same throwaway tree; `tasks.md` example comment ships `evidence=tests`.
- **Expected:** the template's own example value is accepted.
- **Actual:** `QUALITY_DECLARATION_INVALID: "tests" must be class/check-id (valid classes: test, output_eval, trajectory_eval, review)`
- **Root cause:** harness bug — same untested template/consumer contract.
- **Recommendation:** change the example to `evidence=test/<check-id>`. Tracked as spec `template-conformance` R2.2.
- **Status:** open

### 2026-07-20 — friction — task `files:` cannot express the test file its own `verify` line requires
- **Context:** executing spec `agent-protocol-clarity` T1, role craftsman. Task declares `files: internal/core/roles.go` and `verify: go test ./internal/core -run TestRoleCapabilityContract -count=1`.
- **Expected:** the declared file set covers everything the task must touch to make its own verify line pass.
- **Actual:** `TestRoleCapabilityContract` did not exist, so the verify line could only pass by editing `internal/core/roles_test.go` — a file the task never declared. `.specd/roles/craftsman.md` says "Touch only files explicitly named in the task's `files:`; tests must also be declared", so the role text names the obligation the task row cannot satisfy. Nothing blocked: `specd verify` and `specd complete-task` both exited 0 against an undeclared write.
- **Root cause:** authoring gap with no gate behind it — a `verify` line naming `-run <TestName>` implies a test file, and no check asserts that file is declared.
- **Recommendation:** in the tasks gate, when a `verify` line contains `-run <TestName>`, require at least one `_test.go` path in that task's `files:`. Fails closed at authoring time, costs one regexp, and needs no new state. Deterministic — it reads the tasks table only.
- **Status:** open

### 2026-07-20 — friction — slug traversal is guarded at ~29 call sites, never at the path builders
- **Context:** executing `agent-protocol-clarity` T5. A new typed-refusal test drove `specd check ../../escape` and found a second, independent traversal-rejection path in `loadSpec` (`internal/cmd/registry.go`) that the migrated `checkPhase` route never reaches — `check` declares `PhaseAny`, so the phase gate returns before validating.
- **Expected:** one chokepoint rejects a traversal slug, or the sink refuses to build the path.
- **Actual:** `grep -rn ValidateSlug internal/ --include="*.go"` returns ~29 non-test call sites. Every spec path builder (`StatePath`, `EvidencePath`, `SpecMemoryPath`, `EvalStorePath`, `RunLedgerPath`, ~20 total) takes a slug and `filepath.Join`s it with **no** validation, e.g. `filepath.Join(SpecdDir(root), "specs", slug, "memory.md")`. Safety depends entirely on every caller remembering. Probed all 29 spec-taking verbs against a live tree: **no escape today** — this is fragility, not a live vulnerability.
- **Root cause:** defense placed at callers instead of the sink, with no palette field marking "this verb takes a slug" (`SpecSlugArg` is set only on phase-enforced verbs; usage strings spell it `<spec>`, `<slug>`, `<from-slug>`, `<new-spec>` inconsistently). The existing `TestSlugTraversalRejected` was a hand-maintained list of 15 verbs, so `drift`, `archive`, `spike`, `brain`, `unlink` and any future verb were uncovered.
- **Recommendation:** add `core.SpecDir(root, slug string) string` as the single join for everything under `.specd/specs/<slug>/`, and route all ~20 builders through it. `url.PathEscape` on the slug inside it is a no-op for valid slugs (`^[a-z0-9][a-z0-9-]*$`) and neutralizes traversal, so it is defense-in-depth with zero behavior change and no signature churn. Separately, add a `TakesSpecSlug` bool to the palette so coverage tests stop inferring it from usage prose. Needs its own task — outside T5's declared files.
- **Status:** open

