# Tasks — Replace `boot` + `enrich` with an init-scaffolded skill pack

> Waves are dependency batches. A task's `depends` must live in an earlier-or-equal
> wave. Mirrors the specd task schema (seven mandatory keys). `verify:` lines are
> the deterministic proof each task is done.

---

## Wave 0 — Author the skill pack (no removals yet; additive, safe)

- [ ] T1 — Write `specd-foundations` skill
  - why: the always-loaded constitution + skill index that makes every other skill lazy-loadable (Goal 4)
  - role: builder
  - files: internal/core/embed_templates/skills/specd-foundations/SKILL.md
  - contract: YAML frontmatter (name, description). Body = the 8 principles, the Foundational Split, exit codes 0/1/2/3, the `.specd/` file map, and a one-line index of the other 5 skills with their trigger condition. Keep to ~1 screen. Do NOT restate full stage procedures here — link to the stage skills.
  - acceptance: file parses as markdown with valid frontmatter; names all 5 sibling skills; cites real exit codes from internal/core/exit.go
  - verify: test -f internal/core/embed_templates/skills/specd-foundations/SKILL.md && grep -q "name:" internal/core/embed_templates/skills/specd-foundations/SKILL.md
  - depends: —
  - requirements: 3, 4

- [ ] T2 — Write `specd-steering` skill (the boot + enrich replacement)
  - why: the agent must learn to inspect the repo itself and author product/structure/tech.md + set config.defaultVerify — replacing both deleted commands (Goal 1, 3)
  - role: builder
  - files: internal/core/embed_templates/skills/specd-steering/SKILL.md
  - contract: Teach the agent to (a) detect stack/layout/CI from manifests, dir tree, README, CI files; (b) author each steering file grounding claims in cited evidence, never invented; (c) set config.defaultVerify to the detected test command. Reuse the sound guidance from the old specd-enrich SKILL.md and enrich_evidence.go targetSections/targetInstructions. No reference to `specd boot`/`specd enrich`.
  - acceptance: covers product, structure, tech, and defaultVerify; contains zero `specd boot`/`specd enrich` invocations
  - verify: f=internal/core/embed_templates/skills/specd-steering/SKILL.md; test -f $f && ! grep -qE "specd (boot|enrich)" $f
  - depends: —
  - requirements: 1, 3

- [ ] T3 — Write the four stage skills (requirements/design/tasks/execute)
  - why: per-stage progressive disclosure so the agent loads only the stage it is in (Goal 4)
  - role: builder
  - files: internal/core/embed_templates/skills/specd-requirements/SKILL.md, internal/core/embed_templates/skills/specd-design/SKILL.md, internal/core/embed_templates/skills/specd-tasks/SKILL.md, internal/core/embed_templates/skills/specd-execute/SKILL.md
  - contract: Each = frontmatter + the stage's authoring rules and the exact gate(s) `specd check` enforces for it. requirements→EARS; design→mandatory design.md sections; tasks→wave DAG + 7 keys + acyclicity; execute→next/implement/verify/task evidence loop + roles + dispatch. Ground every named command in real CLI behavior.
  - acceptance: all four files exist with valid frontmatter; every `specd <cmd>` mentioned exists in core.Commands
  - verify: for s in requirements design tasks execute; do test -f internal/core/embed_templates/skills/specd-$s/SKILL.md || exit 1; done
  - depends: —
  - requirements: 3, 4

## Wave 1 — Wire init to scaffold the pack

- [ ] T4 — Add `core.SkillsDir` path helper
  - why: init needs a single source for the `.specd/skills` location (Design §4.3)
  - role: builder
  - files: internal/core/paths.go
  - contract: add `func SkillsDir(root string) string` returning `.specd/skills`, matching the style of SteeringDir/RolesDir. Nothing else.
  - acceptance: builds; consistent with sibling helpers
  - verify: go build ./... && grep -q "func SkillsDir" internal/core/paths.go
  - depends: —
  - requirements: 2

- [ ] T5 — Extend `RunInit` to place skill files
  - why: make init the single scaffolder of the skill pack (Goal 2)
  - role: builder
  - files: internal/cmd/init.go
  - contract: add a `skillFiles` slice of the 6 skill names; loop calling the existing `place(core.SkillsDir(root)+"/"+s+"/SKILL.md", "skills/"+s+"/SKILL.md")`. Preserve idempotency + `--force`. Do not touch the AGENTS.md merge logic here.
  - acceptance: `specd init` in a temp dir writes all 6 SKILL.md; re-run skips; `--force` rewrites
  - verify: go build -o /tmp/specd . && d=$(mktemp -d) && (cd $d && /tmp/specd init) && test -f $d/.specd/skills/specd-foundations/SKILL.md && test -f $d/.specd/skills/specd-execute/SKILL.md
  - depends: T1, T2, T3, T4
  - requirements: 2

- [ ] T6 — Add embedded-skill parity test
  - why: prevent a skill named in init's list without a shipped template (Risk: skill drift)
  - role: builder
  - files: internal/cmd/init_test.go
  - contract: table test asserting every name in `skillFiles` has a readable `skills/<name>/SKILL.md` via core.ReadTemplate, and that `specd init` writes each. Mirror the spirit of TestRegistryMatchesHelp.
  - acceptance: test passes; fails if a skill is added to the list without a template
  - verify: go test ./internal/cmd/ -run TestInit -count=1
  - depends: T5
  - requirements: 2

## Wave 2 — Remove `boot` and `enrich`

