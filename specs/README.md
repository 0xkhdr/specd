# specd Refactor Specs

Derived from `CODE_REVIEW_PROMPT.md`. Each stage is a self-contained refactor
domain with two files:

- `spec.md` — full analysis of the domain: current state, grounded findings
  (file:line), the refactor intent, design decisions, risks, and acceptance
  criteria.
- `tasks.md` — ordered, detailed implementation tasks for a coding agent, with
  exact files, code shape, and verification per task.

Stages are ordered by **dependency + risk**: do them top to bottom. Earlier
stages establish primitives (security, locking, shared helpers) that later
stages build on.

| Stage | Domain | Why first/later | Risk if skipped |
|-------|--------|-----------------|-----------------|
| [01-security](01-security/spec.md) | Shell exec, self-update, install script, env validation, file perms | Highest blast radius (RCE, supply chain). No code deps. | Arbitrary code execution, MITM binary swap. |
| [02-concurrency-state](02-concurrency-state/spec.md) | `lock.go`, `state.go` CAS, `io.go` atomic write | Integrity primitive every command relies on. | Lost writes, corrupt state, deadlock. |
| [03-command-decomposition](03-command-decomposition/spec.md) | `check.go`, `task.go`, `verify.go` split into gate/handler units | Needs shared helpers from 04 but defines the gate funcs they reuse. | Unmaintainable 288-line funcs, gate bugs. |
| [04-cli-output-consistency](04-cli-output-consistency/spec.md) | `args.go`, `main.go` dispatch, exit codes, JSON nil-slices, output helper | Cross-cutting; standardizes what 03 emits. | Inconsistent exit codes / JSON, brittle parsing. |
| [05-dag-domain-logic](05-dag-domain-logic/spec.md) | `dag.go`, `ears.go`, `tasksparser.go`, boot, enrich correctness | specd's core value; depends on stable state shape. | Wrong scheduling, cycle/wave bugs. |
| [06-performance](06-performance/spec.md) | Regex hoisting, allocations, `byID`, marshaling | Pure optimization; safe after correctness locked. | Wasted CPU/alloc in hot paths. |
| [07-testing-ci](07-testing-ci/spec.md) | Coverage gaps, `-race`, golden determinism, multi-OS CI | Locks in every prior stage. | Regressions ship unnoticed. |

## Conventions for the implementing agent

- One stage = one branch = one PR. Do not mix stages.
- Run `go vet ./... && gofmt -l . && go test -race ./...` before declaring a
  task done. A non-empty `gofmt -l` is a failure.
- No `state.json` schema breaks without a `migrate()` path bump (`state.go`
  `SchemaVersion`). Backward compatibility is a hard constraint.
- Preserve the project axiom: **"The agent reasons. The harness enforces."**
  No refactor may weaken a gate.
- Exit codes always come from `core.Exit*` constants — never bare `return 1`.
