# Quickstart — specd in 5 minutes

> This guide walks through installing `specd` and running a complete spec lifecycle
> from `init` to `task complete`. Time to first green verify: ~5 minutes.

---

## 1. Prerequisites

- **Go 1.22+** — `go version` to check.
- **git** — required at runtime for evidence pinning and the advisory lock.
- No other runtime dependencies. `specd` ships as a single static binary.

---

## 2. Install

```bash
# Clone and build
git clone https://github.com/0xkhdr/specd.git
cd specd
go build -o specd .

# Verify
./specd help
```

Or copy the binary somewhere in your `PATH`:

```bash
cp specd /usr/local/bin/specd
```

---

## 3. Initialize a project

Run `specd init` in the root of your project repository:

```bash
cd my-project
specd init
```

This creates:
```
.specd/
  roles/         # scout, craftsman, validator, auditor role prompts
  steering/      # reasoning, workflow, product, tech, structure, memory
AGENTS.md        # updated with specd integration guide (marker-merged)
```

---

## 4. Create a spec

A *spec* is a scoped piece of work with requirements, design, and a task DAG.

```bash
specd new payment-service
```

This creates `.specd/specs/payment-service/` with stub files:
- `requirements.md` — EARS-shaped requirements placeholder.
- `design.md` — module boundaries + invariants placeholder.
- `tasks.md` — task DAG stub.
- `state.json` — machine state (`status: requirements, revision: 0`).

---

## 5. Author requirements

Open `.specd/specs/payment-service/requirements.md` and replace the stub with real
EARS-shaped requirements:

```markdown
# Requirements — payment-service

- **R1** When a POST /payments request is received, the system shall create a
  payment record and return HTTP 201 with the payment ID.
- **R2** When a payment record is created, the system shall append an audit log
  entry with timestamp, actor, and amount.
```

EARS format: *When \<trigger\>, the system shall \<response\>.*

Check the gate before approving:

```bash
specd check payment-service
```

Approve requirements (human-only; agents cannot approve via MCP):

```bash
specd approve payment-service requirements
```

---

## 6. Author design

Open `.specd/specs/payment-service/design.md` and fill in the module boundaries:

```markdown
# Design — payment-service

## Modules
- `internal/payments/` — payment record creation + ID generation.
- `internal/audit/` — append-only audit log writer.

## On-disk contracts
- `payments.db` — SQLite file, created on first write.

## Invariants
- Payment IDs are UUID v4, never sequential.
- Audit log entries are append-only; no update or delete path.
```

Approve design:

```bash
specd approve payment-service design
```

---

## 7. Author the task DAG

Open `.specd/specs/payment-service/tasks.md` and write the task table:

```markdown
# Tasks — payment-service

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | craftsman | internal/payments/payment.go | - | go test ./internal/payments/ | payment record created, ID is UUID v4 |
| T2 | craftsman | internal/audit/log.go | T1 | go test ./internal/audit/ | audit log entry appended on payment create |
| T3 | validator | - | T1,T2 | go test ./... | all tests pass |
```

Approve tasks:

```bash
specd approve payment-service tasks
```

---

## 8. Work the frontier

Get the next runnable task:

```bash
specd next payment-service
# → T1
```

Get the context manifest for a task (the minimum files the agent needs):

```bash
specd context payment-service T1
```

Implement `T1`. Then verify:

```bash
specd verify payment-service T1
```

If the `verify:` command passes, mark the task complete:

```bash
specd task complete payment-service T1
```

Repeat for T2, then T3.

---

## 9. Check and report

Run the gate registry at any time:

```bash
specd check payment-service
```

View the current status:

```bash
specd status payment-service
specd status payment-service --json
```

Generate a PR summary:

```bash
specd report payment-service --pr
```

---

## What's next?

- **[Commands reference](commands.md)** — all flags and exit codes.
- **[Architecture](architecture.md)** — understand the data flow.
- **[Configuration](configuration.md)** — `config.yml` keys and `SPECD_*` env vars.
- **[Contributing](contributing.md)** — add a gate, add a command, run tests.
- **[Troubleshooting](troubleshooting.md)** — if something goes wrong.
