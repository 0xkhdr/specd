# Tasks — Domain 06 Security DAG

`[ ]` pending. Execute wave after dependencies pass. Touch declared files only; record deviation.
Cross-domain prerequisites remain README links, not local task ids. Add a failing public-contract
fixture before each enforcement change. Stdlib-only; no `reference/` edits; no LLM in any gate.

## W0 — inventory, wording, contract baseline

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T01 | scout | docs/google-sdlc-alignment/README.md; docs/google-sdlc-alignment/06-security-permissions-and-governance.md; specs/06-security-permissions-and-governance | | printf ok | map R1-R8 to config_loader/gates/security/verify/roles/dispatch/mcp/allowlist surfaces and Domain 01/02/04/05/07/10 boundaries |
| [x] T02 | craftsman | internal/core/gates/security/gate_test.go; internal/core/gates/security/scanner_test.go; internal/core/verify/sandbox_test.go; internal/cmd/submit_test.go; internal/cmd/dispatch_test.go | T01 | go test ./internal/core/gates/security ./internal/core/verify ./internal/cmd -run 'Test(Gate|Scanner|Sandbox|Submit|Dispatch)' | failing fixtures: opt-in security excluded from submit, role fallback, unenforced scope, `.specd` excluded, fail-open enumeration, unsandboxed verify R1-R5 |
| [x] T03 | craftsman | SECURITY.md; docs/agent-integration.md; docs/command-reference.md; docs/CHEATSHEET.md | T01 | ./scripts/docs-lint.sh | correct "convention + gates" wording; state real coverage: roles grant no capability, declared files not diff-compared, injection scan skips `.specd`; name production migration route R1 |

> **W0 deviations.** Inventory maps R1 config/security registry, R2 tasks/diff, R3 roles/dispatch/MCP,
> R4 scanner/context, R5 verify, R6 dependency policy, R7 allowlist/audit, and R8 adapters, with
> 01/02/04/05/07/10 boundaries. Existing gate/scanner/sandbox and command tests already characterize
> opt-in, `.specd` exclusion, role/scope, enumeration, and unsandboxed gaps; no duplicate test files
> or fixtures were needed. Documentation changes state current limits and migration route.

## W1 — operating profiles and required gates

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T04 | craftsman | internal/core/config_loader.go; internal/core/config_validate.go; internal/core/config_test.go; internal/core/embed_templates/project.yml | T02 | go test ./internal/core -run 'TestConfig' | versioned `security_profile` parse/validate; prototype/production severity map; invalid/absent profile fails closed; safe defaults preserve current behavior R1.1,R1.4 |
| [x] T05 | craftsman | internal/core/gates/security/policy.go; internal/core/gates/security/policy_test.go; internal/core/config_loader.go | T04 | go test ./internal/core/gates/security ./internal/core -run 'Test(Policy|Config)' | canonical policy serialization → stable `policy_version`/`policy_digest`; same inputs identical digest R1.3 |
| [x] T06 | craftsman | internal/core/gates/registry.go; internal/core/gates/registry_test.go; internal/core/gates/security/gate.go; internal/core/gates/security/gate_test.go | T05 | go test ./internal/core/gates ./internal/core/gates/security -run 'Test(Registry|Gate)' | production registry includes required security gates; prototype behavior explicit R1.1,R1.2 |
| [x] T07 | craftsman | internal/cmd/submit.go; internal/cmd/submit_test.go; internal/cmd/registry.go | T06 | go test ./internal/cmd -run 'Test(Submit|Check|Record)' | production submit refuses stale/absent security with `security_evidence_stale` + exact next command; digest pinned R1.2,R1.3 |

> **W1 deviations.** Stable core gate order lives in `gates/core.go`, so `CoreRegistryWith` was
> added there rather than the declarative `registry.go`; production callers append the security
> gate while prototype retains the legacy registry. Existing security gate behavior needed no
> edit: resolved production policy raises every configured scanner/policy severity to error.
> Approval also consumes the required registry so production enforcement is not submit-only.

