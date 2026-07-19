# Template Conformance — Analysis and Action Plan

**Date:** 2026-07-20
**Status:** proposed — nothing here is applied except the agent-definition fix already committed
**Scope:** `internal/core/embed_templates/`, `internal/core/scaffold.go`, and the consumers that parse what those templates emit

---

## 1. Summary

specd ships scaffold templates that its own parsers, gates, and host harnesses reject.
Four independent instances were found and reproduced. They are not four unrelated bugs — they
are one structural gap: **no test asserts that a shipped template satisfies the consumer that
reads it.** Every instance is a template/consumer contract that exists only in a developer's head.

The user-visible effect ranges from noisy warnings to silent capability loss. The steering
instance is the worst: the default install produces steering files that are silently dropped
from every machine context manifest, so an orchestrated agent runs with no project constitution
and nothing reports an error.

| # | Template | Consumer | Symptom | Severity |
|---|---|---|---|---|
| 1 | `.codex/agents/*.toml`, `.claude/agents/*.md` | codex / Claude Code loaders | agent roles rejected at load | high — **fixed** |
| 2 | `steering/*.md` | `context.SelectSteering` | steering silently omitted from machine manifest | **critical** |
| 3 | `specs/<slug>/requirements.md` | `core.ParseRequirements` | scaffold cannot pass its own gate | high |
| 4 | `specs/<slug>/tasks.md` | `quality-declaration` gate | template's own example value rejected | medium |

---

## 2. Findings

### 2.1 Agent definitions rejected by both hosts — FIXED

`pinkyCodexAgent` emitted `name` + `instructions`; codex requires `description` +
`developer_instructions`. `pinkyClaudeAgent` emitted plain markdown with no YAML frontmatter;
Claude Code requires `name` + `description` frontmatter to register a subagent.

Observed:

```
⚠ Ignoring malformed agent role definition: agent role file at .codex/agents/pinky-auditor.toml
  must define `developer_instructions`
⚠ Ignoring malformed agent role definition: agent role `pinky-auditor` must define a description
```

Eight warnings = four files failing two distinct codex checks. The Claude Code half failed
silently — no warning at all, the subagents simply never registered.

**Resolution:** `internal/core/scaffold.go:129` now emits both required fields for both hosts,
sharing one `pinkyRoleDescription(role)` source of truth.
`TestPinkyAgentDefinitionsCarryHostRequiredFields` asserts both schemas. Full suite passes.

