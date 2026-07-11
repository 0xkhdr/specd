# W0-T01 — Domain 06 requirement→surface inventory

Read-only scout mapping of requirements R1–R8 (`specs/06-security-permissions-and-governance/requirements.md`)
to existing code surfaces, with the gap and the owning boundary domain. File:line refs are to the
current tree. No product code was edited.

## Requirement → surface map

| Requirement | Current code surface | Gap | Boundary domain |
|---|---|---|---|
| R1.1 versioned `security_profile` (prototype/production) resolves pre-mission; invalid/absent fails closed | `internal/core/config_loader.go:30-39` `SecurityConfig` is per-scanner `off|warn|error` only; `DefaultConfig.Security` `:127-133`; `LoadConfig` `:142-158` | No `security_profile` field, no prototype/production concept, no fail-closed on invalid profile; profile never resolved before dispatch | Domain 01 (phase/approve semantics) |
| R1.2 `CoreRegistry`/`submit` include required security gates under production; submit refuses on stale/missing security with `security_evidence_stale` | `internal/cmd/submit.go:32` runs `gates.CoreRegistry().Run(...)` — security excluded; security gate is opt-in `internal/core/gates/security/gate.go:21-31` invoked only via `check --security` | Security not in CoreRegistry or submit; no freshness/stale check; no `security_evidence_stale` structured failure | Domain 01; Domain 04 (evidence freshness) |
| R1.3 stable `policy_version`/`policy_digest` pinned to dispatch/evidence/report | none | No policy digest exists anywhere; config values only, no unified resolved-policy hash | Domain 07 (trusted measurement/export) |
| R1.4 profile change explicit with migration diagnostics, no silent downgrade | `LoadConfig` diagnostics `internal/core/config_loader.go:142-158` (generic parse diagnostics only) | No profile-change detection or migration diagnostic path | Domain 01 |
| R2.1 declared paths normalized repo-relative globs; `../`/absolute/symlink escapes fail closed; tests explicit | `internal/core/tasksparser.go:12-13` `Role`/`Files` are free text; `files` gate `internal/core/gates/core.go:134-142` checks only non-empty | No path normalization, no glob model, no escape rejection, no explicit-tests rule ("plus tests" is undefined) | Domain 05 (report correlation) |
| R2.2 harness derives change set from pinned baseline (tracked/staged/untracked/delete/rename/mode/symlink); worker paths are hints | `internal/orchestration/acp.go:41` `ChangedFiles` is worker-reported; no consumer compares it | No harness-derived diff from a pinned task baseline; worker report never reconciled | Domain 05 (completion/report) |
| R2.3 derived change outside declared scope fails completion even when verify passes | task completion `internal/core/task_complete.go` gates on evidence only; no scope verdict | No `outside_scope` finding; scope never enforced at completion | Domain 05 |
| R2.4 byte-stable parser keeps round-tripping after scope metadata added | `internal/core/tasksparser.go` (byte-stable today) | New scope columns/metadata not yet added; must preserve round-trip when they are | Domain 05 |
| R3.1 role gate accepts only 4 roles; no craftsman fallback for unknown/auditor | `roles` gate `internal/core/gates/core.go:124-132` non-empty only; `RolePrompt` `internal/core/roles.go:12-25` unknown→craftsman; `ModeForTask` `internal/context/manifest.go:157-168` has scout/validator/scribe, **auditor falls through to craftsman**, unknown→craftsman | Role gate does not enumerate the 4 roles; two fallbacks silently grant craftsman write-mode; `scribe` mapped but not a documented role | Domain 05 (mission envelope) |
| R3.2 machine-readable authority packet (actor/worker/spec/task/phase/role, allow/deny tools+arg constraints, read/write paths, network/sandbox, baseline rev, expiry, policy digest) | none; `internal/orchestration/authority.go` is brain dispatch-authority boolean only | No authority packet type or serialization exists | Domain 05 (envelope/lease transport) |
| R3.3 dispatch/MCP deny scout/validator/auditor writes; craftsman writes outside declared paths denied; unknown tools default-deny in production | `internal/mcp/server.go:86-88` denies via `core.ForbiddenTool`; `internal/core/manifest_tools.go:16-23` is a **static global** verb denylist, role-agnostic | No role-aware or path-aware tool policy; no default-deny-in-production; ForbiddenTool ignores role/phase | Domain 05 |
| R3.4 stale/expired/wrong-phase authority packets fail closed | none | No packet, so no expiry/phase validation | Domain 05 |
| R4.1 context scan includes runtime `.specd/` tree + pending untracked/generated set | `trackedFiles` `internal/core/gates/security/gate.go:172-189`; `excludedDirs` `:210` **excludes `.specd`**; only git-tracked files scanned | `.specd/` (specs/steering/roles/memory) — the highest-value context — is excluded; untracked/generated files never scanned | Domain 02 (context receipt/selection) |
| R4.2 failed git enumeration / unreadable in-scope file yields error finding, not empty scan; exclusions explicit | `trackedFiles` `:173-175` returns `nil` on `git ls-files` error (**fail open**); `:182-185` silently skips unreadable files | Enumeration/read failure produces a green empty scan; no error finding | Domain 02 |
| R4.3 each context item carries trust label (instruction vs data) + digest; only embedded role/policy renders as instruction | none in scanner; `internal/context/manifest.go` builds context without trust labels | No trust label or per-item digest; untrusted text is not structurally prevented from acting as instruction | Domain 02 |
| R4.4 findings reference exact locations + bounded safe excerpts; never inline full secret/payload | `redact` `internal/core/gates/security/scanner.go:62-67`; `formatMessage` `internal/core/gates/security/gate.go:59-65` | Redaction exists for secrets scanner excerpts but is not a central pipeline covering verify output/evidence/reports/context | Domain 07 (export) / Domain 02 |
| R5.1 production verify runs in required sandbox (net off, synthetic HOME, minimal env, private temp, controlled writable) | `verify.Options.Sandbox` bool `internal/core/verify/exec.go:14-28`; `wrapArgv` `:98-112` (bwrap `--unshare-all`, ro-root, private `/tmp`, dir bind); missing binary fails closed `:55-58` | Sandbox is an **opt-in CLI flag**, not required by profile; not mandatory at completion; only verify path | Domain 04 (verify/evidence) |
| R5.2 host credential paths unavailable in sandbox; read-only root insufficient — secrets hidden | `wrapArgv` `:104` `--ro-bind / /` makes host **readable**; `scrubbedEnv` `:114-125` keeps real `HOME` | Real `HOME`/`$HOME/.aws/credentials` and all host files remain readable; no path hiding/synthetic HOME | Domain 04 |
| R5.3 verify stdout/stderr, evidence, reports, context pass central redaction | `redact` in security scanner only `internal/core/gates/security/scanner.go:62-67`; verify captures raw `stdout/stderr` `internal/core/verify/exec.go:67-71` | No central redaction of verify output/evidence/reports; a secret in output prints in full | Domain 04; Domain 07 |
| R5.4 sandbox args express resource limits (CPU/mem/proc/output/wall/fs growth); breach records failure | `TimeoutSecs` wall-clock only `internal/core/verify/exec.go:44-48,77-81` | Only wall timeout; no CPU/memory/process/output-size/fs-growth limits; no breach failure record beyond timeout | Domain 04 |
| R6.1 dependency policy inspects manifest diff with declared reason/source; unknown registry/checksum/provenance fails; lockfiles inspected | `slopsquat` scanner (Go-name typosquat) via `Analyze` scanner set `internal/core/gates/security/gate.go:95`; lockfiles excluded `:197` | Go-name heuristic only; no manifest diff, no registry/checksum/provenance, lockfiles globally excluded; no declared reason/source | Domain 10 (external evidence adapter transport) |
| R6.2 external vuln/provenance evidence as pinned offline adapter artifacts; malformed/stale fails; offline+stdlib | none | No adapter artifact ingestion/validation path | Domain 10 |
| R6.3 deterministic policies over normalized diffs detect destructive shell, world-writable/exec mode, authz changes, generated secrets, path/symlink escapes | none (no diff-based policy engine; scanners are content-only) | No normalized-diff policy set; no dangerous-command/mode/authz/symlink detection | Domain 10 |
| R7.1 exception requires finding/action, reason, ticket, owner, scope, rev/env, issue/expiry, compensating control; missing field fails | `allowEntry` `internal/core/gates/security/allowlist.go:16-19` = fingerprint + reason only; `loadAllowlist` `:34-59` fails closed on missing reason/fingerprint | Only reason+fingerprint required; no ticket/owner/scope/rev/expiry/compensating-control fields | Domain 07 (export) |
| R7.2 exceptions append-only, edit changes digest; expired/revoked/wrong-rev suppress nothing; cannot waive evidence/broaden authority; reports show active+historical | `allow.json` array (rewritable), fingerprint match `internal/core/gates/security/allowlist.go:25-28` | Not append-only, no digest, no expiry/revoke/rev scope, no report projection of active vs historical | Domain 07 |
| R7.3 unified sanitized audit view keyed by run/mission/task + policy digest correlating authority/tools/diff/scans/verify/review/exceptions/submit; dup/out-of-order fails import; no secrets | partial ACP ledger `internal/orchestration/acp.go`; `state.security` records findings | No unified policy-digest-keyed mission view; records live in separate stores; no import ordering validation | Domain 07 |
| R8.1 sandbox capability negotiation across Linux/macOS/CI adapters; production refuses missing capability; conformance fixtures; no runtime dep | `wrapArgv` is bwrap/Linux-specific `internal/core/verify/exec.go:98-112`; `--sandbox-binary` swap only | No capability negotiation, no macOS/CI adapters, no conformance fixtures | Domain 10 (adapter) |
| R8.2 promoted incidents become deterministic regression fixtures with redacted provenance + expected finding; policy changes invalidate stale attestations; trend reports need no model | `internal/core/gates/security/testdata/` fixtures exist for current scanners | No incident→regression promotion path, no attestation invalidation, no trend report | Domain 10; Domain 07 |

