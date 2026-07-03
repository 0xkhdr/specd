# specd command cheatsheet

| Command | One-sentence description |
|---|---|
| `specd init` | Scaffold `.specd/`, managed agent integration, repair, packs, and orchestration defaults. |
| `specd new` | Create a spec and optionally select orchestrated execution with `--orchestrated`. |
| `specd migrate` | Migrate a v0.1.x project onto the v0.2.0 state schema and report available config blocks. |
| `specd status` | Show one-spec/all-spec progress, recorded mode, and the cross-spec frontier with `--program`. |
| `specd context` | Print the phase-scoped briefing and budgeted LOAD-NOW manifest. |
| `specd check` | Run validation gates or emit/validate the embedded schema with `--schema`/`--schema-only`. |
| `specd review` | Scaffold a structured review report, or extract a deterministic review checklist. |
| `specd approve` | Clear a human approval gate and ratchet the spec to the next phase. |
| `specd next` | Show the next runnable task, all frontier tasks, or dispatch packets with `--dispatch`. |
| `specd verify` | Run a task verification command or record per-criterion proof. |
| `specd task` | Perform the evidence-gated task status transition and telemetry annotation. |
| `specd eval` | Score a spec against its rubric, compile a rubric skeleton with `init`, or report trends. |
| `specd promote` | Promote a prototype spec to a full spec after a passing eval. |
| `specd conductor` | Drive the interactive micro-task conductor session over an append-only ledger. |
| `specd orchestrate` | Inspect and resolve deterministic auto-escalations (status, or resume with an override). |
| `specd submit` | Validate all gates, build the PR summary, and run the configured submit command. |
| `specd deploy` | Run the evidence-gated deploy driver or replay the recorded rollback chain. |
| `specd observe` | Correlate a production error payload into an evidenced mid-requirement. |
| `specd ingest` | Inventory a legacy codebase into an ingestion-flavored spec. |
| `specd report` | Generate snapshots, HTML, metrics, history, diff, live dashboard, or frontier stream views. |
| `specd decision` | Append an architectural decision record to `decisions.md`. |
| `specd midreq` | Log mid-flight requirement feedback with impact and analyzed changes. |
| `specd memory` | Add or promote a durable learning from a spec. |
| `specd waves` | Render the task wave DAG, critical paths, and blockers. |
| `specd harness` | Share the configured harness (guardrails, deploy, roles, routing) as a versioned team asset with quarantine. |
| `specd dashboard` | Serve the unified, read-only project dashboard (waves, cost, escalations, evals, harness). |
| `specd brain` | Drive deterministic orchestration sessions and context checkpoints. |
| `specd pinky` | Record worker claims, briefs, heartbeats, progress, queries, reports, blockers, and releases. |
| `specd version` | Print the binary version. |
| `specd help` | Show human help or dump the command registry JSON. |
| `specd mcp` | Run the MCP server or print host configuration snippets. |
| `specd handshake` | Emit hidden host bootstrap and binding-policy diagnostics. |
