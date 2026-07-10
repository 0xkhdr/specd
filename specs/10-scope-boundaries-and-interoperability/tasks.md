# Tasks — Domain 10 boundary/interoperability DAG

`[ ]` pending. Execute a wave only after dependency evidence passes. Files are declared scope;
record deviation before edit. Cross-domain prerequisites live in `README.md` and
`../progress-plan.md`, not local DAG IDs.

## W0 — baseline and boundary invariant

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T01 | scout | docs/google-sdlc-alignment/10-scope-boundaries-and-interoperability.md; specs/10-scope-boundaries-and-interoperability | | printf ok | boundary inventory maps R1-R10 to code and to consuming domains 03-09 |
| [ ] T02 | craftsman | internal/adapter/import_guard_test.go; internal/adapter/doc.go | T01 | go test ./internal/adapter -run TestImportGuard | R1.1,R1.3 prohibited import in trusted core fails |
| [ ] T03 | craftsman | internal/adapter/import_guard_test.go; go.mod; go.sum | T02 | go test ./internal/adapter -run TestZeroDependency && go mod verify | R1.2 zero runtime dependency assertion |

## W1 — envelope, identity, classification

| id | role | files | depends-on | verify | acceptance |
| [ ] T04 | craftsman | internal/adapter/envelope.go; internal/adapter/envelope_test.go; internal/adapter/testdata | T02 | go test ./internal/adapter -run 'TestEnvelope' | R2.1,R2.3 versioned request/result golden round-trip |
| [ ] T05 | craftsman | internal/adapter/envelope.go; internal/adapter/envelope_test.go | T04 | go test ./internal/adapter -run 'TestEnvelopeReject|TestExitClass' | R2.2,R2.4 unknown version/field fail; stable status classes |
| [ ] T06 | craftsman | internal/adapter/identity.go; internal/adapter/identity_test.go | T04 | go test ./internal/adapter -run 'TestIdentity' | R3.1,R3.2,R3.3 mismatch rejected before gate; stale marked historical |
| [ ] T07 | craftsman | internal/adapter/classify.go; internal/adapter/classify_test.go; docs/data-classification.md | T04 | go test ./internal/adapter -run 'TestClassify' && ./scripts/docs-lint.sh | R4.1,R4.2,R4.3 restricted classes absent/redacted by default |
| [ ] T08 | craftsman | docs/google-sdlc-alignment/README.md; docs/adapter-contract.md; specs/10-scope-boundaries-and-interoperability/README.md | T01 | ./scripts/docs-lint.sh | R5.1 every integration item classified core/adapter/reference/external |

## W2 — runner and capability inspection

| id | role | files | depends-on | verify | acceptance |
| [ ] T09 | craftsman | internal/adapter/runner.go; internal/adapter/runner_test.go; internal/core/submit.go | T05,T06,T07 | go test ./internal/adapter ./internal/core -run 'TestRunner|TestSubmit' | R6.1,R6.2,R6.3 typed failing records; no unsafe fallback; secrets via env only |
| [ ] T10 | craftsman | internal/cmd/adapters.go; internal/cmd/adapters_test.go; internal/core/commands.go; internal/cmd/registry.go; docs/command-reference.md; docs/CHEATSHEET.md | T09 | go test ./internal/cmd ./internal/core -run 'TestAdapters|TestCommand' && ./scripts/docs-lint.sh | R7.1,R7.2 capability negotiation; read-only doctor; no secret load |

## W3 — offline continuity and conformance

| id | role | files | depends-on | verify | acceptance |
| [ ] T11 | craftsman | internal/cmd/e2e_test.go; internal/adapter/offline_test.go; internal/cmd/integration_polish_test.go | T09,T10 | go test ./internal/cmd ./internal/adapter -run 'TestOffline|TestProviderOutage|TestLifecycleE2E' | R8.1,R8.2 all-adapters-absent green; outage blocks with exact cause |
| [ ] T12 | craftsman | scripts/adapter-conformance.sh; internal/adapter/testdata; scripts/regress-domains.sh | T11 | ./scripts/adapter-conformance.sh && ./scripts/regress-domains.sh | R9.1,R9.2 third-party adapter certified without internal/ import |

## W4 — ecosystem mappings

| id | role | files | depends-on | verify | acceptance |
| [ ] T13 | craftsman | internal/adapter/a2a.go; internal/adapter/a2a_test.go; internal/mcp; internal/orchestration/acp.go | T06,T12 | go test ./internal/adapter ./internal/mcp ./internal/orchestration -run 'TestA2A|TestMCPMap' | R10.1 mission/tool round trip preserves authority/scope/evidence |
| [ ] T14 | craftsman | internal/adapter/otel_export.go; internal/adapter/otel_export_test.go; internal/cmd/report.go | T07,T12 | go test ./internal/adapter ./internal/cmd -run 'TestOTel|TestTraceExport' | R10.2 OTel correlation preserved; raw source/prompt absent |

## W5 — release/feedback contract and proof

| id | role | files | depends-on | verify | acceptance |
| [ ] T15 | craftsman | internal/adapter/feedback.go; internal/adapter/feedback_test.go; internal/core/program.go | T12 | go test ./internal/adapter ./internal/core -run 'TestFeedback|TestProgram' | R10.3 feedback links maintenance; cannot mutate completed history |
| [ ] T16 | craftsman | docs/adapter-contract.md; docs/command-reference.md; docs/CHEATSHEET.md; internal/adapter/version_test.go | T13,T14,T15 | go test ./internal/adapter -run 'TestSchemaVersion' && ./scripts/docs-lint.sh | R10.4 adapter-schema versioning/negotiation policy documented |
| [ ] T17 | validator | specs/10-scope-boundaries-and-interoperability; internal/adapter; internal/cmd | T16 | go test ./... -race -count=1 && go vet ./... && ./scripts/test-lint.sh && ./scripts/docs-lint.sh && ./scripts/regress-all.sh && ./scripts/regress-domains.sh | all R1-R10 release evidence |

## Cross-wave rules

- Add the failing contract/conformance test before public schema/default change.
- Freeze the `10c` common envelope only after Domains 04/05/07/08 P0 field demands are recorded
  in T01's inventory; payload extensions stay additive and domain-owned.
- Never let an adapter result satisfy a gate before identity, capability, and classification checks
  pass. Consuming-domain gates remain the completion authority.
- Preserve byte-stable parsers, atomic writes, CAS state, zero runtime dependencies, and the
  `reference/` museum boundary. Keep `docs/command-reference.md` and `docs/CHEATSHEET.md` identical.