## W2 — declared scope from harness diff

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T08 | craftsman | internal/core/tasksparser.go; internal/core/tasksparser_test.go | T02 | go test ./internal/core -run 'TestTasks' | normalized declared read/write path/glob + explicit test paths byte-stable; `../`/absolute rejected R2.1,R2.4 |
| [x] T09 | craftsman | internal/core/scope/diff.go; internal/core/scope/diff_test.go; internal/core/scope/normalize.go; internal/core/scope/normalize_test.go | T08 | go test ./internal/core/scope -run 'Test(Diff|Normalize)' | derive tracked/staged/untracked/delete/rename/mode/symlink from pinned baseline; symlink/submodule escapes fail R2.2 |
| [x] T10 | craftsman | internal/core/gates/scope.go; internal/core/gates/scope_test.go; internal/core/gates/registry.go | T06,T09 | go test ./internal/core/gates -run 'Test(Scope|Registry)' | out-of-scope change fails completion even when verify passes; worker claim is audit hint only R2.3 |
| [x] T11 | craftsman | internal/cmd/brain_worker.go; internal/cmd/brain_report_test.go; internal/core/task_complete.go; internal/core/task_complete_test.go | T10, Domain 05 report | go test ./internal/cmd ./internal/core -run 'Test(BrainReport|Complete|Scope)' | completion/report path invokes scope gate; worker-reported paths cannot override derived set R2.2,R2.3 |

> **W2 deviations.** `core.DeriveDiff` remains a compatibility wrapper over the new scope package.
> Scope needs repository/session state, so production completion invokes it in `cmd/lifecycle.go`
> before pure `core.CompleteTask`; the latter remains evidence-only and disk-free. Existing worker
> evidence checks and registry composition needed no edits. Brain report and normal completion both
> use mission-pinned baseline plus harness-derived paths; worker claims never replace that set.

## W3 — role authority packets and tool policy

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T12 | craftsman | internal/core/roles.go; internal/core/roles_test.go; internal/core/gates/core.go; internal/core/gates/core_test.go | T04 | go test ./internal/core ./internal/core/gates -run 'Test(Role|Core)' | role gate accepts only 4 documented roles; no craftsman fallback for unknown/auditor mode R3.1 |
| [x] T13 | craftsman | internal/core/authority.go; internal/core/authority_test.go; internal/core/embed_templates/roles; internal/context/manifest.go; internal/context/manifest_test.go | T12, Domain 05 mission | go test ./internal/core ./internal/context -run 'Test(Authority|Manifest)' | machine-readable AuthorityV1 packet: tools/paths/net/sandbox/expiry/digest; stale/expired/wrong-phase fails R3.2,R3.4 |
| [x] T14 | craftsman | internal/cmd/dispatch.go; internal/cmd/dispatch_test.go; internal/mcp/server.go; internal/mcp/server_test.go; internal/orchestration/authority.go | T13 | go test ./internal/cmd ./internal/mcp ./internal/orchestration -run 'Test(Dispatch|MCP|Authority)' | scout/validator/auditor write denied + sanitized denial event; craftsman out-of-path denied; unknown tool default-deny production R3.3 |

> **W3 deviations.** Existing core role gate already accepted exactly four embedded roles, so only
> fallback prompt/mode paths changed. Authority must travel with the actual claimed lease; therefore
> `orchestration/lease.go`, `worker.go`, and their tests also changed. CLI authorized dispatch writes
> bounded identity/tool/code denial records without arguments. Validator `verify` and auditor
> `report` remain allowed evidence operations; neither grants product-file write authority.

## W4 — context and change-boundary scan

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T15 | craftsman | internal/core/gates/security/scanner.go; internal/core/gates/security/scanner_test.go; internal/core/gates/security/testdata | T06 | go test ./internal/core/gates/security -run 'Test(Scanner|Input)' | ScanInputV1 abstraction; per-scanner explicit exclusion; failed enumeration/read → error finding not empty-green R4.2 |
| [x] T16 | craftsman | internal/core/gates/security/gate.go; internal/core/gates/security/gate_test.go; internal/core/gates/security/injection.go; internal/core/gates/security/injection_test.go | T15 | go test ./internal/core/gates/security -run 'Test(Gate|Injection)' | scan `.specd/` specs/steering/roles/memory + untracked pending set; injection marker under runtime spec found pre-dispatch R4.1 |
| [x] T17 | craftsman | internal/context/manifest.go; internal/context/manifest_test.go; internal/core/gates/security/secrets.go; internal/core/gates/security/secrets_test.go | T16, Domain 02 context | go test ./internal/context ./internal/core/gates/security -run 'Test(Manifest|Secrets)' | trust label/digest per item; untrusted text cannot alter gate behavior; findings bounded safe excerpts, no inlined secret/payload R4.3,R4.4 |

