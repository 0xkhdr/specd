# Tasks — deep-review-phase5-decisions

> Add only real work. The optional columns beyond the six required ones may be omitted.
> Production rows declare full trace, risk, routing, context, capability, evidence, and edge-check intent.

| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity |
|---|---|---|---|---|---|---|---|---|---|
| ✅ T1 | scout | internal/core/commands.go | - | printf ok | R1.1 | R1, R1.1 | chore | low | standard |
| ✅ T2 | craftsman | .specd/specs/deep-review-phase5-decisions/state.json | T1 | bash -c 'grep -qi consolidat .specd/specs/deep-review-phase5-decisions/state.json' | R1.1 | R1, R1.1 | chore | medium | simple |
| ✅ T3 | craftsman | docs/observability.md | - | bash -c 'grep -qi -e prometheus -e otel docs/observability.md && grep -qi decision .specd/specs/deep-review-phase5-decisions/state.json' | R2.1, R2.2 | R2, R2.1, R2.2 | chore | low | simple |
| T4 | craftsman | docs/adapter-contract.md, docs/delivery-contract.md, docs/operating-model-contract.md, docs/telemetry-schema.md, docs/scale-envelope.md, docs/data-classification.md | - | bash -c 'for f in docs/adapter-contract.md docs/delivery-contract.md docs/operating-model-contract.md docs/telemetry-schema.md docs/scale-envelope.md docs/data-classification.md; do if ! grep -qi -e "driver:" -e "status: historical" "$f"; then exit 1; fi; done' | R3.1 | R3, R3.1 | chore | low | standard |
