# Tasks — Deployment and production assurance

Waves map to the deliverable specs in `README.md`. Each task is one atomic craftsman/scout/validator
unit with a runnable `verify:` line. Delivery verbs/flags are declared once in
`internal/core/commands.go`, derived into MCP/help, and mirrored in `docs/command-reference.md` and
`docs/CHEATSHEET.md`. No LLM in any gate/transition/report; no network in core; no `reference/` edits.
`verify:` lines target this repo's tree; runtime paths under `.specd/specs/` are exercised in temp
trees by the smoke/regression scripts.

Legend: **role** ∈ {scout=read-only, craftsman=write+verify one task, validator=read-only run verify,
auditor=read-only audit diff}. **deps** are task IDs (and cross-domain notes). **req** cites
`requirements.md`.

## W0 — `08a-delivery-assurance-baseline` (requires —)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T01 | scout | `docs/google-sdlc-alignment/08-*.md`; `internal/core/handshake.go`; `internal/core/state.go`; `scripts/regress-domains.sh`; `scripts/install.sh` | — | `printf ok` | inventory current handshake/mode/installer/regress behavior for every P0 gap |
| T02 | craftsman | new `docs/delivery-contract.md`, `docs/open-spec-format.md` | T01 | `test -s docs/delivery-contract.md && grep -q release_id docs/delivery-contract.md` | R1.1 draft envelope/field/state definitions |
| T03 | craftsman | new `internal/core/delivery_fixtures_test.go`; `testdata/delivery/*.json` | T01 | `go test ./internal/core -run TestDeliveryFixture -count=1` (RED: assert-only skeleton) | R1.1,R1.2 failing fixtures for every P0 gap |
| T04 | craftsman | `internal/core/gates/regress`; `scripts/regress-domains.sh` | T01 | `bash scripts/regress-domains.sh; test $? -ne 0 || grep -q 'input absent' <(bash scripts/regress-domains.sh)` | R4.2 reproduce W0 fail-open |
| T05 | auditor | domain README/requirements/design vs `08-*.md` | T02 | `printf ok` | confirm 15 scenarios each have a planned fixture |

> **08a/T04 deviation:** the fail-open reproduction is shell-only, so `internal/core/gates/regress`
> (a listed file) was **not** created — a Go gate package there is the 08e/T22 *fix*, not the W0
> *reproduction*. Subtractive bias: no empty package added.

## W1 — `08b-agent-bootstrap-binding` (requires 08a)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| [x] T06 | craftsman | `internal/core/managed.go`; new `internal/core/managed_digest.go` | T01 | `go test ./internal/core -run TestManagedDigest -count=1` | R2.3 managed role/steering content digest |
| [x] T07 | craftsman | `internal/core/handshake.go`; `internal/version/version.go` | T06 | `go test ./internal/core -run TestHandshakeBind -count=1` | R2.1 one packet binds all identities |
| [x] T08 | craftsman | `internal/core/handshake.go`; `internal/cmd/registry.go` | T07 | `go test ./internal/cmd -run TestHandshakeMismatchExits -count=1` | R2.2 pinned mismatch exits non-zero pre-mutation |
| [x] T09 | craftsman | `internal/core/handshake.go` (typed source fields) | T07 | `go test ./internal/core -run TestHandshakeTypedSources -count=1` | R2.3 harness vs untrusted separation |
| [x] T10 | craftsman | `docs/mcp-guide.md`; managed `AGENTS.md` template | T08 | `./scripts/docs-lint.sh` | R2.1 document bootstrap packet |

> **08b scope deviations:** TDD requires `internal/core/handshake_test.go` and
> `internal/cmd/integration_polish_test.go`; T08's public expectation flags require their canonical
> declaration in `internal/core/commands.go`; the CLI-surface invariant requires mirrored
> `docs/command-reference.md` and `docs/CHEATSHEET.md` updates. Template-version bump exposed a
> hardcoded marker assertion, so backprop V9/B1 updates `internal/core/managed_test.go` and
> `SPEC.md`.

