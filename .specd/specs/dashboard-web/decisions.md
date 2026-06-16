# Decisions — Dashboard Decoupling from VS Code

<!--
ADR ledger (append-only). Use `specd decision <spec> "<text>" [--supersedes ADR-NNN]`
to append. Entries are numbered monotonically and never edited. Format:

## ADR-001 — <decision summary> · 2026-06-16
**Context:** <what forced the choice>
**Decision:** <what we chose>
**Consequences:** <trade-offs, what it rules out>
**Supersedes:** <ADR-id or —>
-->

## ADR-001 — Remove editors/vscode/ outright (no deprecation stub); migration path = browser-first specd serve documented in docs/dashboard.md. Context: extension owns no dashboard logic (only spawns specd serve + iframes it per T1), so a stub adds maintenance burden with zero value. Consequences: VS Code users lose the in-editor command and must run 'specd serve <slug>' then open http://127.0.0.1:8765/ in a browser; specd binary stays the single source of dashboard truth; rules out a privileged client. · 2026-06-16
**Context:** TODO
**Decision:** Remove editors/vscode/ outright (no deprecation stub); migration path = browser-first specd serve documented in docs/dashboard.md. Context: extension owns no dashboard logic (only spawns specd serve + iframes it per T1), so a stub adds maintenance burden with zero value. Consequences: VS Code users lose the in-editor command and must run 'specd serve <slug>' then open http://127.0.0.1:8765/ in a browser; specd binary stays the single source of dashboard truth; rules out a privileged client.
**Consequences:** TODO
**Supersedes:** —
