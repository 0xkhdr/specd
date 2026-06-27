# Tasks — Cross-Spec Program Recovery

## Wave 1 — Program-state persistence
- [ ] T1 — `ProgramState` type + canonical JSON + validator
  - why: authoritative program frontier on disk (Req 1)
  - role: builder
  - files: internal/core/program_state.go (new)
  - contract: define `ProgramState{ParentSessionID, ChildSessions map, InflightKeys, ChildStatus,
    UpdatedAt}`; canonical-JSON encoder + validator; fail-closed on corrupt read.
  - acceptance: round-trips byte-stable; corrupt input errors clearly.
  - verify: go test ./internal/core/ -run ProgramState
  - depends: —
  - requirements: 1

- [ ] T2 — Path helper + CAS write
  - why: crash-coherent latest frontier (Req 1.2)
  - role: builder
  - files: internal/core/runtime_paths.go, internal/core/program_state.go
  - contract: add `ProgramStatePath(parentSessionID)`; CAS write helper mirroring `SaveState`.
  - acceptance: concurrent writes are CAS-guarded; latest frontier always coherent.
  - verify: go test ./internal/core/ -run ProgramState
  - depends: T1
  - requirements: 1

- [ ] T3 — Write program-state each driver step
  - why: keep frontier current (Req 1)
  - role: builder
  - files: internal/core/orchestration_driver.go
  - contract: at the existing program per-step commit point, persist `ProgramState`. No change to
    scheduling behavior.
  - acceptance: program-state.json updated every step; values match driver in-memory frontier.
  - verify: go test ./internal/core/ -run Program
  - depends: T2
  - requirements: 1

## Wave 2 — Resume command
- [ ] T4 — `brain resume --program --session <parent>`
  - why: resume whole program (Req 2)
  - role: builder
  - files: internal/cmd/brain.go
  - contract: read program-state, classify children (complete/running/pending) re-deriving from
    child sessions as authoritative, restart `DriveProgramOrchestration` from the frontier; skip
    complete children; rely on lease/checkpoint recovery for running; CAS-idempotent.
  - acceptance: complete children not re-dispatched; double-call safe; frontier resumes correctly.
  - verify: go test ./internal/cmd/ -run "BrainProgram|Resume"
  - depends: T3
  - requirements: 2

## Wave 3 — Discovery + test
- [ ] T5 — Mark program sessions in resume discovery
  - why: hosts pick the right resume path (Req 3)
  - role: builder
  - files: internal/core/orchestration_resume.go
  - contract: extend `ListResumableSessions` to detect `program-state.json`, set `program:true`
    and `complete/total` child counts; auto-resume routes program sessions to `--program`.
  - acceptance: list flags program parents with counts; selection routes to program resume.
  - verify: go test ./internal/core/ -run Resumable
  - depends: T4
  - requirements: 3

- [ ] T6 — Program crash-recovery integration test
  - why: prove whole-program resume (Req 1,2,3)
  - role: verifier
  - files: internal/cmd/brain_program_run_cov_test.go
  - contract: run a 3-child program, interrupt mid-frontier (one complete, one running, one
    pending), resume --program, assert frontier reconstructs and only pending/running advance.
  - acceptance: test green; no re-dispatch of completed child; idempotent.
  - verify: go test ./internal/cmd/ -run BrainProgram
  - depends: T5
  - requirements: 1, 2, 3