## W2 — `08c-orchestrated-mode-reachability` (requires 08a, Domain 05 dispatch)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| [x] T11 | craftsman | `internal/core/state.go` (declared `orchestrated` mode + schema validation) | T01 | `go test ./internal/core -run TestModeSchema -count=1` | R3.1 orchestrated is a validated mode |
| [x] T12 | craftsman | `internal/cmd/lifecycle.go`; `internal/core/config_*` | T11 | `go test ./internal/cmd -run TestEnterOrchestratedCAS -count=1` | R3.1 CAS/approval transition, no hand-edit |
| [x] T13 | craftsman | `internal/cmd/brain_run.go`; orchestration `Sense` | T11 | `go test ./internal/orchestration -run TestSenseCost -count=1` | R3.2 populate cost, wire brake |
| [x] T14 | craftsman | `internal/orchestration/decide.go` (fail-closed on missing trusted telemetry) | T13 | `go test ./internal/orchestration -run TestBrakeUntrusted -count=1` | R3.3 fail closed + labeled |
| [x] T15 | validator | grep tests/guides for `state.json` hand-edit | T12 | `! grep -rn 'orchestrated' --include=*_test.go internal | grep -q 'state.json'` | R3.1 no forbidden mutation path |

> **08c scope deviations:** strict TDD requires `internal/core/state_test.go`,
> `internal/cmd/lifecycle_test.go`, `internal/orchestration/sense_test.go`, and
> `internal/orchestration/brakes_test.go`. The public human-only approval surface is declared in
> `internal/core/commands.go`, so its usage/docs metadata is updated with mirrored
> `docs/command-reference.md` and `docs/CHEATSHEET.md` text. Mode validation exposed a stale v1
> migration fixture using never-declared mode `build`; backprop B2 updates `SPEC.md` and
> `internal/core/state_test.go` to use declared legacy mode `agent`.

## W3 — `08d-delivery-envelopes-and-state-machine` (requires 08a)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| [x] T16 | craftsman | new `internal/core/delivery.go` (envelope structs, versioned) | T03 | `go test ./internal/core -run TestDeliveryEnvelope -count=1` | R1.1 release/env/deploy/health/rollback envelopes |
| [x] T17 | craftsman | `internal/core/delivery.go` (closed status set + transition table) | T16 | `go test ./internal/core -run TestDeliveryTransition -count=1` | R1.2 every transition fails closed |
| [x] T18 | craftsman | `internal/core/delivery.go`; fixtures | T16 | `go test ./internal/core -run TestDeliveryFixture -count=2` (GREEN) | R1.1,R1.2 offline fixtures validate; jumps/mismatch rejected |
| [x] T19 | validator | evidence gate with delivery structs present | T17 | `go test ./internal/core/gates -run TestEvidenceAdditive -count=1` | R1.3 delivery is additive, no gate crossover |

> **08d scope deviations:** strict TDD requires new `internal/core/delivery_test.go` and
> `internal/core/gates/delivery_additive_test.go`. T18 extends the predeclared
> `internal/core/delivery_fixtures_test.go` to exercise typed validation rather than JSON shape only.
> Backprop protocol records the T16 compile-time test typo in root `SPEC.md` §B3; no new invariant
> applies to the one-off local-identifier error.

## W4 — `08e-installed-lifecycle-e2e-and-regression-prereqs` (requires 08b)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| [x] T20 | craftsman | new `scripts/production-smoke.sh` | T08 | `bash scripts/production-smoke.sh` | R4.1 empty repo → full documented lifecycle via CLI |
| [x] T21 | craftsman | `scripts/production-smoke.sh` (deliberately invalid step) | T20 | `bash scripts/production-smoke.sh --negative; test $? -eq 0` | R4.1 invalid step fails closed w/ next action |
| [x] T22 | craftsman | `scripts/regress-domains.sh` (prove input exists) | T04 | `bash scripts/regress-domains.sh` | R4.2 absent input → fail/skip, never pass |
| [x] T23 | craftsman | `.github/workflows/ci.yml`; `internal/integration` | T20 | `go test ./internal/integration -run TestProductionSmokeLane -count=1` | R4.1 CI lane wired |
| [x] T24 | craftsman | `TESTING.md` | T22 | `./scripts/docs-lint.sh` | R4.2 declared skip policy documented |

