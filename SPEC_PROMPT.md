Specd Orchestrated Mode — Pinky Worker Pool

Setup
./specd status --program    # confirm mode = orchestrated

Worker Policy — PINKY POOL
Spawn **one** `pinky-craftsman` for the first task and **continue it** for related tasks via `SendMessage`.  
Dispatch **additional** craftsmen only when:
- Task boundaries differ (e.g., frontend vs. backend, unrelated domains)
- The active worker is saturated or blocked
- Explicit isolation is required (security, conflicting file locks)

Brief each worker once with: repo constraints, spec paths, role file, and:
> "Do not open/close sessions, do not approve, do not complete-task, do not commit — the driver owns those."

Per-Task Driver Flow
1. Open session (baseline = current HEAD)
./specd session open <slug> <task> --driver claude-code

2. Ack tokens
./specd session ack <slug> <task> --tokens <n>

3. Dispatch to appropriate pinky worker
    - Same worker if task is related/continuous
    - New worker if boundaries differ or worker saturated
    -> Worker edits only declared files, ends with:
./specd verify <slug> <task>

4. Complete & commit
./specd complete-task <slug> <task> --session <id> --nonce <fresh nonce>
git commit -m "<slug>: <task>"
./specd session close <slug>

**Commit after every task** before the next, or diff-scope bleeds files forward.

Approvals
Run yourself at every phase gate. Do not stop to ask.

./specd check <slug>
./specd approve <slug>

Constraint: Driver Never Edits Source
If a task needs an undeclared file: **stop**, report exact file + reason, amend `tasks.md` in its own commit, then resume.

Completion
Done when `./specd status --program` shows spec complete. Then:

gofmt -l .
go vet ./...
go test ./... -race -count=1
./scripts/docs-lint.sh
./scripts/regress-domains.sh

Append any workflow friction to `WORKFLOW-FEEDBACK.md` with its inventory row. Report what you skipped.
