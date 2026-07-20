# Audit — template-conformance T8

Scope: full diff for the template-conformance spec against acceptance R1.1, R2.3, R5.1
and the edge check "any gate relaxed rather than template corrected."

## Verdict: pass

- **R1.1** — Each shipped steering template (product/reasoning/structure/tech/workflow)
  carries a `specd-context` block (id/version/priority). `SelectSteering` over a freshly
  scaffolded root returns zero "missing explicit applicability metadata" omissions
  (`TestScaffoldedSteeringSelects`). memory.md stays excluded.
- **R2.3 / R5.1** — The conformance suite asserts each template against its real consumer:
  steering→SelectSteering (context pkg), requirements→ParseRequirements/ValidateRequirements,
  tasks→ParseQualityContract + ParseTasksMd, agent defs→host schema. A corrupted
  requirements template fails the suite (teeth verified).
- **No gate relaxed.** gates/core.go and gates/registry.go only *add* the warning-severity
  `steering-applicability` diagnostic and refine `task-trace` to name requirements.md on an
  empty ID set (still Error severity, no downgrade). No gate removed, no severity lowered,
  no bypass flag, no evidence manufactured. Every added check is a pure function of on-disk
  bytes; no LLM enters any gate path.
- Templates moved to the gates, not gates to the templates (design invariant upheld).

## Evidence
- `go test ./... -race -count=1` — pass
- `go test ./... -count=2` — pass (no iteration-order flakiness)
- gofmt, go vet, test-lint, docs-lint, regress-domains — all pass