## W5 — `08f-release-install-upgrade-hardening` (requires 08b)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T25 | craftsman | `scripts/install.sh` (staged temp path + atomic rename) | T01 | `bash scripts/install-scripts-test.sh` | R5.2 staged atomic swap |
| T26 | craftsman | `scripts/install.sh` (retain previous binary, restore on fail) | T25 | `bash scripts/install-scripts-test.sh` | R5.2 rollback-on-failed-smoke |
| T27 | craftsman | new `scripts/release-smoke.sh` | T25 | `bash scripts/release-smoke.sh` | R5.1 checksum/attestation + version commit + handshake smoke |
| T28 | craftsman | `internal/core/state.go` (schema preflight) | T11 | `go test ./internal/core -run TestSchemaPreflight -count=1` | R5.3 future schema/unsafe downgrade fail before write |
| T29 | craftsman | `.github/workflows/release.yml`; `.goreleaser.yml` | T27 | `printf ok` (workflow-lint offline) | R5.1 install real just-built archive per OS/arch |
| T30 | craftsman | `scripts/install.sh` (managed-asset diff preview) | T26 | `bash scripts/install-scripts-test.sh` | R5.2 preview managed changes |

## W6 — `08g-release-and-deployment-ledgers` (requires 08d)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T31 | craftsman | new `internal/core/delivery_ledger.go` | T18 | `go test ./internal/core -run TestDeliveryLedger -count=2` | R6.2 append/replay, crash-safe torn line |
| T32 | craftsman | `internal/core/commands.go`; new `internal/cmd/release.go` | T31 | `go test ./internal/cmd -run TestReleaseCandidate -count=1` | R6.1 immutable reproducible candidate, no build/upload |
| T33 | craftsman | `internal/core/commands.go`; new `internal/cmd/deploy.go` | T31 | `go test ./internal/cmd -run TestDeployAppend -count=1` | R6.2 attempt monotonic under spec lock |
| T34 | validator | evidence gate with ledgers present/absent | T31 | `go test ./internal/core/gates -run TestEvidenceLedgerNeutral -count=1` | R6.3 no retroactive complete change |
| T35 | craftsman | `docs/command-reference.md`; `docs/CHEATSHEET.md` | T32,T33 | `./scripts/docs-lint.sh` | R6.1 mirror new verbs |

## W7 — `08h-environment-policy-and-delivery-gates` (requires 08d,08g, Domain 06 authority)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T36 | craftsman | `internal/core/config_loader.go`; `internal/core/config_validate.go`; `project.yml` template | T31 | `go test ./internal/core -run TestEnvPolicy -count=1` | R7.1 closed env → strategy/approver/criteria/window/freshness/rollback |
| T37 | craftsman | new `internal/core/gates/delivery.go` | T36 | `go test ./internal/core/gates -run TestDeliveryGate -count=2` | R7.1 same policy+evidence → same verdict |
| T38 | craftsman | `internal/core/gates/delivery.go` (production requirements) | T37,T36 | `go test ./internal/core/gates -run TestProductionRequires -count=1` | R7.2 prod needs adapter/authority/artifact/freshness/rollback |
| T39 | craftsman | `internal/core/gates/delivery.go` (artifact digest check) | T37 | `go test ./internal/core/gates -run TestArtifactSubstitution -count=1` | R7.3 swapped artifact fails digest |

## W8 — `08i-deployment-adapter-envelope` (requires 08g, Domain 10 adapter)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T40 | craftsman | new `internal/core/adapter_envelope.go`; `docs/adapters/deployment.md` | T33 | `go test ./internal/core -run TestAdapterEnvelope -count=1` | R8.1 stdin/file, no implicit creds, zero new deps |
| T41 | craftsman | `internal/core/adapter_envelope.go` (idempotency) | T40 | `go test ./internal/core -run TestIdempotencyKey -count=1` | R8.2 duplicate key = no-op/conflict |
| T42 | craftsman | `internal/core/adapter_envelope.go` (reject malformed/untrusted) | T40 | `go test ./internal/core -run TestEnvelopeReject -count=1` | R8.3 hostile prose/cred stored bounded, never instruction |
| T43 | validator | core deps unchanged | T40 | `go mod tidy && git diff --exit-code go.mod go.sum` | R8.1 zero new dependencies/network |

