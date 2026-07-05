# Tasks — 04-criterion-evidence

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | scout | internal/core/evidence.go, internal/core/gates/ears.go, internal/core/state.go | | `printf ok` | Confirms evidence record shape, EARS parser criterion enumeration capability, and whether state.json shape change needs schema bump per spec 02 |
| T2 | craftsman | internal/core/evidence.go, internal/core/evidence_test.go | T1 | `go test ./internal/core -run TestCriterionRecord -race -count=1` | Criterion record type {criterion, status, evidence, gitHead, timestamp, actor}, append-only, atomic write under spec lock; fail records retained after later pass; git HEAD pinning matches task-verify discipline (R1, R3, R4) |
| T3 | craftsman | internal/core/gates/ears.go (or shared parser), internal/core/gates/ears_test.go | T1 | `go test ./internal/core/gates -run TestCriterionIDs -race -count=1` | Single parser enumerates valid `<r>.<n>` ids from requirements.md; no duplicate parsing logic (R2 groundwork) |
| T4 | craftsman | internal/cmd/verify.go, internal/cmd/verify_test.go | T2,T3 | `go test ./internal/cmd -run TestVerifyCriterion -race -count=1` | `verify --criterion <r>.<n> --status pass\|fail --evidence <v>`; unknown id exits 2 naming it; criterion path never creates task verify records (R1, R2, R7) |
| T5 | craftsman | internal/cmd/status.go, internal/cmd/report.go + tests | T2 | `go test ./internal/cmd -run 'TestStatusCriteria|TestReportCriteria' -race -count=1` | Per-requirement coverage n/m shown in status and report; "current" = recorded after latest requirements approval (R5) |
| T6 | craftsman | internal/core/gates/approval.go (or evidence gate), config loader, gate tests | T2 | `go test ./internal/core/gates -run TestCriteriaRequired -race -count=1` | Opt-in `criteria.required` config key; completion approval refuses while any criterion lacks current passing record; default off leaves existing flows untouched (R6) |
| T7 | craftsman | docs/command-reference.md, docs/CHEATSHEET.md, docs/validation-gates.md | T4,T6 | `./scripts/docs-lint.sh` | New flags + opt-in gate documented; operator-supplied-evidence asymmetry vs task verify explained in validation-gates.md |
| T8 | validator | (read-only) | T4,T5,T6 | `go test ./... -race -count=1` | Full suite green including lifecycle e2e with criteria.required enabled |
