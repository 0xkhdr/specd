# Prompt: Audit & Upgrade specd Documentation to Production Grade

You are a senior technical documentation engineer and developer-experience (DX) specialist. Your task is to analyze the documentation of the `specd` project (an agent-agnostic, spec-driven coding harness CLI) on the `optimization` branch at `https://github.com/0xkhdr/specd/tree/optimization` and identify every gap, inconsistency, and improvement opportunity needed to bring it to **enterprise production-grade** quality.

## Context: What specd Is

- A Go CLI (stdlib-only, zero external deps) that enforces a structured spec workflow for AI coding agents.
- Core philosophy: "The agent reasons. The harness enforces."
- Key concepts: phase-gated spec lifecycle (requirements → design → tasks → execute → verify → reflect), EARS requirements syntax, DAG-based wave execution, evidence-gated completion, validation gates (7 core + 3 opt-in), agent-agnostic design, deterministic reporting.
- The project ships templates via `go:embed`, has a custom CLI parser (no Cobra), and supports MCP, sandboxed verify, frontier dispatch, and cross-spec programs.

## Documentation Inventory to Analyze

Analyze **every** of the following files. Do not skip any.

**Root-level docs:**
1. `README.md` — Project overview, quick start, feature list
2. `AGENTS.md` — Guide for agents developing specd itself
3. `TESTING.md` — Deterministic test harness, coverage policy, CI matrix
4. `SECURITY.md` — Threat model, reporting, supported versions

**`docs/` directory:**
5. `docs/README.md` — Documentation index / navigation hub
6. `docs/concepts.md` — Philosophy, eight principles, architecture overview
7. `docs/user-guide.md` — Install, lifecycle, artifacts, execution, troubleshooting
8. `docs/command-reference.md` — Every command, flag, exit code, env var, config key
9. `docs/validation-gates.md` — What each gate checks (7 core + acceptance/scope/custom)
10. `docs/agent-integration.md` — Steering, roles, subagent modes, context, programs
11. `docs/custom-gates.md` — External custom-gate subprocess contract
12. `docs/spec-packs.md` — Declarative scaffold bundles (`specd init --pack`)
13. `docs/open-spec-format.md` — The versioned open spec format JSON Schema
14. `docs/github-action.md` — PR gates + deterministic summary comment
15. `docs/contributor-guide.md` — CLI architecture, concurrency model, extension recipes

## Audit Dimensions

For each document, evaluate against these **12 dimensions** and flag specific gaps with line/section references where possible:

### 1. Completeness & Coverage
- Are all features/commands/flags documented? Are there undocumented capabilities mentioned in code but missing from docs?
- Is the `open-spec-format.md` JSON Schema fully specified with all types, required fields, and nested structures?
- Are all error codes and edge cases covered?

### 2. Accuracy & Currency
- Does every code example, command snippet, and file path actually work on the `optimization` branch?
- Are referenced file paths (`internal/core/...`, `internal/cmd/...`) still correct?
- Is the `config.json` schema complete and current? Are default values accurate?
- Are there stale references to retired files (e.g., old `SPEC.md`)?

### 3. Consistency & Cross-References
- Do terms match across all docs? (e.g., "planning ratchet", "frontier dispatch", "evidence gate")
- Are cross-references between docs bidirectional and working?
- Is the status/phase mapping identical in `user-guide.md`, `concepts.md`, and `command-reference.md`?
- Do the 8 principles appear identically in `README.md` and `concepts.md`?

### 4. Clarity & Accessibility
- Is the writing clear for both human developers and AI agents reading it?
- Is jargon defined on first use? (e.g., EARS, DAG, CAS, MCP, bwrap)
- Are complex workflows (verify→complete, dispatch→subagent→verify) explained with visual diagrams or step-by-step flows?
- Is the reading level appropriate for an international engineering team?

### 5. Structure & Navigation
- Does `docs/README.md` serve as an effective hub? Are all docs discoverable from it?
- Are table of contents accurate and deep-linked?
- Is information duplicated unnecessarily across files? (e.g., security model appears in both `SECURITY.md` and `validation-gates.md`)
- Should some content be merged or split?

### 6. Code Examples & Runnable Snippets
- Are all shell commands copy-paste ready?
- Are example outputs shown where helpful?
- Are JSON examples valid and complete?
- Is there a "Getting Started" tutorial that actually walks through a full spec from `init` to `approve`?

### 7. Error Handling & Troubleshooting
- Is the troubleshooting table in `user-guide.md` comprehensive? What common errors are missing?
- Are there "What to do when..." guides for failure modes? (e.g., CAS conflict, lock timeout, verify sandbox failure, custom gate timeout)
- Is there a debug/verbose mode documented?

