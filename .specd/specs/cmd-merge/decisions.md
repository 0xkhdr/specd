# Decisions — cmd-merge

<!--
ADR ledger (append-only). Use `specd decision <spec> "<text>" [--supersedes ADR-NNN]`
to append. Entries are numbered monotonically and never edited. Format:

## ADR-001 — <decision summary> · 2026-06-30
**Context:** <what forced the choice>
**Decision:** <what we chose>
**Consequences:** <trade-offs, what it rules out>
**Supersedes:** <ADR-id or —>
-->

## ADR-001 — Legacy merge aliases stay functional + warn (exit 0) during grace, flip to exit non-zero at removal version · 2026-06-30
**Context:** cmd-merge's design said a removed top-level name should exit 2 with a "moved to" hint; cmd-deprecate REQ-1.2 said grace-period commands must stay functional and warn. The two conflict, and the shipped code resolved it by neither — the merged names dispatched silently at exit 0 (optimization-plan GAP-1).
**Decision:** During the grace period a merged alias (doctor, dispatch, program, validate, schema, replay, diff, serve, watch, mode) stays FUNCTIONAL: it runs its original handler, returns that handler's exit code, and emits a one-line stderr deprecation warning naming the survivor home + removal version. Genuinely retired names with no functional survivor (migrate, update, uninstall) warn and exit non-zero. At each alias's recorded `removedIn` the entry is deleted from `legacyAliases`, so the name flips to the unknown-command help path (effectively exit 2 / not-found).
**Consequences:** Honors REQ-1.2 (functional + warn) without the silent-exit-0 gap, at the cost of two reachable paths per behavior until the removal version. The original handler is called rather than re-routing through `survivor --flag`, because for doctor/mode the survivor does not preserve full capability (doctor diagnostics ≠ `init --repair`; `mode --set` has no survivor — GAP-2). `mode` removal is held one extra minor (v0.3.0) pending Phase 2 set-mode migration.
**Supersedes:** —

## ADR-002 — Survivor home for the merged mode command's set/advise paths is 'specd status <slug> --set-mode base|orchestrated' and '--recommend'. status already owns mode reporting; the new flags delegate to the original mode handlers in the same package, so no logic is duplicated and no capability is lost (optimization-plan GAP-2 / Phase 2). The mode alias stays functional through its recorded v0.3.0 grace period, but every mode capability now also has a survivor (view=status, create=new --orchestrated, set/advise=status --set-mode/--recommend), so deletion at v0.3.0 drops nothing. · 2026-06-30
**Context:** TODO
**Decision:** Survivor home for the merged mode command's set/advise paths is 'specd status <slug> --set-mode base|orchestrated' and '--recommend'. status already owns mode reporting; the new flags delegate to the original mode handlers in the same package, so no logic is duplicated and no capability is lost (optimization-plan GAP-2 / Phase 2). The mode alias stays functional through its recorded v0.3.0 grace period, but every mode capability now also has a survivor (view=status, create=new --orchestrated, set/advise=status --set-mode/--recommend), so deletion at v0.3.0 drops nothing.
**Consequences:** TODO
**Supersedes:** —
