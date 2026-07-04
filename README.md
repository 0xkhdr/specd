# specd

> **The agent reasons. The harness enforces.**

`specd` is an **agent-agnostic, spec-driven coding harness CLI** — a single static Go binary
with zero runtime dependencies. It moves process integrity *off* the LLM's non-deterministic
context window and *onto* a strict, local, tool-gated pipeline any coding agent can drive.

```
Agent = Model + Harness
```

The model (Claude Code, Cursor, Codex, Aider, or any MCP client) supplies reasoning.
`specd` supplies everything else: lifecycle gates, task DAG, evidence integrity, and
deterministic reporting — all as pure functions of on-disk state, with no LLM in any
decision path.

---

## Why specd

Most agent failures, examined honestly, are configuration failures. The harness is the
~90% the team owns; the model provider owns the rest. `specd` *is* that harness:

| Harness layer | What specd provides |
|---|---|
| **Instructions** | Role-prompt files + steering constitution for the agent |
| **Tools** | Deterministic CLI verbs callable as MCP tools |
| **Sandboxes** | Scrubbed, bounded `verify:` execution environments |
| **Orchestration** | Task DAG / wave controller (`brain`) |
| **Guardrails** | Gates that block state changes without proof or human approval |
| **Observability** | Truthful projections of `state.json` — never LLM-generated |
| **Context** | Bounded, progressive-disclosure context manifests |

---

## Quickstart

```bash
# Build
git clone https://github.com/0xkhdr/specd.git && cd specd
go build -o specd .

# Initialize a project
cd my-project
specd init

# Create a spec
specd new my-feature

# Author requirements, design, tasks — then run the lifecycle
specd approve my-feature requirements
specd approve my-feature design
specd approve my-feature tasks
specd next my-feature          # → T1
specd verify my-feature T1
specd task complete my-feature T1
```

See **[docs/quickstart.md](docs/quickstart.md)** for the full walkthrough.

---

## The 16 verbs

```
init      new       approve   next      verify    task
check     status    context   memory    decision  midreq
report    handshake mcp       brain
```

Every verb maps to exactly one harness component and one governing principle.
See **[docs/charter.md](docs/charter.md)** and **[docs/commands.md](docs/commands.md)**.

---

## On-disk layout

```
<project-root>/
└── .specd/
    ├── roles/          # scout · craftsman · validator · auditor
    ├── steering/       # reasoning · workflow · product · tech · structure · memory
    └── specs/
        └── <slug>/
            ├── requirements.md   # EARS-shaped requirements
            ├── design.md         # module boundaries + invariants
            ├── tasks.md          # task DAG (pipe-table Markdown)
            ├── state.json        # machine truth (status, phase, revision, records)
            ├── evidence.jsonl    # append-only evidence ledger
            └── memory.md         # per-spec steering memory
```

---

## Hard invariants

These hold at every commit (ADR-8):

- **Atomic writes** — `temp → fsync → chmod 0644 → rename`; no partial write ever replaces.
- **CAS on revision** — `SaveStateCAS` inside `WithSpecLock`; test builds panic on unlocked writes.
- **Parser byte round-trip** — `Serialize(Parse(x)) == x`; property + fuzz tested.
- **Zero runtime dependencies** — `go.mod` has no `require`; single static binary.
- **Evidence integrity** — no task completes without a passing verify record at a real git HEAD.
- **Determinism** — gates, DAG, reports are pure functions of on-disk state; no LLM or network in any decision path.

---

## MCP integration

```bash
specd mcp   # → stdio JSON-RPC 2.0 server
```

The tool set is data-driven from `Commands[]` — the same registry that drives `specd help`.
Per-spec tool policy (`required / optional / forbidden`) is read from `.specd/manifest.json`.
Agents cannot call `approve`, `init`, `mcp`, or `brain` over MCP (human-gate policy).

---

## Documentation

| Doc | Purpose |
|---|---|
| [docs/quickstart.md](docs/quickstart.md) | First-time user guide |
| [docs/commands.md](docs/commands.md) | Full CLI reference with flags and exit codes |
| [docs/architecture.md](docs/architecture.md) | Package map, data flow, on-disk layout |
| [docs/charter.md](docs/charter.md) | Verb → harness component + principle map |
| [docs/configuration.md](docs/configuration.md) | `config.yml` keys and `SPECD_*` env vars |
| [docs/adr-log.md](docs/adr-log.md) | Architecture Decision Records (ADR-0 – ADR-11) |
| [docs/contributing.md](docs/contributing.md) | Build, test, and contribution guide |
| [docs/troubleshooting.md](docs/troubleshooting.md) | Common errors and remedies |
| [PROJECT.md](PROJECT.md) | Authoritative context: philosophy, ADRs, roadmap, current position |

---

## Build & test

```bash
go build -o specd .           # zero-dep build
go vet ./...                  # static analysis
go test -race ./...           # all tests + race detector
go test -fuzz=FuzzParseSerialize ./internal/core/ -fuzztime=10s
```

No `require` stanza in `go.mod`. No external dependencies. Zero CGO.

---

## License

[MIT](LICENSE)
