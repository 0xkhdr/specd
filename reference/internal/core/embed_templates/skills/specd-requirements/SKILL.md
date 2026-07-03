---
name: specd-requirements
description: Author requirements.md in EARS form for a specd spec. Load when entering the requirements (ANALYZE) phase. Covers EARS syntax and the `ears` gate that `specd check` enforces before you can `specd approve` into design.
---

# specd requirements

Phase ANALYZE: pin down what must be true, in EARS. Start with `specd new <slug>`
(or an existing spec at status `requirements`). Load `.specd/steering/product.md`
for grounding. Run `specd context <slug>` for the phase-scoped briefing.

## EARS — the required form

Every acceptance criterion must be an EARS sentence. The `ears` gate lints
`requirements.md` and `specd check <slug>` fails (exit 1) on any non-EARS line.
The supported patterns:

- **Ubiquitous** — `THE SYSTEM SHALL <response>.`
- **Event-driven** — `WHEN <trigger>, THE SYSTEM SHALL <response>.`
- **State-driven** — `WHILE <state>, THE SYSTEM SHALL <response>.`
- **Unwanted behavior** — `IF <condition>, THEN THE SYSTEM SHALL <response>.`
- **Optional feature** — `WHERE <feature>, THE SYSTEM SHALL <response>.`

Each requirement gets a user story ("As a … I want … so that …") plus one or more
numbered EARS criteria. The criterion numbers (`<req>.<n>`) are what tasks trace to
and what `specd verify --criterion` records against.

## The gate `specd check` enforces here

- `ears` — every acceptance criterion parses as EARS; `requirements.md` must exist
  and be non-empty.

## Exit and advance

```
specd check <slug>      # ears gate green (exit 0)
specd approve <slug>    # human approves requirements → advances to design
```

Do not write design or tasks yet. Author only WHAT must be true, not HOW. Then load
`specd-design` when the approve advances you into the design phase.
