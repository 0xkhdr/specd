# Tasks — Code Quality & Readability (S3)

## Wave 1

- [x] T1 — Measure true cyclomatic complexity of the five named hotspots
  - why: the live-evidence pass used branch-keyword density as a proxy (gocyclo wasn't installed during research); confirm actual cyclomatic numbers before choosing a linter threshold (Requirement 1.1) or committing to refactor scope (Requirement 2)
  - role: investigator
  - files: internal/cmd/pinky.go, internal/core/orchestration_driver.go, internal/cmd/init.go, internal/cmd/doctor.go, internal/core/acp.go
  - contract: install/run `gocyclo` (or `gocognit`) against the five named functions and the full `internal/` tree; report exact complexity scores for each of the five, and the full distribution (how many functions repo-wide exceed common thresholds of 10/15/20). Do NOT write or modify code.
  - acceptance: written table of exact complexity scores for the five named functions plus repo-wide distribution summary
  - verify: N/A
  - depends: —
  - requirements: 1, 2

- [x] T2 — Inventory exported symbols missing doc comments in the six undocumented packages
  - why: Requirement 3.3 names three confirmed examples but requires completeness across all six packages; need the full list before writing comments
  - role: investigator
  - files: internal/core/, internal/cmd/, internal/cli/, internal/runner/, internal/pack/, internal/schema/
  - contract: for each of the six packages, list every exported (capitalized) function/type/const lacking a doc comment immediately above its declaration. Do NOT write or modify code.
  - acceptance: a per-package list of undocumented exported symbols (file:line)
  - verify: N/A
  - depends: —
  - requirements: 3

## Wave 2

- [x] T3 — Add complexity linter to .golangci.yml
  - why: enforce the threshold going forward, per Requirement 1.1, using T1's measured baseline to pick a realistic number
  - role: builder
  - files: .golangci.yml
  - contract: add `gocyclo` (or `gocognit`) with a threshold informed by T1's distribution (e.g., set just above the bulk of the codebase, not so high it's meaningless). For any of the five named hotspot functions not yet refactored by T5 at the time this lands, add an explicit per-function exclusion with a one-line rationale comment — do not silently raise the global threshold to absorb them. Separately, evaluate `revive`: either enable it with a specific rule subset that doesn't duplicate `staticcheck`/`gocritic`, or record in this task's evidence why it's skipped.
  - acceptance: `make lint` passes; any exclusions are function-specific and commented, not a blanket high threshold
  - verify: cd /var/www/html/rai/up/specd && make lint
  - depends: T1
  - requirements: 1

- [x] T4 — Add package-level doc comments
  - why: close the package-doc gap in the two largest packages plus four others, per Requirement 3.1-3.2
  - role: builder
  - files: internal/core/doc.go (new or existing file), internal/cmd/doc.go (new or existing file), internal/cli/doc.go, internal/runner/doc.go, internal/pack/doc.go, internal/schema/doc.go
  - contract: add a `// Package x ...` comment (2-5 sentences, describing the package's role per `docs/contributor-guide.md`'s architecture description) to each of the six packages. Check whether an existing file already has a near-top comment that should be upgraded to a package doc rather than creating a redundant `doc.go`.
  - acceptance: `go doc ./internal/core`, `./internal/cmd`, `./internal/cli`, `./internal/runner`, `./internal/pack`, `./internal/schema` each show a non-trivial package summary
  - verify: cd /var/www/html/rai/up/specd && for p in core cmd cli runner pack schema; do go doc ./internal/$p > /dev/null || exit 1; done
  - depends: —
  - requirements: 3

