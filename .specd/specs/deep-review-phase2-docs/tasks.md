# Tasks — deep-review-phase2-docs

> Add only real work. The optional columns beyond the six required ones may be omitted.
> Production rows declare full trace, risk, routing, context, capability, evidence, and edge-check intent.

| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity |
|---|---|---|---|---|---|---|---|---|---|
| ✅ T1 | craftsman | tools/gendocs/main.go, docs/command-reference.md | - | bash -c 'go run ./tools/gendocs -check && go run ./tools/gendocs -check' | R1.1 | R1.1 | feature | low | standard |
| ✅ T2 | craftsman | scripts/docs-lint.sh, .github/workflows/ci.yml, CLAUDE.md, docs/CHEATSHEET.md | T1 | bash -c './scripts/docs-lint.sh && ! test -f docs/CHEATSHEET.md' | R1.2, R1.3 | R1.2, R1.3 | chore | low | simple |
| ✅ T3 | craftsman | CONTRIBUTING.md, scripts/ci-local.sh | - | bash -c 'test -x scripts/ci-local.sh && grep -q staticcheck scripts/ci-local.sh && grep -q ci-local CONTRIBUTING.md' | R2.1 | R2.1, R2.2 | chore | low | simple |
| ✅ T4 | validator | - | T1, T2, T3 | bash -c 'gofmt -l . && go vet ./... && go test ./... -race -count=1 && ./scripts/test-lint.sh && ./scripts/docs-lint.sh' | R1.2, R2.2 | R1, R2 | chore | low | standard |
