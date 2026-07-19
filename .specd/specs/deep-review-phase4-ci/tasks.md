# Tasks — deep-review-phase4-ci

> Add only real work. The optional columns beyond the six required ones may be omitted.
> Production rows declare full trace, risk, routing, context, capability, evidence, and edge-check intent.

| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity |
|---|---|---|---|---|---|---|---|---|---|
| ✅ T1 | craftsman | .github/workflows/ci.yml, .github/workflows/heavy.yml | - | bash -c 'test -f .github/workflows/heavy.yml && ! grep -q -e stress -e perf-gate -e count=2 .github/workflows/ci.yml && grep -q count=2 .github/workflows/heavy.yml' | R1.1, R1.2 | R1.1, R1.2, R1.3 | chore | medium | standard |
| ✅ T2 | craftsman | scripts/stress.sh, scripts/stress-acp.sh, scripts/stress-orchestration.sh, scripts/stress-program.sh, scripts/stress-brain-recovery.sh, scripts/stress-checkpoint-fault.sh | T1 | bash -c 'test -x scripts/stress.sh && ! ls scripts/stress-*.sh 2>/dev/null && (./scripts/stress.sh bogus-domain; [ $? -ne 0 ])' | R2.1, R2.2 | R2.1, R2.2 | refactor | low | standard |
| ✅ T3 | validator | - | T1, T2 | bash -c 'go build ./... && ./scripts/stress.sh acp && ! grep -rn "stress-" .github/workflows/' | R2.2 | R1, R2 | chore | low | standard |
