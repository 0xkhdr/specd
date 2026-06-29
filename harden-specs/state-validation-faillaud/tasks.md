# Tasks — Fail-Loud State Validation (A10)

## Wave 1 — Reject impossible child status
- [ ] T1 — Strict status parse on resume
  - why: corrupt child status must not be coerced (Req 1)
  - role: builder
  - files: internal/spec/status.go, cross-spec-recovery resume path
  - contract: parse on-disk child status strictly; unknown/impossible value →
    error naming spec + value; no default coercion.
  - acceptance: impossible status rejected; valid statuses still load.
  - verify: go test ./internal/spec/ ./... -run "Resume|Status"
  - depends: —
  - requirements: 1

- [ ] T2 — Resume-rejection test
  - why: lock the fail-loud behavior (Req 1)
  - role: verifier
  - files: resume test (cross-spec-recovery)
  - contract: hand-edit a child status to impossible value; assert clear error
    with spec name + value; table includes valid statuses that must pass.
  - acceptance: fails if corrupt status silently coerced.
  - verify: go test ./... -run "ResumeReject"
  - depends: T1
  - requirements: 1

## Wave 2 — Disable warning
- [ ] T3 — pinky-brain disable warning on real path
  - why: warning must fire on actual disable (Req 2)
  - role: verifier
  - files: internal/cmd/ pinky-brain-console test
  - contract: assert warning emitted when active sessions exist; none when absent.
  - acceptance: fails if warning missing on disable-with-active-sessions.
  - verify: go test ./internal/cmd/ -run "PinkyBrain|Disable"
  - depends: —
  - requirements: 2
