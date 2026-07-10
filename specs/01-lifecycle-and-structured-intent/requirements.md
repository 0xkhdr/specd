# Requirements — lifecycle structured intent

## Scope

Requirements below define Domain 01 implementation. IDs stable. Criteria IDs stable.

### R1 — Structured requirements

- R1.1: When author submits requirements for approval, system shall parse stable requirement IDs, EARS clauses, criteria IDs, exclusions, edge/failure behavior, owner, priority, and risk.
- R1.2: When requirement ID, criterion ID, or required EARS clause is missing, duplicate, malformed, or conflicting, system shall fail approval with exact addressable findings.
- R1.3: When parser reads identical bytes repeatedly, system shall produce identical normalized records without changing author bytes.

### R2 — Design decision trace

- R2.1: When design is approved, system shall require resolvable requirement references, boundaries, interfaces, invariants, failure/integration modes, alternatives, chosen disposition, human owner, and digest.
- R2.2: When design reference is unknown or required decision metadata absent, system shall refuse design approval.

### R3 — Task trace and planning quality

- R3.1: When tasks are planned, system shall require each implementation task to declare requirement/design references, work kind, risk tier, required context, evidence classes, and negative/edge checks.
- R3.2: When approved requirement lacks implementing task, explicit deferred disposition, or required criterion coverage, system shall refuse execution approval.
- R3.3: When design declares external/integration boundary and production policy applies, system shall require error-path and integration evidence planning.

### R4 — Role and verify baseline

- R4.1: When task declares unknown role, system shall reject task plan.
- R4.2: When write task uses configured trivial verify command, system shall reject it; explicit read-only task may retain trivial verify.

### R5 — Amendments and freshness

- R5.1: When approved requirement/design change is recorded, system shall append amendment with affected IDs, rationale, before/after digests, and required rechecks.
- R5.2: When amendment affects downstream contract, system shall mark dependent approvals/evidence stale and block unsafe dispatch without moving lifecycle status backward.
- R5.3: When amendment does not affect record, system shall keep that record current.

### R6 — Legal guidance

- R6.1: When agent requests machine guidance, system shall return current phase, legal commands, blockers, required artifact, and human-only action separately.
- R6.2: When phase lacks execution task, system shall not suggest task context, task verify, or agent self-approval.

### R7 — Profiles and spikes

- R7.1: When profile is default, system shall retain explicit backward-compatible policy.
- R7.2: When profile is production, system shall require configured risk-proportionate criterion, review, integration, and negative-path evidence; policy digest shall pin judgment.
- R7.3: When spike records output, system shall bound scope/question/expiry and shall not let spike complete task or approve architecture.

### R8 — Lifecycle proof

- R8.1: When release binary operates fresh git fixture, system shall preserve approvals, CAS revisions, task frontier, evidence, coverage, and staleness across restart/concurrency failure cases.
- R8.2: When report runs, system shall deterministically expose requirement-to-evidence coverage, stale records, amendments, and escaped-defect links.

## Non-goals

- No LLM semantic gate.
- No deployment/runtime-agent platform.
- No history rewrite to simulate feedback loop.
- No mandatory ceremony beyond declared risk/profile policy.
