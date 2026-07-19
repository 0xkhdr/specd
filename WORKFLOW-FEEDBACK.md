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
