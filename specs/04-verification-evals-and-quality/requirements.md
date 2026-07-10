# Requirements — Verification, evals, quality

## Scope

Domain 04 turns quality analysis into deterministic contracts. Preserve Go stdlib-only core,
append-only evidence, atomic/CAS state, byte-stable task parsing, human approval, current
`verify` no-bypass rule, offline gates, and explicit compatible migration.

### R1 — Evidence contract

- R1.1: System shall represent required proof as versioned `test`, `output_eval`,
  `trajectory_eval`, or `review` evidence class; command prose shall not infer class.
- R1.2: Every record shall bind `spec_slug`, `task_id`, run/attempt identity, subject revision,
  producer/version/config digest, verdict, actor/time, and tamper-evident artifact digest.
- R1.3: Dataset-backed/output/trajectory records shall additionally pin relevant check, dataset,
  rubric, output, and trace identifiers/digests. Unknown enum/version/required field fails closed.
- R1.4: Legacy `verify` files retain current meaning: test evidence only, never silently full eval.

### R2 — Task quality declaration

- R2.1: Task contract shall declare required evidence classes and stable check/eval ids without
  weakening non-empty `verify` for executable tasks.
- R2.2: Declaration/parser round trip shall preserve unchanged task bytes. Existing task files
  shall remain valid under documented compatibility rules.
- R2.3: Required evidence class/id shall not be satisfied by another class/id or free-form prose.

### R3 — Offline import, provenance, freshness

- R3.1: `specd` shall import/validate local adapter artifacts deterministically; no gate/report
  invokes provider, model, network, or scorer.
- R3.2: Malformed/truncated/duplicate/wrong-task/wrong-check/wrong-digest artifact shall produce
  stable ordered failure. Imported content cannot become proof before schema/policy validation.
- R3.3: Completion/check shall reject required evidence not current for subject revision and
  configured output/diff/dataset/rubric/trace digest. Historical record stays auditable.
- R3.4: Required deterministic test failure always blocks completion despite eval/review score,
  waiver, or later unrelated record.

### R4 — Observable trajectory

- R4.1: Trace shall contain ordered run-scoped observable events: sequence, tool/action identity,
  sanitized argument/result class, affected paths, time, actor, correlation. No hidden reasoning.
- R4.2: Duplicate run/event, non-monotonic sequence, secret/raw sensitive field, digest mismatch,
  missing mandatory step, or forbidden step shall fail deterministic trajectory policy.
- R4.3: Worker claim about changed files is not authority; policy consumes harness-derived/project
  evidence and Domain 05 identity where available.

### R5 — Risk coverage and verify quality

- R5.1: Production/risk-critical write task shall map every critical acceptance criterion to named
  required test/eval id and classify integration/error/concurrency/rollback risks when applicable.
- R5.2: Production verify-quality policy shall reject trivial/compile-only evidence where declared
  risk/coverage requires stronger check. Read-only task exception remains explicit and narrow.
- R5.3: Unknown id, unmapped critical criterion, threshold-less score policy, or prohibited waiver
  fails closed. Noncritical exception requires scoped, owned, expiring approval reference.

### R6 — Eval policy and governance

- R6.1: External adapter artifact may use code/human/heuristic/LM scorer; LM metadata records
  provider/model/prompt/sampling but remains optional and never gate execution dependency.
- R6.2: Dataset/rubric manifest shall have owner/version/digest, case identity/labels, critical-case
  rules, threshold/repetition/aggregation policy, redaction/source policy, and review state.
- R6.3: Dataset/rubric modification invalidates incompatible result. Fixed imported runs aggregate
  by pure documented function; insufficient samples is fail/insufficient, never pass.

### R7 — Quality context, reports, learning

- R7.1: Context packet shall expose compact quality contract: required ids/classes, critical-case
  summaries, rubric dimensions/thresholds, commands, refs/digests/freshness/labels; not raw corpus.
- R7.2: Report shall distinguish passed proof, missing proof, stale proof, score, and review; no
  prose/evidence class conflation.
- R7.3: Optional quality ledger shall keep redacted, append-only production failure taxonomy and
  immutable regression-promotion provenance. Ingestion validates redaction/policy first.

## Non-goals

- No model hosting, provider SDK, ML platform, network client, opaque score in core gate, or
  evaluator secret transport.
- No chain-of-thought capture, production corpus dump, generic “quality passed” bit, or eval
  waiver that fabricates required passing test evidence.