### 8. Security & Compliance Documentation
- Is the threat model in `SECURITY.md` complete? Are there missing attack vectors?
- Is the sandbox/isolation contract (`bwrap` vs `container` vs `none`) fully specified with configuration examples?
- Are secrets-handling and env-scrubbing documented clearly enough for a security audit?
- Is there a changelog or security advisory process documented?

### 9. API / Machine-Readable Contracts
- Is the `--json` output schema documented? What does `SPECD_JSON=1` return for each command?
- Is the `FrontierEvent` NDJSON/SSE structure specified?
- Is the custom gate stdin/stdout JSON contract complete with all fields, types, and edge cases?
- Is the MCP tool schema documented?

### 10. Configuration Reference
- Is `config.json` fully documented with all keys, types, defaults, and valid values?
- Are env vars (`SPECD_*`) all listed with type, range, and behavior?
- Are build tags (`specd_redis`, `specd_postgres`) documented with compilation instructions?

### 11. Visual & Diagram Quality
- Are ASCII diagrams readable and accurate? (e.g., foundational split, lifecycle flow, cross-spec DAG)
- Would Mermaid diagrams improve clarity?
- Are tables well-formatted and complete?

### 12. Production Readiness Signals
- Is there a versioning policy for the spec format schema?
- Is there a migration guide for schema changes?
- Is there a release notes / changelog section?
- Are there "Best Practices" or "Anti-patterns" guides?
- Is there performance guidance? (e.g., large spec performance, watch interval tuning)
- Is there a FAQ?

## Specific Gaps to Investigate (High Priority)

1. **`docs/open-spec-format.md`**: This file could not be retrieved. Verify it exists, is populated, and contains the full JSON Schema for `state.json`, `program.json`, and artifact formats. If missing or empty, this is a **critical** gap.

2. **`docs/README.md` hub**: Does it link to `open-spec-format.md`? Does it have a "Quick Start" path for different personas (first-time user, agent integrator, contributor)?

3. **Command Reference completeness**: Are `specd decision`, `specd midreq`, `specd memory`, `specd diff`, `specd replay`, `specd schema`, `specd validate`, `specd update`, `specd uninstall`, `specd version`, `specd help` all fully documented with every flag?

4. **Missing docs**: Are there any topics that deserve their own doc but don't have one? Consider:
   - `docs/migration-guide.md` — Schema version migrations
   - `docs/performance.md` — Large spec handling, tuning
   - `docs/faq.md` — Common questions
   - `docs/adr.md` — Architecture decision records (the `decisions.md` artifact)
   - `docs/telemetry.md` — Cost/token ledger, metrics
   - `docs/troubleshooting.md` — Expanded from the user-guide table

5. **Template documentation**: The `embed_templates/` directory contains `AGENTS.md`, steering files, roles, stubs, and skills. Are these templates documented so users know what gets scaffolded and how to customize them?

6. **Windows support**: `TESTING.md` mentions Windows is build-only in CI. Is this limitation documented in `user-guide.md` or `README.md` for Windows users?

7. **MCP integration**: `specd mcp` is mentioned but is there a dedicated guide for MCP client authors? What tools are exposed? What is the JSON-RPC contract?

8. **Steering skill**: The `specd-steering` skill is referenced in `user-guide.md` but never fully documented. What does it contain? How does an agent use it?

## Output Format

Produce a structured report with the following sections:

### Executive Summary
- Overall documentation maturity score (1-10)
- Top 5 critical gaps that block production adoption
- Top 5 high-impact improvements

### Per-Document Audit
For each of the 15 files, provide:
- **Status**: (Complete / Needs Minor Work / Needs Major Work / Missing-Critical)
- **Score**: 1-10
- **Gaps Found**: Bullet list with specific section/line references
- **Recommended Improvements**: Actionable, prioritized list
- **Cross-Reference Issues**: Links to other docs that contradict or duplicate

### Global Issues
- Inconsistencies found across multiple documents
- Missing documents that should exist
- Duplicated content that should be centralized
- Terminology drift

### Improvement Roadmap
Prioritized backlog of documentation tasks:
1. **P0 (Critical)**: Blockers for production use
2. **P1 (High)**: Significant DX improvement
3. **P2 (Medium)**: Polish and completeness
4. **P3 (Low)**: Nice-to-have enhancements

### Suggested New Documents
- Title, purpose, target audience, and outline for each proposed new doc

## Constraints
- Be ruthlessly honest. Production-grade means "a new senior engineer can onboard and operate specd without asking questions in Slack."
- Cite specific text from the docs when flagging issues.
- Suggest concrete rewrites or additions, not just "this could be better."
- Consider both human readers and AI agents consuming this documentation.
