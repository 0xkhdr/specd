# Requirements — Regression: CLI + Command Surface (args, lifecycle, JSON contracts)

## Introduction
The CLI (`internal/cli`) and command handlers (`internal/cmd`) are specd's primary human and
agent interface: ~25 subcommands, their flags, exit codes, and `--json` contracts. A
regression here freezes the public contract so that agents parsing specd output never break
on a silent format change. Value: specd is a stable, scriptable substrate for agent harnesses.

## Requirement 1 — Argument parsing & help integrity
**User story:** As a user, I want consistent flag parsing and help across all subcommands, so
that the CLI is predictable.

**Acceptance criteria:**
1. THE SYSTEM SHALL parse every documented flag for every subcommand
2. IF an unknown flag or missing required arg is given THEN THE SYSTEM SHALL print a usage error and exit non-zero
3. WHEN `--help` is requested THE SYSTEM SHALL print usage and exit zero

## Requirement 2 — Stable JSON contracts
**User story:** As an agent parsing specd, I want `--json` output to be a stable schema, so
that my parser does not break between releases.

**Acceptance criteria:**
1. WHEN a command supports `--json` THE SYSTEM SHALL emit valid JSON with a stable top-level shape
2. THE SYSTEM SHALL keep `--json` output free of human-only decoration (no ANSI, no spinners)
3. IF a command fails THEN THE SYSTEM SHALL still emit machine-readable error context under `--json`

## Requirement 3 — Lifecycle end-to-end
**User story:** As an author, I want the new→check→approve→task→report lifecycle to work end to
end, so that a spec can be driven entirely from the CLI.

**Acceptance criteria:**
1. WHEN a spec is created, checked, approved, and its tasks flipped THE SYSTEM SHALL advance phases correctly
2. IF a lifecycle step is attempted out of order THEN THE SYSTEM SHALL block it with a clear reason
3. THE SYSTEM SHALL produce a report reflecting the final state

## Requirement 4 — Exit-code taxonomy
**User story:** As a script author, I want documented, distinct exit codes, so that I can
branch on failure type.

**Acceptance criteria:**
1. THE SYSTEM SHALL return distinct, documented exit codes for success, validation failure, and gate block
2. WHEN a command succeeds THE SYSTEM SHALL exit zero
3. THE SYSTEM SHALL keep exit codes stable across the regression suite (golden)
