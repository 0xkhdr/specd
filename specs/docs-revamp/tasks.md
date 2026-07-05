# Tasks — Documentation Revamp

Execution DAG for the doc correction pass. Roles follow specd's own convention
(scout = read-only, craftsman = write+verify, validator = read-only re-check).

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | scout | internal/, main.go | - | N/A | Audit code → confirm 18 verbs, project.yml config, evidence-only completion, no install.sh/Makefile/ears.go (R1) |
| T2 | craftsman | README.md | T1 | go build ./... | Install/build uses `go build`/`go install`; doc map drops charter; verbs accurate (R2,R7,R8) |
| T3 | craftsman | docs/user-guide.md | T1 | go build ./... | Install section deshimmed (no install.sh); escape-hatch paragraph removed; config = project.yml (R2,R3,R4) |
| T4 | craftsman | docs/command-reference.md | T1 | go build ./... | Config Keys + env section state project.yml, no global XDG claim (R3) |
| T5 | craftsman | docs/validation-gates.md | T1 | go build ./... | Escape-hatch note removed; ears path = gates/ears.go (R4,R5) |
| T6 | craftsman | docs/agent-integration.md | T1 | go build ./... | project.yml not config.yml; orchestrated-mode claim corrected (R3,R6) |
| T7 | craftsman | docs/contributor-guide.md | T1 | go build ./... | `make` targets → `go build`/`go test`/`go vet`; ears path fixed (R4,R5) |
| T8 | craftsman | docs/concepts.md | T1 | go build ./... | Config/path references reconciled to project.yml; nav drops charter (R3,R8) |
| T9 | craftsman | docs/README.md | T2,T3,T4,T5,T6,T7,T8 | go build ./... | Single accurate nav map; no charter link (R8) |
| T10 | craftsman | docs/charter.md | T9 | go build ./... | File deleted; no dangling links remain (R6) |
| T11 | craftsman | AGENTS.md | T1 | go build ./... | Stale top brief removed; integration block escape-hatch line fixed (R4,R6) |
| T12 | craftsman | CLAUDE.md | T1 | go build ./... | PROJECT.md/specs/progress.md refs removed; points to real docs (R6) |
| T13 | craftsman | internal/core/embed_templates/AGENTS.md, internal/core/embed_templates/steering/reasoning.md | T1 | go test ./... | Shipped templates drop the escape-hatch lie (R4,R9) |
| T14 | validator | docs/, README.md, AGENTS.md, CLAUDE.md, internal/core/embed_templates/ | T2,T3,T4,T5,T6,T7,T8,T9,T10,T11,T12,T13 | go build ./... && go test ./... | Acceptance greps clean; build+tests green (R9) |
