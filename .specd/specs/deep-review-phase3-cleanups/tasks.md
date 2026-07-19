# Tasks — deep-review-phase3-cleanups

> Add only real work. The optional columns beyond the six required ones may be omitted.
> Production rows declare full trace, risk, routing, context, capability, evidence, and edge-check intent.

| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity |
|---|---|---|---|---|---|---|---|---|---|
| T1 | craftsman | internal/core/config_validate.go, internal/cmd/dispatch.go, internal/core/authority.go, internal/core/driver.go | - | bash -c 'go vet ./... && go test ./internal/core ./internal/cmd -count=1 && ! grep -rn "func contains" internal/core internal/cmd' | R1.1 | R1.1 | refactor | low | simple |
| T2 | craftsman | internal/cmd/registry.go, internal/core/prometheus.go, internal/core/gates/intake.go | T1 | bash -c 'go vet ./... && go test ./internal/... -count=1 && ! grep -rn "func sortedKeys" internal/' | R1.2 | R1.2 | refactor | low | simple |
| T3 | craftsman | internal/cmd/registry.go | T2 | bash -c 'go build ./... && go test ./internal/cmd -race -count=1 && [ $(wc -l < internal/cmd/registry.go) -lt 500 ]' | R2.1, R2.2 | R2.1, R2.2 | refactor | medium | standard |
| T4 | validator | - | T1, T2, T3 | bash -c 'gofmt -l . && go vet ./... && go test ./... -race -count=1 && go test ./... -count=2 && ./scripts/test-lint.sh' | R1.1, R1.2, R2.2 | R1, R2 | chore | low | standard |
