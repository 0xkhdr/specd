# Prompt for Claude Coding Agent — specd Command Palette Optimization Spec Suite

> **Mission:** Analyze the specd repository and the provided analysis document, then author a complete spec suite under `.specd/specs/` that plans the implementation of a minimal, non-redundant, user-and-agent-friendly command palette for the specd CLI.

---

## 1. Context Loading (REQUIRED — Do Not Skip)

Before writing any spec, load the following context in this exact order:

1. **Read the analysis document:** `specd_analysis_and_action_plan.md` (provided in context or at the path given by the user). This contains the foundational domain knowledge, command taxonomy, path analysis, and recommendations.
2. **Read the specd repository:** Clone or inspect `https://github.com/0xkhdr/specd`. Focus on:
   - `README.md` — feature overview and philosophy
   - `AGENTS.md` — agent workflow rules
   - `docs/command-reference.md` — every command, flag, exit code
   - `docs/agent-integration.md` — MCP tools, Brain/Pinky orchestration
   - `docs/user-guide.md` — lifecycle and artifact formats
   - `internal/core/commands.go` — CommandMeta registry (canonical command list)
   - `internal/cmd/*.go` — command implementations
3. **Read specd steering:** If `.specd/` exists in the target repo, read `.specd/steering/{reasoning,workflow,product,tech,structure}.md` and `.specd/skills/specd-foundations/SKILL.md`.

---

## 2. Mission Statement

The specd CLI currently exposes **~40+ commands** across Lifecycle, Execution, Inspection, Record, Program, Orchestration, and Meta categories. While comprehensive, this surface risks overwhelming both human users and AI coding agents. Your job is to design a **spec suite** that plans the implementation of an optimized command palette with the following properties:

- **Minimal:** Only commands required for the 95% workflow (plan → execute → verify → report).
- **Non-redundant:** No two commands should overlap in purpose. If one command can subsume another, eliminate the subsumed one or merge them.
- **Phase-aligned:** Commands must map 1:1 to the spec lifecycle phases (Analyze → Plan → Execute → Verify → Reflect).
- **Agent-containable:** The entire active palette must fit in a coding agent's working memory without reference lookups.
- **User-containable:** A human user should be able to memorize the full palette after one session.
- **Path-pure:** All file paths must be derived from `<slug>` only. No absolute paths, no manual path construction.

---

## 3. Spec Suite Structure

Create the following directory structure under `.specd/specs/`:

```
.specd/
├── specs/
│   ├── cmd-audit/
│   │   ├── spec.md      # Requirements + Design for full command audit
│   │   └── tasks.md     # Wave DAG to execute the audit
│   ├── cmd-merge/
│   │   ├── spec.md      # Requirements + Design for merging redundant commands
│   │   └── tasks.md     # Wave DAG to execute merges
│   ├── cmd-deprecate/
│   │   ├── spec.md      # Requirements + Design for deprecating low-utility commands
│   │   └── tasks.md     # Wave DAG to execute deprecations
│   ├── cmd-mcp-sync/
│   │   ├── spec.md      # Requirements + Design for MCP tool surface alignment
│   │   └── tasks.md     # Wave DAG to sync MCP with optimized CLI
│   ├── cmd-docs/
│   │   ├── spec.md      # Requirements + Design for docs rewrite (command-ref, agent-integration)
│   │   └── tasks.md     # Wave DAG to update docs
│   └── progress.md      # Master progress tracker for all waves across all specs
```

> **Note:** You may create additional specs if your analysis reveals further decomposition is needed (e.g., `cmd-aliases`, `cmd-shell-completions`, `cmd-error-codes`). However, **do not exceed 7 specs total** — if you need more, merge related concerns.

---

## 4. Artifact Format Requirements

### 4.1 `spec.md` Format

Each `spec.md` must contain two mandatory sections:

#### Section A: Requirements (EARS Syntax)
Use **EARS** (Easy Approach to Requirements Syntax) exclusively:

- **Ubiquitous:** `THE SYSTEM SHALL <behavior>`
- **Event-driven:** `WHEN <event> THE SYSTEM SHALL <behavior>`
- **State-driven:** `WHILE <state> THE SYSTEM SHALL <behavior>`
- **Optional-feature:** `WHERE <feature> THE SYSTEM SHALL <behavior>`
- **Unwanted:** `IF <condition> THEN THE SYSTEM SHALL <behavior>`

