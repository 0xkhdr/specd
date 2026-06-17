# Requirements — Regression: Agent-Harness Value (context engineering, perf, market-gap)

## Introduction
This capstone spec measures specd not as code but as a *product for agent harnesses*: is its
context engineering tight (token-efficient, low-noise outputs), is it fast, and where are the
gaps versus competing spec/agent tools? It depends on every component spec passing, then runs
a cross-cutting regression that quantifies value and surfaces the improvements specd must ship
to maximize market value. Value: a defensible, measured claim that specd is the best-value,
best-performing, most-compatible agentic spec substrate.

## Requirement 1 — Context-engineering efficiency
**User story:** As an agent consuming specd output, I want briefings and tool results to be
token-lean and high-signal, so that specd costs little context budget.

**Acceptance criteria:**
1. THE SYSTEM SHALL bound the token size of `context`, `next`, and `dispatch` briefings within a documented budget
2. WHEN output is for agents (`--json` / MCP) THE SYSTEM SHALL exclude human-only decoration
3. IF a briefing exceeds its budget THEN THE SYSTEM SHALL fail a regression test with the measured size

## Requirement 2 — Performance & determinism
**User story:** As a harness invoking specd thousands of times, I want fast, deterministic
commands, so that specd is not the bottleneck.

**Acceptance criteria:**
1. THE SYSTEM SHALL complete core read commands (status, next, program) within a documented latency budget on the sample repo
2. WHEN the same command runs twice on unchanged state THE SYSTEM SHALL produce identical output
3. THE SYSTEM SHALL expose benchmarks for hot paths (dag/frontier, render) under `go test -bench`

## Requirement 3 — Harness & host compatibility matrix
**User story:** As an integrator, I want a proven compatibility matrix across MCP hosts and
invocation modes, so that I know specd works in my stack.

**Acceptance criteria:**
1. THE SYSTEM SHALL document and test a compatibility matrix over {stdio, HTTP/SSE} × {five embedded hosts}
2. WHERE a host has a known limitation THE SYSTEM SHALL record it explicitly rather than imply full support
3. THE SYSTEM SHALL verify each documented integration path produces a working tool call

## Requirement 4 — Market-gap & improvement ledger
**User story:** As the product owner, I want a recorded, evidence-backed gap analysis, so that
no optimization to market value is left implicit.

**Acceptance criteria:**
1. THE SYSTEM SHALL produce a gap-analysis artifact comparing specd to the spec-driven/agent-tooling field
2. WHEN a gap is identified THE SYSTEM SHALL record it as a decision (ADR) and/or a memory item with a proposed remedy
3. IF a gap is reliability- or compatibility-critical THEN THE SYSTEM SHALL link it as a blocker or follow-up spec
4. THE SYSTEM SHALL conclude with a prioritized improvement ledger ranked by market-value impact
