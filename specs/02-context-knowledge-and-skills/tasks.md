# Tasks — Domain 02 Context DAG

`[ ]` pending. Execute wave only after dependency evidence passes. Files are declared scope;
record deviation before edit. Cross-domain prerequisites live in `README.md`, not local DAG IDs.

## W0 — contract, fixtures, migration

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T01 | scout | docs/google-sdlc-alignment/02-context-knowledge-and-skills.md; specs/02-context-knowledge-and-skills | | printf ok | contract inventory/maps R1-R8 to code and external domains |
| [x] T02 | craftsman | internal/context/manifest_test.go; internal/context/steering_manifest_test.go; internal/cmd/lifecycle_test.go; internal/cmd/integration_polish_test.go | T01 | `go test ./internal/context ./internal/cmd -run 'Test(BuildManifest|SteeringInManifest|Lifecycle|Integration)'` | failing wrong-root/missing/overflow/stale/route fixture baseline R8 |
| [x] T03 | craftsman | internal/context/manifest.go; internal/context/manifest_test.go; docs/command-reference.md; docs/CHEATSHEET.md | T01 | go test ./internal/context -run TestManifest && ./scripts/docs-lint.sh | V1/V2 compatibility/render decision documented R1.3,R8.2 |

> **W0 deviations.** T02 (subtractive): the R8 baseline scenarios are all observable on the `context.Manifest` value, so the fixtures live in the two declared context-package test files (`manifest_test.go`, `steering_manifest_test.go`). The two declared cmd files (`internal/cmd/lifecycle_test.go`, `internal/cmd/integration_polish_test.go`) were not needed — their existing `TestLifecycleE2E`/handshake tests already cover the CLI surface; end-to-end route/stale baselines are deferred to the W6 conformance wave. No product code changed in W0.

## W1 — typed v2 foundation

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T04 | craftsman | internal/context/manifest.go; internal/context/manifest_test.go; internal/context/estimate.go; internal/context/estimate_test.go | T02,T03 | `go test ./internal/context -run 'TestManifest|TestEstimate'` | typed V2 schema, required fields, canonical ordering/digest R1 |
| [x] T05 | craftsman | internal/context/resolver.go; internal/context/resolver_test.go; internal/core/paths.go; internal/core/paths_test.go | T04 | `go test ./internal/context ./internal/core -run 'TestResolver|Test.*Path'` | canonical root, traversal/symlink/wrong-root refusal R2.2 |
| [x] T06 | craftsman | internal/context/manifest.go; internal/context/budget.go; internal/context/budget_test.go; internal/core/gates/contextbudget.go; internal/core/gates/contextbudget_test.go | T04,T05 | `go test ./internal/context ./internal/core/gates -run 'Test.*Budget|TestManifest'` | emitted-byte accounting; required overflow fails R3 |

> **W1 deviations.** T04 (subtractive): the typed V2 schema and canonical digest reuse the existing `EstimateText` estimator, so the declared `internal/context/estimate.go` / `estimate_test.go` were not modified. T06 (subtractive): the V2 budget (`EnforceBudgetV2`) is additive in `manifest.go`; the V1 `budget.go` gate and `contextbudget.go` were left unchanged. V2 is built and validated alongside V1 but is **not** yet the default renderer (per the W1 note below) — the W0 baselines still characterize the V1 default.

## W2 — required lanes and driver contract

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T07 | craftsman | internal/core/tasksparser.go; internal/core/tasksparser_test.go; internal/context/manifest.go; internal/context/manifest_test.go | T04 | `go test ./internal/core ./internal/context -run 'TestTasks|TestManifest'` | structured selected-task record, normalized declared files R2.1 |
| [x] T08 | craftsman | internal/context/selector.go; internal/context/selector_test.go; internal/context/manifest.go; internal/context/manifest_test.go | T05,T06,T07 | `go test ./internal/context -run 'Test(Selector|Manifest)'` | required requirements/design/role/source selection; named missing/selector findings R2 |
| [x] T09 | craftsman | internal/core/manifest_tools.go; internal/core/handshake.go; internal/core/handshake_test.go; internal/context/manifest.go; internal/context/manifest_test.go | T04 | `go test ./internal/core ./internal/context -run 'Test(Handshake|Manifest)'` | tool/guardrail lane contains route, authority, palette/config digest R4 |
| [x] T10 | craftsman | internal/cmd/registry.go; internal/cmd/integration_polish_test.go; internal/core/embed_templates/AGENTS.md; docs/mcp-guide.md | T08,T09 | `go test ./internal/cmd -run 'TestIntegration|TestLifecycle'` | bootstrap/context surfaces route and drift before mutable action R4.3,R8.2 |

> **W2 deviations.** T10 required `internal/context/manifest.go` beyond its declared list to
> assemble required selector and driver lanes into authoritative V2 output; `registry.go` only
> selects renderer and supplies effective config. No declared files were omitted.