Each requirement must have:
- A unique REQ-ID (e.g., `REQ-001`, `REQ-002`)
- A user story (As a <role>, I want <goal> so that <benefit>)
- 2–5 acceptance criteria in EARS format
- A **rationale** sentence explaining why this requirement supports the "minimal palette" goal

#### Section B: Design (7 Mandatory H2 Headers)

```markdown
# Design — <Spec Name>

## Overview
## Architecture
## Components and interfaces
## Data models
## Error handling
## Verification strategy
## Risks and open questions
```

Each header must be non-empty and free of `TODO` placeholders. The **Verification strategy** must explicitly name which commands will be used to verify the spec's own implementation.

### 4.2 `tasks.md` Format

Tasks are Markdown checklist items grouped under `## Wave N` headers. Every task must include **all 7 metadata keys**:

| Key | Value Guidance |
|-----|----------------|
| `why` | Architectural reason this task exists |
| `role` | One of: `investigator`, `builder`, `reviewer`, `verifier` |
| `files` | Comma-separated files modified or researched |
| `contract` | Technical signature or behavior contract |
| `acceptance` | Test or user criteria for completion |
| `verify` | Shell command to verify (or `N/A` for read-only roles) |
| `depends` | Comma-separated task IDs, or `—` |

Use these checkbox conventions:
- `- [ ] T1` = pending
- `- [/] T1` = running
- `- [x] T1` = complete
- `- [!] T1` = blocked

### 4.3 `progress.md` Format

Create `.specd/specs/progress.md` as a master tracker. It must contain:

```markdown
# Spec Suite Progress — Command Palette Optimization

## Overall Status
- Total Specs: <n>
- Total Waves: <n>
- Tasks Complete: <n> / <total>
- Current Phase: <analyze|plan|execute|verify|reflect>

## Spec Registry
| Spec | Status | Current Wave | Blockers |
|------|--------|--------------|----------|
| cmd-audit | <status> | <wave> | <none or reason> |
| cmd-merge | <status> | <wave> | <none or reason> |
| ... | ... | ... | ... |

## Wave Schedule
### Wave 1: Audit & Analysis
- [ ] cmd-audit: T1–T3
### Wave 2: Merge & Deprecate
- [ ] cmd-merge: T1–T4
- [ ] cmd-deprecate: T1–T2
### Wave 3: MCP Sync
- [ ] cmd-mcp-sync: T1–T3
### Wave 4: Documentation
- [ ] cmd-docs: T1–T3
### Wave 5: Integration Verify
- [ ] All specs: final verify + approve

## Cross-Spec Dependencies
- `cmd-merge` depends on `cmd-audit`
- `cmd-deprecate` depends on `cmd-audit`
- `cmd-mcp-sync` depends on `cmd-merge` and `cmd-deprecate`
- `cmd-docs` depends on `cmd-mcp-sync`
```

Update `progress.md` as you advance through waves.

---

## 5. Optimization Criteria (Apply These Rigorously)

For every command in the current specd CLI, apply this decision matrix:

| Question | If YES | If NO |
|----------|--------|-------|
| Is this command required in the 95% workflow? | Keep | Consider deprecation |
| Does another command subsume this functionality? | Merge into the broader command | Keep separate |
| Is this command used by both user and agent? | Keep | Consider hiding or deprecating |
| Does this command mutate state? | Must be in palette | Could be deprecated if read-only alternative exists |
| Is this command part of a phase gate? | Keep | Consider removal |
| Does this command have a unique exit code contract? | Keep | Merge or remove |

### 5.1 Commands That MUST Survive (Non-negotiable)

These are the backbone of the specd workflow. Do not remove:

- `specd init` — bootstrap
- `specd new` — spec creation
- `specd check` — validation gates
- `specd approve` — phase ratchet
- `specd next` — frontier dispatch
- `specd verify` — evidence gate
- `specd task --status` — state mutation
- `specd status` — orientation
- `specd context` — context engineering
- `specd report` — reporting
- `specd brain start|step|status` — orchestration (if enabled)
- `specd pinky claim|report|release` — worker lifecycle (if enabled)

### 5.2 Commands That SHOULD Be Merged or Deprecated

Based on the analysis, consider the following for elimination or merging:

- `specd doctor` → merge into `specd init --repair` or `specd init --doctor`
- `specd mode` → merge into `specd new --orchestrated` and `specd status` (show mode)
- `specd dispatch` → merge into `specd next --all --json` (add a `--dispatch` flag)
- `specd serve` → deprecate; `specd report --serve` or separate binary
- `specd watch` → deprecate; use `specd report --watch` or external tool
- `specd replay` → merge into `specd report --history`
- `specd diff` → merge into `specd report --diff`
- `specd schema` → keep but move to `specd validate --schema` (already exists)
- `specd validate` → keep as `specd check --schema-only`
- `specd decision` / `specd midreq` / `specd memory` → merge into a single `specd record` command with sub-actions
- `specd program link/unlink` → merge into `specd new --depends-on` and `specd status --program`
- `specd brain run` → merge into `specd brain start --auto-step`
- `specd brain why` → merge into `specd brain status --verbose`
- `specd brain ledger` → merge into `specd brain status --ledger`
- `specd brain compact/clear` → merge into `specd brain checkpoint --compact`
- `specd brain directive` → merge into `specd brain step --directive`
- `specd pinky brief/heartbeat/progress/query/inbox/checkpoint` → merge into `specd pinky status` and `specd pinky update`
- `specd update/uninstall` → move to install script only; not part of runtime palette
- `specd mcp` → keep as meta, but hide from daily palette
- `specd version/help` → keep as meta, acceptable overload

### 5.3 Flag Consolidation

Merge overlapping flags:
- `--json` → universal; every command supports it
- `--sandbox` → consolidate to `specd verify --sandbox` only
- `--revert-on-fail` → consolidate to `specd verify --revert-on-fail` only
- `--evidence` → only on `specd task` and `specd record`
- `--unverified` → only on `specd task`
- `--all` → only on `specd next` and `specd status`
- `--format` → only on `specd report`
- `--pr-summary` → merge into `specd report --format pr`

---

## 6. Verification Strategy for the Spec Suite Itself

Each spec's `verify` commands must use the specd harness itself:

```bash
# Example verify commands for a builder task
specd check <spec-slug>
specd verify <spec-slug> <task-id>
```

For the final integration wave, use:

```bash
# Verify the optimized palette is consistent
specd check cmd-audit
specd check cmd-merge
specd check cmd-deprecate
specd check cmd-mcp-sync
specd check cmd-docs

# Verify no duplicate command names exist in the registry
# (Write a small Go test or script that asserts uniqueness)
go test ./internal/core/commands.go -run TestNoDuplicateCommands

# Verify the command reference doc mentions only the surviving palette
grep -c "specd <command>" docs/command-reference.md
```

---

## 7. Constraints & Non-Goals

### Constraints
- Do NOT add new commands to specd. This is a **subtraction and consolidation** exercise.
- Do NOT change exit code semantics (0, 1, 2, 3).
- Do NOT change the `.specd/` directory structure.
- Do NOT change EARS, design headers, or task schema formats.
- Do NOT remove `specd check`, `specd approve`, `specd verify`, or `specd task`.
- Every surviving command must have a clear, single-sentence description that fits in a cheat sheet.

### Non-Goals
- Rewriting the Go codebase (this is planning; implementation is future work).
- Changing the Brain/Pinky protocol semantics.
- Adding new validation gates.
- Supporting new agents or hosts.

---

## 8. Deliverables Checklist

Before finishing, ensure:

- [ ] `.specd/specs/` directory exists with 4–7 spec subdirectories.
- [ ] Each spec subdirectory contains `spec.md` (EARS requirements + 7-section design).
- [ ] Each spec subdirectory contains `tasks.md` (wave DAG with all 7 keys per task).
- [ ] `.specd/specs/progress.md` exists and tracks all waves and cross-spec dependencies.
- [ ] No `TODO` placeholders in any design section.
- [ ] Every task has a non-empty `verify` line (or `N/A` with justification for read-only roles).
- [ ] The command palette proposed in the specs reduces the active surface to **≤20 commands**.
- [ ] A `CHEATSHEET.md` is written at `.specd/specs/CHEATSHEET.md` showing the final minimal palette.

---

## 9. Example Task Entry (For Reference)

```markdown
## Wave 1
- [ ] T1 — Audit all commands in internal/core/commands.go
  - why: We cannot optimize what we have not measured
  - role: investigator
  - files: internal/core/commands.go, docs/command-reference.md
  - contract: Produce a CSV of command, category, usage frequency, overlap score
  - acceptance: CSV has one row per command with overlap analysis
  - verify: test -f .specd/specs/cmd-audit/audit.csv && wc -l .specd/specs/cmd-audit/audit.csv
  - depends: —
```

---

**Execute this mission now. Start by loading context, then author the specs in dependency order, updating `progress.md` as you complete each wave.**
