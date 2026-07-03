# specd command cheat sheet

<!-- Verbatim mirror of the "## Cheat sheet" table in docs/command-reference.md.
     Enforced in lockstep by scripts/docs-lint.sh; edit the source table, not this copy. -->

| Command | One-sentence description |
|---|---|
| `specd init` | Scaffold `.specd/`, managed agent integration, repair, packs, and orchestration defaults. |
| `specd new` | Create a spec and optionally select orchestrated execution with `--orchestrated`. |
| `specd status` | Show one-spec/all-spec progress, recorded mode, and the cross-spec frontier with `--program`. |
| `specd context` | Print the phase-scoped briefing and budgeted LOAD-NOW manifest. |
| `specd check` | Run validation gates or emit/validate the embedded schema with `--schema`/`--schema-only`. |
| `specd approve` | Clear a human approval gate and ratchet the spec to the next phase. |
| `specd next` | Show the next runnable task, all frontier tasks, or dispatch packets with `--dispatch`. |
| `specd verify` | Run a task verification command or record per-criterion proof. |
| `specd task` | Perform the evidence-gated task status transition and telemetry annotation. |
| `specd report` | Generate snapshots, HTML, metrics, history, diff, live dashboard, or frontier stream views. |
| `specd decision` | Append an architectural decision record to `decisions.md`. |
| `specd midreq` | Log mid-flight requirement feedback with impact and analyzed changes. |
| `specd memory` | Add or promote a durable learning from a spec. |
| `specd waves` | Render the task wave DAG, critical paths, and blockers. |
| `specd brain` | Drive deterministic orchestration sessions and context checkpoints. |
| `specd pinky` | Record worker claims, briefs, heartbeats, progress, queries, reports, blockers, and releases. |
| `specd version` | Print the binary version. |
| `specd help` | Show human help or dump the command registry JSON. |
| `specd mcp` | Run the MCP server or print host configuration snippets. |
| `specd handshake` | Emit hidden host bootstrap and binding-policy diagnostics. |