## W3 — progressive static lanes

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T11 | craftsman | internal/context/steering.go; internal/context/steering_test.go; internal/context/manifest.go; internal/context/steering_manifest_test.go | T08 | `go test ./internal/context -run 'Test(Steering|Manifest)'` | deterministic steering tags/applicability/omissions R3,R6 |
| [x] T12 | craftsman | internal/core/memory.go; internal/core/memory_test.go; internal/context/memory.go; internal/context/memory_test.go | T08 | go test ./internal/core ./internal/context -run 'TestMemory' | stable block index/selector; critical ordering R6 |
| [x] T13 | craftsman | internal/context/examples.go; internal/context/examples_test.go; internal/context/manifest.go; internal/context/manifest_test.go | T08 | `go test ./internal/context -run 'Test(Examples|Manifest)'` | versioned applicable positive/negative examples R6 |

## W4 — portable skills

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T14 | craftsman | internal/context/skills.go; internal/context/skills_test.go; internal/core/scaffold.go; internal/core/scaffold_test.go | T04 | `go test ./internal/context ./internal/core -run 'Test(Skills|Scaffold)'` | `.specd/skills` package metadata/version/provenance validation R7.1 |
| [x] T15 | craftsman | internal/context/skills.go; internal/context/skills_test.go; internal/core/manifest_tools.go | T09,T14 | `go test ./internal/context ./internal/core -run 'Test(Skills|ManifestTools)'` | phase/role/capability subset and explicit unsupported result R7.2,R7.3 |
| [x] T16 | craftsman | internal/context/skills.go; internal/context/manifest.go; internal/context/skills_test.go; internal/context/manifest_test.go | T11,T13,T15 | `go test ./internal/context -run 'Test(Skills|Manifest)'` | progressive skill selection, refs, digest, budget R6,R7 |

## W5 — receipts and durable knowledge

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T17 | craftsman | internal/context/receipt.go; internal/context/receipt_test.go; internal/context/manifest.go; internal/context/manifest_test.go | T06,T08,T16 | `go test ./internal/context -run 'Test(Receipt|Manifest)'` | stable receipt; no raw content/secret field R5.1,R5.3 |
| [x] T18 | craftsman | internal/core/evidence.go; internal/core/evidence_test.go; internal/context/receipt.go; internal/context/receipt_test.go | T17 | `go test ./internal/core ./internal/context -run 'Test(Evidence|Receipt)'` | required digest change marks receipt stale; historical readable R5.2 |
| [x] T19 | craftsman | internal/cmd/memory.go; internal/cmd/memory_test.go; internal/core/memory.go; internal/core/memory_test.go; internal/context/memory.go; internal/context/memory_test.go | T12,T17 | `go test ./internal/cmd ./internal/core ./internal/context -run 'TestMemory|TestReceipt'` | evidence/review or exception provenance; expired/superseded excluded R6.3 |

## W6 — conformance and release proof

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T20 | craftsman | internal/context/manifest_test.go; internal/context/resolver_test.go; internal/context/budget_test.go; internal/cmd/lifecycle_test.go | T10,T18 | `go test ./internal/context ./internal/cmd -run 'Test(Manifest|Resolver|Budget|Lifecycle)'` | wrong-root, missing design/source, overflow, stale receipt black-box cases R8.1 |
| [x] T21 | craftsman | internal/context/skills_test.go; internal/context/memory_test.go; internal/cmd/integration_polish_test.go; internal/core/gates/security | T15,T19,T20 | `go test ./internal/context ./internal/cmd ./internal/core/gates/security -run 'Test(Skills|Memory|Integration)'` | injection label, route mismatch, portable skill, poisoned memory cases R8.1 |
| [x] T22 | craftsman | docs/open-spec-format.md; docs/mcp-guide.md; docs/command-reference.md; docs/CHEATSHEET.md; docs/contributor-guide.md | T20,T21 | ./scripts/docs-lint.sh && go test ./internal/context -run TestManifest | V2 migration/operator/host contract docs R8.2 |
| [x] T23 | craftsman | scripts/regress-domains.sh; scripts/regress-lint.sh; internal/cmd/e2e_test.go; internal/context/perf_test.go | T20,T21,T22 | `go test ./internal/cmd ./internal/context -run 'Test(LifecycleE2E|BuildManifestNoN1FileReads)' && ./scripts/regress-domains.sh && ./scripts/regress-lint.sh` | release binary conformance/perf regression R8 |
| [x] T24 | validator | specs/02-context-knowledge-and-skills; internal/context; internal/cmd; internal/core | T23 | go test ./... -race -count=1 && go vet ./... && ./scripts/test-lint.sh && ./scripts/docs-lint.sh && ./scripts/regress-all.sh && ./scripts/regress-domains.sh | full Domain 02 evidence |

## Cross-wave rules

- Add failing contract test before public schema/default change.
- Do not make V2 default until T03 compatibility decision and T20 migration fixtures pass.
- Do not let context receipt satisfy evidence. Domain 04 integration remains gate authority.
- Preserve byte-stable tasks parser, atomic writes, CAS state, no runtime dependencies, and
  `reference/` museum boundary.
