# Workflow feedback log

Append-only log of friction hit while *using* specd to build specs. Input for
improving the harness so agents work with specd as native knowledge.

Do not delete entries. Do not rewrite history — resolved entries get
`Status: resolved (<commit/spec>)`, they stay.

## Entry format

```markdown
### <YYYY-MM-DD> — <short title>
- **Context:** spec slug, phase, role, exact command run
- **Expected:** what the agent believed would happen and why
- **Actual:** exact output / exit code / blocker (shortest decisive line)
- **Root cause:** harness bug | missing guidance | ambiguous docs | agent error
- **Recommendation:** concrete change (verb/flag/gate/doc/message text)
- **Status:** open
```

---