## P0 gaps (from design.md action plan; each has no current enforcement)

1. **No operating profile / security not required (R1, R1.2).** `SecurityConfig` is per-scanner
   severity only (`internal/core/config_loader.go:30-39`); security gate is opt-in
   (`gate.go:21-31`) and excluded from both `CoreRegistry` and `submit` (`internal/cmd/submit.go:32`).
   Production has no mandatory security/scope/sandbox binding and no `security_evidence_stale` refusal.
2. **Declared-file scope unenforced (R2).** `Files` is free text (`tasksparser.go:12-13`), the `files`
   gate checks only non-empty (`gates/core.go:134-142`), and `ChangedFiles` is a worker claim never
   reconciled to a harness-derived baseline diff (`orchestration/acp.go:41`). No `outside_scope` verdict.
3. **Role→capability fallbacks grant write silently (R3.1/R3.3).** `roles` gate accepts any non-empty
   string (`gates/core.go:124-132`); `RolePrompt` (`roles.go:12-25`) and `ModeForTask`
   (`context/manifest.go:157-168`, auditor + unknown → craftsman) fall back to craftsman; MCP denial
   is a static global verb list, not role/path/phase aware (`manifest_tools.go:16-23`, `mcp/server.go:86-88`).
4. **Context/change-boundary scan misses `.specd/` and fails open (R4.1/R4.2).** `excludedDirs`
   excludes `.specd` (`gates/security/gate.go:210`); `trackedFiles` returns `nil` on `git ls-files`
   error and skips unreadable files (`:173-185`) — a green empty scan; untracked/generated files never scanned.
5. **Sandbox opt-in and reads unprotected (R5.1/R5.2/R5.3/R5.4).** Sandbox is a CLI bool
   (`verify/exec.go:14`), `--ro-bind / /` leaves host readable (`:104`), `scrubbedEnv` keeps real
   `HOME` (`:114-125`), output is captured raw with no central redaction (`:67-71`), and only a wall
   timeout exists — no CPU/mem/proc/output/fs limits.

## Notes / observations
- Comment tags in existing security code cite "spec 05" (`gate.go:14`, `scanner.go:1`) — the opt-in
  gate predates this Domain 06 program; Domain 06 hardens rather than replaces it.
- `ModeForTask` maps a `scribe` role not among the four documented roles (scout/craftsman/validator/
  auditor); R3.1 will need to reconcile this.
- Redaction (`scanner.go:62-67`) and fail-closed allowlist load (`allowlist.go:34-59`) are the two
  patterns already aligned with the target contract and can be generalized rather than rebuilt.