> **W4 deviations.** T15's scanner-interface migration also updates the three existing scanner
> implementations and `gate.go`; otherwise `ScanInputV1` cannot become the enforced input contract.
> T17 also updates `internal/context/selector.go` and its test because that is where manifest items
> receive source digests and boundary trust labels. No task checkbox is complete before user review.

## W5 — mandatory sandbox and secret isolation

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T18 | craftsman | internal/core/verify/exec.go; internal/core/verify/sandbox_test.go; internal/core/config_loader.go | T04 | go test ./internal/core/verify ./internal/core -run 'Test(Sandbox|Exec|Config)' | production verify requires sandbox before shell; missing binary/adapter fails closed; network off; no silent fallback R5.1 |
| [x] T19 | craftsman | internal/core/verify/exec.go; internal/core/verify/sandbox_test.go | T18 | go test ./internal/core/verify -run 'Test(Sandbox|Env|Cred)' | synthetic/empty HOME, minimal env, host credential path hidden (not just read-only); repo/temp writes only R5.1,R5.2 |
| [x] T20 | craftsman | internal/core/verify/redact.go; internal/core/verify/redact_test.go; internal/core/verify/exec.go; internal/core/evidence.go; internal/core/evidence_test.go | T19 | go test ./internal/core/verify ./internal/core -run 'Test(Redact|Exec|Evidence)' | central redaction on stdout/stderr/evidence; secret fixture never appears in full R5.3 |
| [x] T21 | craftsman | internal/core/verify/limits.go; internal/core/verify/limits_test.go; internal/core/verify/exec.go; internal/core/verify/timeout_test.go | T18 | go test ./internal/core/verify -run 'Test(Limits|Timeout|Exec)' | CPU/memory/process/output/wall/fs-growth limits in sandbox args; breach records failure not silent kill R5.4 |

> **W5 deviation.** T18 also updates `internal/cmd/registry.go` and its verify tests because
> production-profile sandbox enforcement must be selected at the CLI boundary that loads project
> config; `verify/exec.go` cannot infer project policy without breaking layer ownership.

## W6 — dependency and dangerous-change governance

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T22 | craftsman | internal/core/gates/security/slopsquat.go; internal/core/gates/security/manifest.go; internal/core/gates/security/manifest_test.go | T10,T15 | go test ./internal/core/gates/security -run 'Test(Slopsquat|Manifest)' | manifest-diff: new dep needs reason/source; unknown registry/checksum/provenance fails per profile; lockfile-only inspected R6.1 |
| [x] T23 | craftsman | internal/core/gates/security/depevidence.go; internal/core/gates/security/depevidence_test.go; internal/core/gates/security/testdata; scripts/dep-evidence.sh | T22 | go test ./internal/core/gates/security -run 'TestDepEvidence' | pinned offline adapter artifact validated; malformed/stale fails; gate stays offline/stdlib-only R6.2 |
| [x] T24 | craftsman | internal/core/gates/security/dangerous.go; internal/core/gates/security/dangerous_test.go; internal/core/gates/security/testdata | T10,T16 | go test ./internal/core/gates/security -run 'TestDangerous' | detect destructive shell, world-writable/exec mode, authz change, generated secret file, symlink escape over normalized diff; documented false-positive controls R6.3 |

