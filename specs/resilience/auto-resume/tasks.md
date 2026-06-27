# Tasks — Auto-Resume Hook (R5)

## Wave 1 — Discovery core
- [ ] T1 — `ListResumableSessions` enumerator
  - why: pure read of resumable sessions (Req 1)
  - role: builder
  - files: internal/core/orchestration_resume.go (new)
  - contract: read every `session.json` under `SessionsDir()`; filter to running|paused, apply
    optional max-age; derive `lastDecision` from existing recorded decision/event tail; return
    structs sorted by `UpdatedAt` desc. No writes.
  - acceptance: returns only running|paused within age; sorted desc; empty slice when none.
  - verify: go test ./internal/core/ -run Resumable
  - depends: —
  - requirements: 1

- [ ] T2 — Config `resilience.autoResume` block
  - why: declarative startup policy (Req 3)
  - role: builder
  - files: internal/core/specfiles.go, internal/core/embed_templates/config.json
  - contract: add `AutoResume{Enabled,OnHostStart bool; MaxAgeMinutes int}` under the shared
    `Resilience` block (omitempty). Validate `MaxAgeMinutes>=0`. Byte-identical config when absent.
  - acceptance: absent block → no new bytes; invalid maxAge → load error.
  - verify: go test ./internal/core/ -run "Config|Drift"
  - depends: —
  - requirements: 3

## Wave 2 — CLI surface
- [ ] T3 — `brain resume --list` / `--max-age-minutes`
  - why: host-facing discovery (Req 1)
  - role: builder
  - files: internal/cmd/brain.go, internal/cmd/brain_commands.go
  - contract: extend the existing `resume` case: `--list` calls `ListResumableSessions` and
    prints JSON array; `--max-age-minutes` forwards the filter. No-flag behavior unchanged.
  - acceptance: `--list --json` prints sorted array; `[]` exit 0 when none; old behavior intact.
  - verify: go test ./internal/cmd/ -run BrainResume
  - depends: T1
  - requirements: 1

- [ ] T4 — `brain resume --session <id>` idempotent continue
  - why: safe repeatable resume (Req 2)
  - role: builder
  - files: internal/cmd/brain.go
  - contract: reconstruct policy from session.json and continue driver (reuse `brain run`
    machinery); reject complete/failed; rely on existing CAS for idempotency.
  - acceptance: resuming running session continues; complete/failed errors; double-call safe.
  - verify: go test ./internal/cmd/ -run BrainResume
  - depends: T1
  - requirements: 2

## Wave 3 — MCP + docs
- [ ] T5 — MCP `brain_resume` tool
  - why: hosts resume without shelling out (Req 4)
  - role: builder
  - files: internal/cmd/mcp.go
  - contract: register `brain_resume` with `{session?, json?}`; dispatch to the same core
    enumerator/resume entry as the CLI; description instructs startup call.
  - acceptance: tool lists when session omitted, resumes when present; shares core path with CLI.
  - verify: go test ./internal/cmd/ -run Mcp
  - depends: T3, T4
  - requirements: 4

- [ ] T6 — Document host startup contract
  - why: adapter authors need the recipe (Req 5)
  - role: builder
  - files: AGENTS.md, docs/agent-integration.md
  - contract: add the on-startup list→resume recipe and multi-session tie-break rule.
  - acceptance: docs describe the exact command sequence and config knobs.
  - verify: N/A
  - depends: T5
  - requirements: 5

- [ ] T7 — Auto-resume integration test
  - why: prove crash-restart transparency (Req 1,2)
  - role: verifier
  - files: internal/cmd/brain_resume_test.go
  - contract: create running + paused + complete sessions; assert list filters/orders; resume
    the top session and confirm the driver continues without double-dispatch.
  - acceptance: test green; complete session excluded; idempotent resume verified.
  - verify: go test ./internal/cmd/ -run BrainResume
  - depends: T4
  - requirements: 1, 2, 3
