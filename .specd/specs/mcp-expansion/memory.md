# Memory — MCP Expansion for External Tools

<!--
Source-attributed, generalizable learnings (append-only). Use
`specd memory <spec> add --key <slug> --pattern "<one-line>" --body "<detail>"
  --source "<Turn N, Task T?, role>" --criticality <minor|important|critical> [--related k,k]`.
Only generalizable patterns, never raw observations. Promote to project steering at 3+ specs via
`specd memory <spec> promote --key <slug>`. Format:

## <key-slug>
**Pattern:** <one-line generalizable claim>
**Detail:** <why it's true; the mechanism>
**Source:** Task T3, Turn 2, discovered by investigator
**Criticality:** important
**Related:** [[other-key]]
-->

## serialise-stdout-swap
**Pattern:** Any transport reusing mcp.callTool must serialise calls because capture() swaps process-global os.Stdout
**Detail:** callTool's capture() replaces os.Stdout/os.Stderr for the whole process during a tool call. A second front door (HTTP adapter) must hold a mutex around dispatch, and parity tests must gather the stdio reference BEFORE starting a concurrent transport server — else -race flags a data race on os.Stdout.
**Source:** Turn 1, Task T4/T6, builder
**Criticality:** important
**Related:** —