- [x] T5 — Add doc comments to undocumented exported symbols
  - why: close the function/type-level doc gap found in T2, including the three confirmed examples in Requirement 3.3
  - role: builder
  - files: internal/core/state.go, internal/core/orchestration.go, internal/cmd/next.go (plus any additional files from T2's inventory)
  - contract: add a doc comment to every symbol in T2's list, starting with the symbol name per Go convention (e.g. `// LoadState loads ...`). Do not change any symbol's signature or behavior — comments only.
  - acceptance: `go vet ./...` and `make lint` still pass; spot-check via `go doc` that the three named symbols (`LoadState`, `OrchestrationPolicy`, `RunNext`) now show their comments
  - verify: cd /var/www/html/rai/up/specd && make lint
  - depends: T2
  - requirements: 3

## Wave 3

- [x] T6 — Refactor RunPinky and runInitWithRuntime
  - why: highest-complexity, highest-blast-radius hotspots (Brain/Pinky dispatch, init flow) — tackle separately from the lower-risk pair in T7, per Requirement 2
  - role: builder
  - files: internal/cmd/pinky.go, internal/cmd/init.go
  - contract: extract named helper functions from `RunPinky` (internal/cmd/pinky.go:14) and `runInitWithRuntime` (internal/cmd/init.go:136) such that each resulting function passes the Requirement 1 complexity threshold. Run `cmd/lifecycle_test.go` and `cmd/json_contract_test.go` BEFORE starting (confirm green baseline) and after EVERY extracted helper (not just at the end) to catch a behavior change immediately. External CLI flags, exit codes, and stdout/stderr/JSON contract must be byte-for-byte unchanged.
  - acceptance: `cmd/lifecycle_test.go` and `cmd/json_contract_test.go` pass unmodified; `gocyclo` reports both functions under the new threshold (or under an explicitly justified exclusion per T3)
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/cmd/... -race -count=1 -run 'TestLifecycle|TestJSONContract'
  - depends: T3
  - requirements: 2

- [x] T7 — Refactor DriveOrchestration, runDoctor, and validateACPPayload
  - why: remaining three complexity hotspots, per Requirement 2
  - role: builder
  - files: internal/core/orchestration_driver.go, internal/cmd/doctor.go, internal/core/acp.go
  - contract: extract named helpers from `DriveOrchestration` (orchestration_driver.go:98), `runDoctor` (doctor.go:67), and `validateACPPayload` (acp.go:283). For `validateACPPayload`, if T1 confirms its complexity is an irreducible switch over a closed enum, add a linter exclusion with rationale instead of forcing artificial indirection (per Requirement 2.4) — record that decision in this task's evidence rather than silently refactoring or silently skipping. For ACP payload changes specifically, diff serialized wire output before/after to confirm no behavior change beyond the Go-level test suite (per spec.md's open question).
  - acceptance: `DriveOrchestration` and `runDoctor` pass the new complexity threshold; `validateACPPayload` either passes it or has a documented, justified exclusion; existing orchestration/doctor/ACP tests pass unmodified
  - verify: cd /var/www/html/rai/up/specd && go test ./internal/core/... ./internal/cmd/... -race -count=1
  - depends: T6
  - requirements: 2

## Wave 4

- [x] T8 — Full regression and lint gate
  - why: action-prompt rule — validate each wave; this spec touches Brain/Pinky dispatch, init, doctor, and ACP — all high-traffic paths
  - role: verifier
  - files: N/A
  - contract: run the full project test and lint suite
  - acceptance: `make test` and `make lint` pass with zero regressions attributable to S3
  - verify: cd /var/www/html/rai/up/specd && make test && make lint
  - depends: T7, T5, T4
  - requirements: 1, 2, 3
  - evidence: `make test` PASS; `make lint` PASS; targeted `go test ./internal/cmd/... -race -count=1 -run 'TestLifecycle|TestJSONContract'` PASS; `go test ./internal/core/... ./internal/cmd/... -race -count=1` PASS; gocyclo named hotspots after refactor: RunPinky 17, runInitWithRuntime 14, validateACPPayload 12, DriveOrchestration 7, runDoctor 4; package docs verified via `go doc ./internal/{core,cmd,cli,runner,pack,schema}` and symbol spot checks for LoadState, OrchestrationPolicy, RunNext.
