# Tasks 01 — Product & Philosophy Core

> **Build waves:** A (T1.1–T1.2), B (T1.3–T1.4). See `specs/progress.md`.
> **Depends on domains:** none (charter). **Unblocks:** all.

## Wave 0 — charter & guardrails

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1.1 | craftsman | `docs/charter.md` | — | `test -f docs/charter.md && grep -q 'harness component' docs/charter.md` | Charter maps all 16 verbs to a component + principle |
| T1.2 | craftsman | `go.mod` | — | `test -z "$(go list -m all \| grep -v '^'$(go list -m)'$')"` | No `require` deps |
| T1.3 | craftsman | `main.go`, `internal/cli/args.go` | T1.1 | `test $(go run . 2>&1 \| grep -c .) -ge 16` | Bare invocation lists the 16 core verbs, exit 0 |
| T1.4 | validator | `internal/core/commands_test.go` | T1.3 | `go test ./internal/core -run TestRegistryMatchesHelp` | Help cannot drift from dispatch |

## Traceability (task → requirement)
- T1.1 → R1.1 · T1.2 → R1.2, R1.3 · T1.3 → R1.5 · T1.4 → R1.1
- R1.4 (untrusted artifacts) enforced structurally across Specs 03/05; R1.6 verified in Spec 11.