- [ ] T7 — Delete boot/enrich command + core files
  - why: remove the perception/authoring brokers that violate the Foundational Split (Goal 1)
  - role: builder
  - files: internal/cmd/boot.go, internal/cmd/boot_test.go, internal/cmd/enrich.go, internal/cmd/enrich_test.go, internal/core/boot.go, internal/core/boot_detectors.go, internal/core/boot_test.go, internal/core/enrich.go, internal/core/enrich_evidence.go, internal/core/enrich_test.go, internal/core/embed_templates/skills/specd-enrich/
  - contract: delete exactly these paths. Do not touch task/verify/check-gate/DAG/state/lock code. Compile breakages in registry/commands/check are expected and fixed in T8.
  - acceptance: files gone; remaining build breakage is confined to registry.go, commands.go, check.go, args.go
  - verify: ! test -e internal/cmd/boot.go && ! test -e internal/core/enrich.go && ! test -e internal/core/embed_templates/skills/specd-enrich
  - depends: T6
  - requirements: 1

- [ ] T8 — Unwire boot/enrich from registry, commands meta, check, args
  - why: restore a clean build and remove the two repo-global freshness gates (Goal 1, Design §4.6)
  - role: builder
  - files: internal/cmd/registry.go, internal/core/commands.go, internal/cmd/check.go, internal/cli/args.go
  - contract: drop the `boot`/`enrich` Registry entries; drop their CommandMeta; remove check.go's `--boot`/`--enrich` branches + runBootCheck/runEnrichCheck helpers + update its usage string; remove `"boot"`/`"enrich"` from args.go bool set. No behavior change to `check <slug>`.
  - acceptance: `go build ./...` clean; `specd boot` and `specd enrich` exit 2 (unknown command); `specd check <slug>` unchanged
  - verify: go build -o /tmp/specd . && /tmp/specd boot; test $? -eq 2 && /tmp/specd enrich; test $? -eq 2
  - depends: T7
  - requirements: 1

- [ ] T9 — Fix parity / lifecycle tests
  - why: the registry↔help parity and lifecycle tests guard exactly this kind of change and must reflect the new command set
  - role: builder
  - files: internal/cmd/registry_test.go, internal/cmd/commands_test.go, internal/cmd/lifecycle_test.go, main_test.go
  - contract: update expected command lists/counts to exclude boot/enrich; remove any lifecycle step invoking them. Do not weaken assertions — only remove the deleted commands.
  - acceptance: full suite green with -race
  - verify: go test ./... -race -count=1
  - depends: T8
  - requirements: 1, 5

## Wave 3 — Docs, AGENTS.md, and the context pointer

- [ ] T10 — Rewrite AGENTS.md template (skills index + bootstrap step)
  - why: the agent's entry doc must drive the lifecycle via skills, not boot/enrich (Design §4.4)
  - role: builder
  - files: internal/core/embed_templates/AGENTS.md
  - contract: replace the `specd boot`/`specd enrich` Quickstart lines with a "read .specd/skills/specd-steering/SKILL.md to bootstrap steering" step; add a Skills section indexing all 6 skills + their trigger and the progressive-disclosure rule (load a stage skill only when entering that stage).
  - acceptance: zero `specd boot`/`specd enrich` references; names all 6 skills
  - verify: f=internal/core/embed_templates/AGENTS.md; ! grep -qE "specd (boot|enrich)" $f && grep -q "specd-steering" $f
  - depends: T9
  - requirements: 2, 4

- [ ] T11 — Sweep repo docs for boot/enrich and the gate count
  - why: docs must not describe removed commands/gates (Acceptance §6)
  - role: builder
  - files: README.md, AGENTS.md, CHANGELOG.md, docs/concepts.md, docs/user-guide.md, docs/command-reference.md, docs/validation-gates.md, docs/agent-integration.md, docs/contributor-guide.md, TESTING.md
  - contract: remove/replace boot+enrich references; change "7 (+2 repo-global) gates" to "7 gates"; add a CHANGELOG breaking-change entry; describe the skill pack where boot/enrich were documented. Do not invent features.
  - acceptance: no doc describes `specd boot`/`specd enrich` as live commands except the CHANGELOG removal note
  - verify: ! grep -rInE "specd (boot|enrich)" README.md docs/ TESTING.md | grep -v CHANGELOG
  - depends: T10
  - requirements: 1

- [ ] T12 — Point `specd context` at the per-phase skill (optional enhancement)
  - why: harness points at the right knowledge without being it (Design §4.5)
  - role: builder
  - files: internal/cmd/context.go, internal/cmd/context test (if present)
  - contract: in the phase-scoped briefing, name the relevant `.specd/skills/specd-<stage>/SKILL.md` for the current phase. Additive output only; do not change exit codes or JSON keys other than adding a `skill` field. Droppable if it risks the gate.
  - acceptance: `specd context <slug>` names a skill matching the current phase
  - verify: go test ./internal/cmd/ -run Context -count=1
  - depends: T11
  - requirements: 4

## Wave 4 — Final gate

- [ ] T13 — Full CI gate + manual lifecycle smoke
  - why: prove the refactor is integrity-clean end to end (Acceptance §6)
  - role: verifier
  - files: —
  - contract: run `make ci`; then in a temp dir run init → confirm skills written → new → check → approve through executing, with no boot/enrich anywhere. Read-only; record evidence.
  - acceptance: make ci green; lifecycle smoke passes with the skill pack present and boot/enrich absent
  - verify: make ci
  - depends: T12
  - requirements: 5
