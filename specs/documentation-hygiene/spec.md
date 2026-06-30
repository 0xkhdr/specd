# Spec — Documentation & Repository Hygiene (S7)

## Introduction

Live evidence (see `../discrepancies.md` D8, D14, D17, F11) refines this spec
from a generic "improve docs" ask into four specific items: `AGENTS.md`
(293 lines) and `TESTING.md` (265 lines) genuinely lack a table of contents;
`scripts/docs-lint.sh` already exists and works, but extend it rather than
recreate it; the README's Windows-support statement is narrower in reality
than the plan implied (only Brain/Pinky orchestration is POSIX-only); and
`AGENTS.md:252-293` contains an unrelated, out-of-scope content block (a
third-party tool's instructions) whose removal needs explicit user
confirmation since its provenance and intent are unclear.

## Requirement 1 — Table of contents for large docs

**User story:** As a contributor opening `AGENTS.md` for the first time, I
want a table of contents so I can jump to the relevant section instead of
scrolling through 293 lines.

**Acceptance criteria:**
1. THE SYSTEM SHALL add a table of contents (linked to each `##` heading) at
   the top of `AGENTS.md` and `TESTING.md`, immediately after each file's
   title/intro paragraph.
2. THE SYSTEM SHALL keep the table of contents in sync with headings — WHERE
   feasible, generate it via a script (extending `scripts/docs-lint.sh`,
   per Requirement 3) rather than maintaining it by hand, so it can't drift.

## Requirement 2 — Correct the Windows support claim

**User story:** As a Windows user reading the README, I want an accurate
statement of what's actually unsupported, so I don't wrongly conclude the
entire CLI is unusable on Windows.

**Acceptance criteria:**
1. THE SYSTEM SHALL ensure any documentation summarizing Windows support
   (README.md, AGENTS.md, or this review's own output) states precisely that
   **Brain/Pinky worker orchestration** is POSIX-only on Windows (with the
   exact runtime error message it produces), while general `specd` usage
   works on Windows given a POSIX-compatible shell on `PATH`.
2. THE SYSTEM SHALL NOT broaden or narrow this claim beyond what
   `README.md:68`'s actual current wording supports — verify against the
   live file at implementation time, not against this spec's quoted text,
   in case the wording has since changed.

## Requirement 3 — Extend (not replace) scripts/docs-lint.sh

**User story:** As a maintainer relying on `docs-lint.sh`'s dead-command-
reference check, I want its coverage extended to the new TOC consistency
check (Requirement 1.2) without disrupting its existing, working checks.

**Acceptance criteria:**
1. THE SYSTEM SHALL add a new check function to the existing
   `scripts/docs-lint.sh` (not a new separate script) that verifies each
   `##`-heading in `AGENTS.md`/`TESTING.md` has a corresponding TOC entry
   and vice versa.
2. THE SYSTEM SHALL preserve `docs-lint.sh`'s two existing checks (dead
   command references against `.specd/specs/cmd-audit/audit.csv`; the
   20-command cheat-sheet table match) unmodified in behavior.
3. WHERE the existing script's hardcoded 20-command list
   (`docs/command-reference.md` cheat-sheet check) is a maintainability risk
   (it will silently become wrong as commands are added/removed without a
   corresponding `cmd-audit` entry) THE SYSTEM SHALL flag this in a code
   comment as a known limitation — fixing it is out of this spec's scope
   (it would require redesigning the cheat-sheet-table source of truth,
   which is a separate, larger change).

## Requirement 4 — Flag (do not silently remove) the unrelated AGENTS.md block

**User story:** As the repository owner, I want to be asked before an
unrelated content block in my agent-instructions file is deleted, since I
may have added it intentionally for my own tooling even though it's unrelated
to specd's own conventions.

**Acceptance criteria:**
1. THE SYSTEM SHALL NOT delete `AGENTS.md:252-293` (the unrelated
   third-party-tool instruction block) as part of automated task execution.
2. THE SYSTEM SHALL surface this block's existence and exact line range as
   an explicit decision point requiring the user's confirmation before any
   removal, in the task evidence for the relevant task (see tasks.md T4).

## Design

### Overview
Three additive documentation changes (TOC, Windows wording correction,
docs-lint extension) and one flagged-not-executed decision point.

### Architecture
No code architecture change — `scripts/docs-lint.sh` gains one new bash
function; `AGENTS.md`/`TESTING.md`/`README.md` gain/correct prose.

### Components and interfaces
- `AGENTS.md`, `TESTING.md` — TOC added.
- `README.md` — Windows statement verified/corrected if it has drifted from
  the quoted live text.
- `scripts/docs-lint.sh` — new TOC-consistency check function.

### Data models
No changes.

### Error handling
`docs-lint.sh`'s new check fails closed (non-zero exit) like its existing
checks, with a specific message naming the missing/orphaned TOC entry.

### Verification strategy
- `bash scripts/docs-lint.sh` passes with the new check active.
- Manual diff review of the Windows-support wording change against the live
  `README.md:68` text at implementation time.

### Risks and open questions
- Decision required from the user (Requirement 4): confirm whether
  `AGENTS.md:252-293` should be removed, kept, or moved to a separate
  tool-specific file outside `AGENTS.md`. This spec's tasks stop short of
  making that call.