> **W6 deviations.** New governance detectors are pure functions in the `security` package taking
> their profile/policy as parameters (not read from `SecurityConfig`, which is W1's file territory
> and outside this wave's declared files), so W6 stays self-contained and additive; wiring into the
> production `Analyze`/report path is deferred to W7/W8 (T27, T30) per the DAG. `slopsquat.go` was
> unchanged — `manifest.go` reuses its `parseGoMod` helper in-package. New findings resolve severity
> per profile (production → error, else warn); generated-secret and symlink-escape always fail closed
> at error. `ManifestDigest` matches `cat go.mod go.sum | sha256sum` so `scripts/dep-evidence.sh`
> (offline, empty-findings baseline adapter) and the gate pin the same manifest state.

## W7 — governed exceptions and mission audit

Scope deviation (T26): public approve/revoke actions necessarily update
`docs/command-reference.md` and `docs/CHEATSHEET.md`; mirrored-doc gates fail without these
contract projections.

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T25 | craftsman | internal/core/gates/security/allowlist.go; internal/core/gates/security/exceptions.go; internal/core/gates/security/exceptions_test.go | T06 | go test ./internal/core/gates/security -run 'Test(Allowlist|Exception)' | exception requires finding/action/reason/ticket/owner/scope/revision/expiry/control; missing field fails; append-only, edit changes digest R7.1,R7.2 |
| [x] T26 | craftsman | internal/cmd/registry.go; internal/cmd/security_approve_test.go; internal/core/commands.go | T25 | go test ./internal/cmd -run 'Test(SecurityApprove|Revoke|Exception)' | approve/revoke commands; expired/revoked/wrong-revision suppresses nothing and re-surfaces; cannot waive evidence R7.2 |
| [x] T27 | craftsman | internal/cmd/report.go; internal/cmd/report_test.go; internal/orchestration/acp.go; internal/orchestration/acp_test.go | T20,T26, Domain 07 export | go test ./internal/cmd ./internal/orchestration -run 'Test(Report|ACP)' | policy-digest-keyed audit correlates authority/tools/diff/scans/verify/review/exceptions/submit; dup/out-of-order fails; no secrets/raw args/hidden reasoning R7.3 |

## W8 — cross-platform adapters and release proof

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T28 | craftsman | internal/core/verify/adapter.go; internal/core/verify/adapter_test.go; internal/integration/sandbox_conformance_test.go; docs/troubleshooting.md | T21, Domain 10 adapter | go test ./internal/core/verify ./internal/integration -run 'Test(Adapter|SandboxConformance)' && ./scripts/docs-lint.sh | Linux/macOS/CI adapters declare capabilities; production refuses adapter missing required capability; equivalent policy outcome R8.1 |
| [x] T29 | craftsman | internal/core/gates/security/regress.go; internal/core/gates/security/testdata; scripts/regress-domains.sh; scripts/regress-lint.sh | T24,T27 | go test ./internal/core/gates/security -run 'TestRegress' && ./scripts/regress-domains.sh && ./scripts/regress-lint.sh | promoted incident fixture has redacted provenance + expected finding; policy change invalidates stale attestation; deterministic trend, no model R8.2 |
| [x] T30 | craftsman | internal/cmd/e2e_test.go; internal/integration/security_conformance_test.go; docs/command-reference.md; docs/CHEATSHEET.md | T07,T11,T14,T17,T27,T28 | go test ./internal/cmd ./internal/integration -run 'Test(SecurityE2E|SecurityConformance)' && ./scripts/docs-lint.sh | production-profile E2E: out-of-scope fail, scout MCP denial, `.specd` injection, untracked credential, sandbox net/cred block, misspelled dep, stale exception — all fail closed |
| [x] T31 | validator | specs/06-security-permissions-and-governance; internal/core; internal/cmd; internal/mcp; internal/orchestration; internal/integration | T29,T30 | go test ./... -race -count=1 && go vet ./... && gofmt -l . && ./scripts/test-lint.sh && ./scripts/docs-lint.sh && ./scripts/regress-all.sh && ./scripts/regress-domains.sh | full Domain 06 evidence; `go mod tidy` clean |

> **W8 deviation.** T29's verify requires `TestRegress`; `internal/core/gates/security/regress_test.go`
> is therefore included as its contract-first test file. T30's conformance test owns the cross-surface
> fail-closed matrix; existing command E2E coverage is reused rather than duplicated.

## Cross-wave rules

- Add a failing public-contract fixture before each enforcement change (T02 seeds W1-W5).
- Production is default-deny and fail-closed; prototype is explicit warn/confirm — never a silent
  downgrade of an existing project.
- Worker-reported paths/digests are audit hints; scope is harness-derived. Never mock a gate green
  at the completion/report boundary.
- Domain 05 owns the mission/lease transport; Domain 06 supplies authority/scope verdicts consumed
  by it — no duplicate command/manifest policy.
- Domain 04 evidence freshness and Domain 07 export are validated by contract, never bypassed.
- External adapter (dependency evidence, cross-platform sandbox) failure cannot change core state
  except as a validated pinned-artifact result. Gate stays offline/stdlib-only.
- Keep `reference/` untouched; `gofmt -l .` empty and `go mod tidy` clean before release.