## W9 — `08j-canary-health-promotion-rollback` (requires 08h,08i, Domain 07 measurement)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T44 | craftsman | `internal/core/delivery.go` (canary observation window) | T38,T40 | `go test ./internal/core -run TestCanaryWindow -count=1` | R9.1 observing until full fresh window, exact artifact/env |
| T45 | craftsman | `internal/core/gates/delivery.go` (freshness/staleness) | T44 | `go test ./internal/core/gates -run TestObservationStale -count=1` | R9.1 missing/stale/wrong-release fails, never healthy-by-timeout |
| T46 | craftsman | `internal/cmd/deploy.go` (promotion records baseline+refs) | T44 | `go test ./internal/cmd -run TestPromote -count=1` | R9.2 promotion or governed exception |
| T47 | craftsman | `internal/core/delivery.go` (rollback record + post-health) | T44 | `go test ./internal/core -run TestRollbackComplete -count=1` | R9.3 complete only after target health; capability class |
| T48 | craftsman | `internal/cmd/report.go`; `internal/core/prometheus.go` | T46,T47 | `go test ./internal/cmd -run TestDeliveryReportStable -count=2` | R9.3 repeated reports byte-identical; label source separately |

## W10 — `08k-ci-delivery-binding-and-attestation` (requires 08i, Domain 10 adapter)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T49 | craftsman | new `internal/core/attestation.go` | T40 | `go test ./internal/core -run TestAttestationOffline -count=1` | R10.3 offline stdlib-crypto verify; tamper/expiry/audience fail |
| T50 | craftsman | new `.github/actions/specd-delivery`; `docs/github-action.md` | T39,T49 | `./scripts/docs-lint.sh` | R10.1 bind source evidence → artifact/env |
| T51 | validator | artifact-swap + fork-PR fixtures | T50 | `go test ./internal/core -run TestCIDeliveryBinding -count=1` | R10.1,R10.2 swap fails digest; fork gets no prod creds |

## W11 — `08l-incident-portfolio-and-recovery-drills` (requires 08j,08k, Domain 09 maintenance)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T52 | craftsman | new `internal/core/incident.go`; `internal/cmd/incident.go`; scaffold templates | T48 | `go test ./internal/cmd -run TestIncidentSeed -count=1` | R11.1 bounded refs seed spec; raw payload not loaded; ledgers immutable |
| T53 | craftsman | `internal/core/program.go`; status/report renderers | T48 | `go test ./internal/core -run TestPortfolioView -count=2` | R11.2 per-env release view, no network |
| T54 | craftsman | new `scripts/upgrade-matrix.sh`; scheduled workflow; versioned fixtures | T28 | `bash scripts/upgrade-matrix.sh` | R11.3 N-1→N preserves state/evidence; downgrade/future-schema fail |
| T55 | craftsman | crash-boundary drill in `scripts/upgrade-matrix.sh` | T54 | `bash scripts/upgrade-matrix.sh --crash-drill` | R11.3 crash at each swap/checkpoint recovers old or new complete |
| T56 | validator | 15 production validation scenarios end to end | T51,T52,T53,T55 | `go test ./... -race -count=1 && go test ./... -count=2` | release proof: all scenarios pass offline; full suite green |

## Cross-wave rules

- Every craftsman task is one atomic unit with an exit-0 git-pinned `verify:` record; read-only tasks
  carry a trivially-passing line (`printf ok`). No bypass flag.
- RED fixtures (T03) land before their GREEN implementation (T18); the wave is not done until GREEN.
- Any wave touching a verb/flag updates `internal/core/commands.go` once and mirrors
  `docs/command-reference.md` + `docs/CHEATSHEET.md` (`docs-lint.sh`), and keeps high-risk production
  mutations out of the general MCP palette.
- `go test ./... -race -count=1` and `-count=2` (iteration-order) gate every wave; `gofmt -l .`,
  `go vet ./...`, and `go mod tidy` must stay clean.
- No wave adds a runtime dependency, a network call in core, an LLM to a gate/transition/report, a
  new lifecycle status value, or a `reference/` edit.
