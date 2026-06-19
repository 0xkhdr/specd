# MCP Integration — Implementation Progress

> Source plan: [`specd-mcp-analysis-action-plan.md`](../specd-mcp-analysis-action-plan.md)
> Goal: cut MCP tool surface from ~33 to ~8–12, add Resources/Prompts, make the
> tool list phase-adaptive — without breaking existing `expose: "all"` users.

## Specs

| Spec | Plan items | Effort | Risk | Impact | Wave |
|------|-----------|--------|------|--------|------|
| [mcp-config-tool-filtering](./mcp-config-tool-filtering/spec.md) | A1, A3, A4 | Low | Low | High | 1 |
| [mcp-composite-tools](./mcp-composite-tools/spec.md) | A2, A4 | Low | Medium | High | 2 |
| [mcp-resources](./mcp-resources/spec.md) | B1 | Medium | Medium | Very High | 2 |
| [mcp-prompts](./mcp-prompts/spec.md) | B2 | Medium | Medium | High | 2 |
| [mcp-dynamic-tool-list](./mcp-dynamic-tool-list/spec.md) | B3 | Medium | Medium | High | 3 |
| [mcp-context-manifest-tools](./mcp-context-manifest-tools/spec.md) | C1 | High | Medium | Medium | 4 |
| [mcp-host-negotiation](./mcp-host-negotiation/spec.md) | C2 | High | High | Low | 4 |

## Wave plan

Waves are dependency layers. Specs inside a wave have no inter-dependency and
may proceed in parallel. A wave starts only after every spec in the prior wave
passes its acceptance criteria.

### Wave 1 — Foundation (config-aware tool list)
Threads `*core.Config` into `buildTools` and adds the `mcp` config block. Every
later wave consumes this plumbing.

- [x] **mcp-config-tool-filtering** — `MCPConfig` struct, `expose` modes, meta/orchestration gating, `buildTools(cfg)`.

### Wave 2 — Surface reduction + alternate channels
All depend on Wave 1's `buildTools(cfg)` signature + config block.

- [ ] **mcp-composite-tools** — `specd_inspect`/`specd_read`/`specd_query`, unified `specd_orchestrate`/`specd_worker`.
- [ ] **mcp-resources** — `resources/list` + `resources/read` for spec artifacts and steering files.
- [ ] **mcp-prompts** — `prompts/list` + `prompts/get` for phase/role prompts.

### Wave 3 — Adaptivity
Depends on Wave 1 (filter) and Wave 2 (resources/prompts capabilities to advertise).

- [ ] **mcp-dynamic-tool-list** — `listChanged: true`, `notifications/tools/list_changed`, phase watcher, thread-safe tool list.

### Wave 4 — Advanced (speculative, future-proof)
Depends on Wave 1 + Wave 3.

- [ ] **mcp-context-manifest-tools** — per-spec `contextManifest` tool filtering.
- [ ] **mcp-host-negotiation** — `initialize` `maxTools`/`preferredNamespaces` honouring.

## Cross-cutting invariants (apply to every spec)

1. **Backward compatible:** absent config ⇒ `expose: "all"` ⇒ byte-identical `tools/list` to today.
2. **Stdlib-only, no network, no LLM calls** — MCP is a thin transport over existing handlers.
3. **Deterministic output:** stable ordering for tools, resources, prompts.
4. **No new core authority:** new tools/resources translate to existing handlers; never widen what an agent can do.
5. **Tests per [plan §7]:** unit (filtered lists per config), integration (start server, assert counts), round-trip (composite == atomic), concurrency (HTTP transport safe).

## Status legend
`[ ]` not started · `[~]` in progress · `[x]` done (acceptance met)
