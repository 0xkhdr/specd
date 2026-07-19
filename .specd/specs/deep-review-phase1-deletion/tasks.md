# Tasks — deep-review-phase1-deletion

> Add only real work. The optional columns beyond the six required ones may be omitted.
> Production rows declare full trace, risk, routing, context, capability, evidence, and edge-check intent.

| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity |
|---|---|---|---|---|---|---|---|---|---|
| ✅ T1 | craftsman | internal/orchestration/a2a.go, internal/orchestration/a2a_test.go, internal/integration/orchestration_conformance_test.go, internal/mcp/parity_test.go, internal/cmd/e2e_test.go | - | go build ./... && go vet ./... && go test ./internal/orchestration/... -race -count=1 | R1.1 | R1, R1.1, R1.2 | chore | low | simple |
| ✅ T2 | craftsman | internal/adapter/runner.go, internal/adapter/feedback.go, internal/adapter/identity.go, internal/adapter/a2a.go, internal/adapter/*_test.go, internal/mcp/mapping_test.go | T1 | go build ./... && go vet ./... && go test ./internal/adapter/... ./internal/cmd/... ./internal/mcp/... -race -count=1 | R2.1 | R2, R2.1, R2.2 | chore | medium | standard |
| ✅ T3 | craftsman | internal/core/commands.go, internal/cmd/registry_test.go, docs/command-reference.md, docs/CHEATSHEET.md | - | bash -c 'go build -o specd . && go test ./internal/cmd ./internal/core -count=1 && (./specd triage; [ $? -eq 2 ])' | R3.1 | R3, R3.1 | chore | low | simple |
| ✅ T6 | craftsman | scripts/docs-lint.sh | T3 | ./scripts/docs-lint.sh | R3.1 | R3.1 | chore | low | simple |
| ✅ T4 | craftsman | .gitignore | - | test ! -f coverage.out | R4.1 | R4, R4.1 | chore | low | simple |
| ✅ T5 | validator | - | T1, T2, T3, T4, T6 | gofmt -l . && go vet ./... && go test ./... -race -count=1 && ./scripts/test-lint.sh && ./scripts/docs-lint.sh && ./scripts/regress-domains.sh | R1.1, R2.1, R3.1 | R1, R2, R3, R4 | chore | low | standard |
