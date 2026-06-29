# Spec — EARS Requirement → Test Traceability Audit (P2-1)

**Priority:** P2 · **Wave:** 3 (rolling) · **Domain:** requirements verification.

## Introduction

The hardening review sampled the highest-risk surfaces and lowest-coverage
packages; it did **not** re-derive every EARS requirement to a line of code (368
source files). A full requirement-to-test traceability audit is itself an action
item: for each spec, map every EARS acceptance criterion to the test(s) that
prove it, and flag any criterion with no covering test.

This is a rolling spec — executed one spec at a time, not a single big-bang pass.

## Current-state grounding

- All specs under `specs/**` carry EARS-style acceptance criteria ("SHALL ...").
- Many specs already encode criteria as "fails if future change regresses"
  guards — the pattern this audit verifies is complete.
- No traceability matrix exists today linking criterion → test.

## Requirements

### Requirement 1 — Per-spec traceability matrix
**User story:** As a maintainer, I want each EARS criterion mapped to a test, so
I know exactly which criteria are unproven.

**Acceptance criteria:**
1. For the audited spec, every acceptance criterion SHALL map to ≥1 test
   (file:test-name) OR be explicitly flagged as unproven.
2. The mapping SHALL be recorded (matrix in the spec's directory or a central
   index).

### Requirement 2 — Gaps become tracked tasks
**User story:** As a maintainer, I want unproven criteria turned into work items.

**Acceptance criteria:**
1. Each unproven criterion SHALL produce a follow-up task (a new test or a
   documented intentional non-test).
2. The audit for a spec SHALL be marked complete only when all criteria are
   mapped or explicitly waived.

### Requirement 3 — Rolling, one spec at a time
**User story:** As a maintainer, I want this done incrementally, so it does not
block other hardening work.

**Acceptance criteria:**
1. The audit SHALL proceed one spec per iteration; progress tracked here.
2. Highest-risk specs (resilience, mcp, config) SHALL be audited first.

## Design

- For each spec, extract all "SHALL" criteria; grep tests for covering assertions;
  record matrix rows `criterion → test | UNPROVEN`.
- File gaps as tasks (new tests) under the relevant existing or harden spec.
- Track audited specs in this spec's tasks/progress; order by risk.

## Out of scope

- Auto-generating the matrix (manual/grep-assisted is fine).
- Re-writing specs whose criteria are already fully guarded.

## Risks

- **Scope blow-up:** strictly one spec per iteration; do not batch.
