# Tasks 13 — CLI Seam Regression & Wiring Closure

> **Build waves:** R1 (T13.1–T13.3), R2 (T13.4–T13.7), R3 (T13.8–T13.9), R4 (T13.10–T13.11),
> R5 (T13.12–T13.13), R6 (T13.14–T13.15). See `specs/progress.md`.
> **Depends on domains:** 02, 03, 04, 05, 09, 10. **Unblocks:** honest re-verification of B/C/D/G.
> **Discipline:** every `go run .` verify must assert a side effect, never just exit 0.

## Wave R1 — dispatcher integrity (fail-closed) — do first

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T13.1 | craftsman | `internal/cmd/registry.go`, `main.go` | — | `go build -o /tmp/specd . && cd $(mktemp -d) && git init -q && /tmp/specd bogusverb; test $? -eq 2` | unknown verb prints usage to stderr, exits 2, never 0 |
| T13.2 | validator | `internal/cmd/registry_test.go`, `internal/core/commands.go` | T13.1 | `go test ./internal/cmd -run TestEveryCommandHasHandler` | every `core.Commands` verb resolves to non-nil handler or `Deferred:true`; fails today |
| T13.3 | craftsman | `internal/core/commands.go`, `internal/cmd/registry.go` | T13.2 | `go build -o /tmp/specd . && cd $(git rev-parse --show-toplevel) && /tmp/specd check 01-product-philosophy-core; echo $?` | `check` is a registered verb, runs gate registry, exit reflects findings (not silent no-op) |

## Wave R2 — lifecycle wiring

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T13.4 | craftsman | `internal/cmd/registry.go` (`runNew`) | T13.1 | `go build -o /tmp/specd . && cd $(mktemp -d) && git init -q && /tmp/specd init && /tmp/specd new demo && test -f .specd/specs/demo/state.json` | spec dir + `state.json` at rev 0, mode simple, status requirements, via `SaveStateCAS` |
| T13.5 | craftsman | `internal/cmd/registry.go` (`runApprove`) | T13.4 | `go test ./internal/cmd -run TestApproveGatesE2E` | approve refused when readiness red (state unchanged); ratchets + records on green |
| T13.6 | craftsman | `internal/cmd/registry.go` (`runMidreq`,`runDecision`) | T13.4 | `go test ./internal/cmd -run TestMidreqDecisionAppend` | record appended via CAS; unrelated core fields untouched |
| T13.7 | validator | `internal/cmd/lifecycle_test.go` | T13.4 | `go test ./internal/cmd -run TestStatusNextVerifyOnRealSpec` | `status/next/verify/context/report` exercised against a real `new`-created spec, not a fixture |

## Wave R3 — orchestration wiring

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T13.8 | craftsman | `internal/cmd/brain.go`, `internal/cmd/registry.go` (`runBrain`) | T13.4 | `go test ./internal/cmd -run TestBrainDispatchesFrontierViaCLI` | `brain <sub> <spec>` drives driver, fail-closed authority, writes session+evidence, no LLM |
| T13.9 | craftsman | `internal/cmd/registry.go` (`runTriage`) | T13.3 | `go build -o /tmp/specd . && cd $(mktemp -d) && git init -q && /tmp/specd init && /tmp/specd new t && /tmp/specd triage t; test $? -ne 0 -o -n "$(/tmp/specd triage t 2>&1)"` | triage runs or reports deferral explicitly; never silent no-op |

## Wave R4 — UX + surfaces

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T13.10 | craftsman | `internal/cmd/registry.go` (`runHelp`) | T13.1 | `go build -o /tmp/specd . && /tmp/specd help new \| grep -q 'specd new' && /tmp/specd help --json \| grep -q '"name"'` | help renders from `core.Commands` metadata; `--json` machine-readable |
| T13.11 | craftsman | `internal/cmd/registry.go` (`runTask`) | T13.4 | `go test ./internal/cmd -run TestTaskShowsDetails` | `task <id>` prints parsed task row from `tasks.md` |

## Wave R5 — evidence-integrity harness (the meta-fix)

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T13.12 | validator | `internal/cmd/e2e_test.go` | T13.5, T13.7 | `go test ./internal/cmd -run TestLifecycleE2E` | drives `init→new→check→approve→next→verify→report` through built binary in temp repo, asserts on-disk side effects each step |
| T13.13 | validator | `scripts/verify-progress.sh` | T13.12 | `bash scripts/verify-progress.sh` | every task whose verify is `go run .` is executed by the e2e harness; script fails if any is hand-marked only |

## Wave R6 — reconciliation & truth

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T13.14 | scribe | `specs/progress.md` | T13.12 | `grep -q '13-cli-regression' specs/progress.md && ! grep -q '100%' specs/progress.md` | progress reflects verified reality; falsely-green B/C/D/G tasks demoted until R1–R5 green |
| T13.15 | scribe | `fresh-start/00-decisions.md`, `specs/*/tasks.md` | T13.1 | `grep -qi 'consolidat' fresh-start/00-decisions.md` | ADR records `cmd/*.go` consolidation into `registry.go`, or `files:` columns restored; DoD file-scope check meaningful again |

## Traceability (task → requirement)
- T13.1 → R13.1 · T13.2 → R13.2 · T13.3 → R13.6 · T13.4 → R13.3 · T13.5 → R13.4 · T13.6 → R13.5
- T13.7 → R13.10 · T13.8 → R13.7 · T13.9 → R13.8 · T13.10 → R13.9 · T13.11 → R13.9
- T13.12 → R13.10 · T13.13 → R13.10 · T13.14 → R13.11 · T13.15 → R13.11 (open ADR)

## Critical path
`T13.1 → T13.2 → T13.4 → {T13.5, T13.8} → T13.12 → T13.14`. R1 first: dispatcher must fail closed
and the parity test (T13.2) must exist before any lifecycle verify can be trusted. T13.12 (e2e
golden) is the highest-leverage guard — land it as soon as R2 is wired.