**Operator action:** rebuild/reinstall `specd` (templates are `go:embed`'d), then:

```bash
specd init --agent=pinky --refresh --dry-run   # preview
specd init --agent=pinky --refresh
```

### 2.2 Steering silently dropped from the machine manifest — CRITICAL

There are two manifest paths and they disagree:

| Path | Builder | Steering selection | Requires metadata |
|---|---|---|---|
| `specd context <slug> <task>` | `BuildManifest` → `steeringItems` (`manifest.go:205`) | every `.md` in the directory | no |
| `specd context <slug> <task> --json` | `BuildMachineManifest` → `SelectSteering` (`steering.go:203`) | only files with a `specd-context` block | **yes** |

`SelectSteering` drops any file lacking the block with reason
`missing explicit applicability metadata`. **No shipped steering template contains one**
(`grep -rln specd-context internal/core/embed_templates/steering/` → no matches).

Reproduced on a fresh `specd init` + spec driven to `executing`:

```
$ specd context demo T1            # plain — all six present
.specd/steering/memory.md
.specd/steering/product.md
.specd/steering/reasoning.md
.specd/steering/structure.md
.specd/steering/tech.md
.specd/steering/workflow.md

$ specd context demo T1 --json     # machine — all five dropped
STEERING ITEMS: []
STEERING OMISSIONS: [
  ('.specd/steering/product.md',   'missing explicit applicability metadata'),
  ('.specd/steering/reasoning.md', 'missing explicit applicability metadata'),
  ('.specd/steering/structure.md', 'missing explicit applicability metadata'),
  ('.specd/steering/tech.md',      'missing explicit applicability metadata'),
  ('.specd/steering/workflow.md',  'missing explicit applicability metadata')]
```

`memory.md` is exempt by design (`steering.go:214`) and handled by `SelectMemory`.

This is the critical one because the machine manifest is what `specd brain`, the MCP surface,
and every non-interactive driver consume. The plain path masks it during manual use: an operator
who eyeballs `specd context` sees six healthy steering files and concludes it works.

**Compounding trap:** the entire template body ships *inside* the managed markers:

```
<!-- specd:managed:steering/tech.md:v1 begin -->
# Steering: Tech
- **Language / runtime:** <e.g. Go 1.22, stdlib only>   ← edited here = reverted
<!-- specd:managed:steering/tech.md:v1 end -->
                                                         ← edited here = preserved
```

Verified: an edit inside the markers was silently reverted by `specd init --repair`; a section
appended below the closing marker survived. So the placeholders read as slots to fill in, but
filling them in is exactly the action that loses the work. The correct workflow — append below
the marker — is documented nowhere and is not discoverable from the file.

Verified fix, both loading and surviving `--repair`:

```markdown
<!-- specd-context
id: tech
version: 1
priority: 20
phases: plan, execute
roles: craftsman, scout
files: **/*.go
-->

## Stack (project)
- Go 1.26, stdlib only.
```

```
$ specd context demo T1 --json
ITEMS: [('.specd/steering/tech.md', None)]
$ specd init --repair && grep -c specd-context .specd/steering/tech.md
1
```

Keys (`steering.go:parseMetadata`): `id`, `version`, `priority`, `tags`, `phases`, `roles`,
`tasks`, `requirements`, `fields`, `files`. Omitted selector = matches everything.
Default priority 50; lower wins.

### 2.3 Requirements scaffold cannot pass its own gate

The template writes a format `ParseRequirements` does not accept — in two places:

| Element | Template emits | `requirements.go` regex | Match |
|---|---|---|---|
| heading | `## Requirement R1 — <name>` | `^#{2,4}\s+(R\d+)\b(.*)$` (:41) | no — `Requirement` precedes `R1` |
| criterion | `- **R1.1** When <trigger>, …` | `^[-*]\s+(R\d+\.\d+)\s*:\s*(.*)$` (:42) | no — wants `R1.1:` not `**R1.1**` |

Because no requirement IDs parse, `RequirementIDSet` returns empty and the failure surfaces
displaced onto the *tasks* file:

```
error task-trace: T1 references unknown requirement R1.1
```

The message accuses `tasks.md` of a bad reference when `requirements.md` is what is malformed —
and the reference is in fact correct. Reproduced end to end: the spec only advanced after
rewriting the heading to `## R1 —` and the criterion to `- R1.1: …`, i.e. after *departing from
the shipped template*.

This is the highest agent-hours cost of the four. An agent that fills in the scaffold faithfully
gets a gate error pointing at the wrong file, with no hint that the template itself is the defect.

### 2.4 Tasks template's own example value fails the quality-declaration gate

`tasks.md` ships this comment:

```
<!-- Example field values (not a runnable task): … evidence=tests; … -->
```

Copying `evidence=tests` yields:

```
error quality-declaration: T1 quality declaration invalid: QUALITY_DECLARATION_INVALID:
"tests" must be class/check-id (valid classes: test, output_eval, trajectory_eval, review;
expected format: class/check-id (example: test/unit-auth))
```

The template teaches a value the gate refuses. `evidence=test/readme-purpose` passes.

---

## 3. Root cause

One gap, four symptoms: **templates and their consumers are coupled by convention, not by a
test.** `TestWriteScaffoldPinkyArtifacts` checks that the files *exist*; nothing checks that
their *content* satisfies whoever parses it. That is why all four instances survived to release
in a codebase that is otherwise heavily gated.

Two aggravating factors specific to this repo:

- **Silent-omission ergonomics.** `SelectSteering` degrades quietly by design — omissions are
  data in a JSON field, not warnings. Correct for a context budget that intentionally drops
  low-priority material; wrong as the failure mode for *the entire steering set being
  unreadable*. Absence of signal is indistinguishable from correct filtering.
- **Displaced error attribution.** 2.3 reports against `tasks.md` for a `requirements.md` defect.
  A gate that names the wrong file costs more than one that says nothing.

This also brushes a stated invariant. CLAUDE.md §Docs sync requires `docs/command-reference.md`
be generated from the palette and lint-checked for drift — templates got no equivalent
protection, even though template drift is the more expensive failure.

---

## 4. Action plan

Ordered by user-visible harm per unit of work. Each item names its verification.

### P0 — stop the silent failure

**A1. Ship `specd-context` metadata in every steering template.**
Add a block to each of `product|reasoning|structure|tech|workflow.md`, above the managed content,
with permissive selectors (no `phases`/`roles`/`files` — match everything) and a sensible
`priority`. Default install then produces steering that actually loads.
*Verify:* new test asserts `SelectSteering` returns zero omissions for a freshly scaffolded root.

**A2. Make total steering omission loud.**
When `SelectSteering` drops *every* steering file for `missing explicit applicability metadata`,
that is a misconfiguration, not a budget decision. Emit a warning-severity finding through
`specd check`, or surface it in `agents doctor`. Keep the per-file omission silent.
*Verify:* test that a metadata-less steering dir produces exactly one diagnostic.
*Tradeoff:* adds a diagnostic path; no gate, no determinism impact — the check stays a pure
function of on-disk state.

### P1 — fix the templates that cannot pass their own gates

**A3. Correct the requirements template to the parsed format.**
`## R1 — <name>` and `- R1.1: When <trigger>, the system shall <observable response>.`
*Verify:* test that scaffolded `requirements.md` yields a non-empty `RequirementIDSet`.

**A4. Correct `evidence=tests` to `evidence=test/<check-id>` in the tasks template comment.**
*Verify:* covered by A5's round-trip.

**A5. Round-trip test: scaffold → gates.**
Scaffold a spec, fill only the placeholders each template marks as fill-in, and assert the
gate registry reports no *format* findings. This is the test whose absence caused all four
instances. It is the highest-leverage item in this plan.

### P2 — make the class of bug unrepeatable

**A6. Template conformance suite.**
One test per template asserting the shipped bytes satisfy their declared consumer:
steering → `SelectSteering`; requirements → `ParseRequirements`; tasks → tasks parser +
quality-declaration; agent definitions → host schema (done in 2.1).
Mirrors what `docs-lint.sh` does for the command reference, extended to templates.

**A7. Fix the displaced error message in `task-trace`.**
When `RequirementIDSet` is empty, say so — `requirements.md declares no parseable requirement
IDs (expected '## R<n>' headings and '- R<n>.<m>:' criteria)` — instead of blaming `tasks.md`.
*Verify:* test asserting the message on an unparseable requirements doc.

### P3 — document the marker contract

**A8. Put an editing instruction inside each managed region.**
The region is regenerated, so the instruction cannot rot:
`> Edit below the closing marker. Content inside these markers is regenerated by
specd init --repair/--refresh and your edits will be lost.`

**A9. Document the two manifest paths.**
`docs/agent-integration.md` should state that plain `specd context` lists all steering while
`--json` applies `specd-context` selection, and that the machine path is what drivers consume.

**A10. Consider `TemplateVersion` bump.**
Currently `1` (`managed.go:17`). A2–A4 change managed-region shape. If `--refresh` is meant to
migrate existing projects onto corrected templates, the version must move; if markers are
matched by `:v\d+` regardless (they are — `managed.go:100`), confirm old-version regions are
still located and replaced before bumping.

---

## 5. Operator guidance (applies today, before any of the above lands)

Best practice for updating steering with a coding agent, given current behavior:

1. **Never edit inside managed markers.** Append your content below the closing marker. Treat
   everything above it as generated. The placeholders are shape documentation, not slots.
2. **Add a `specd-context` block** to every steering file you rely on, below the marker, or the
   machine manifest ignores the file entirely.
3. **Verify it loads** — `specd context <slug> <task> --json` and confirm the file appears in
   `items`, not `omissions`. Do not trust the plain output; it lists files the driver never sees.
4. **`--dry-run` before every `--repair`/`--refresh`.** Prints managed-region changes, writes
   nothing.
5. **Hand-edit steering; do not route it through an agent write.** There is no CLI write verb
   for steering, by design — it is the operator-owned constitution.
6. **Use the flywheel for recurring lessons** rather than hand-writing one-off rules:
   ```bash
   specd memory <slug> add --key '<key>' --pattern '<one-line rule>' \
     --criticality important --related '<keys>'
   specd memory <slug> promote --key '<key>'
   ```
7. **Record contradictions with `specd decision`** — steering says invariants do not change
   without one, and `specd drift` projects declared invariants against verify evidence.

---

## 6. Invariant check

Nothing proposed here touches the guardrails in CLAUDE.md §Non-negotiable invariants:

- **Determinism** — every change is to static template bytes or a pure function over on-disk
  state. No LLM enters any gate, DAG, or report path.
- **Evidence integrity** — no completion path, verify record, or bypass is added. A2 and A7 add
  *diagnostics*, not gates that can be satisfied without evidence.
- **Structural** — no new dependency, no `go.sum`, no change to atomic writes, CAS, the lock, or
  the byte-stable tasks parser. Templates stay `go:embed`'d.
- **Subtractive bias** — A6 adds test surface, which is the point: it removes an entire class of
  defect rather than patching four instances. A10 is explicitly a question, not a change.

## 7. Follow-up

Findings 2.2, 2.3, and 2.4 are unresolved harness defects and warrant `WORKFLOW-FEEDBACK.md`
friction entries under the repo's dogfooding rule. Per that rule — *"never act on your own
recommendation in the same run"* — this document proposes; it does not apply. Only 2.1, which
was the reported bug, has been fixed.
