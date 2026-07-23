# Design — workflow-09-driver-session

- references: R1,R1.1,R1.2,R1.3,R2,R2.1,R2.2,R2.3,R3,R3.1,R3.2,R4,R4.1,R4.2,R5,R5.1,R5.2,R5.3,R6,R6.1
- disposition: accepted
- owner: project maintainers

## Boundaries

- `internal/core/session.go` owns the driver-session baseline, the snapshot of untracked paths present at that baseline, context receipts, authority, and nonce ledger.
- `internal/cmd/session.go` owns the open/ack/action CLI sequence. A successful `session ack` is the existing task boundary and re-pins HEAD plus the untracked snapshot before issuing task authority.
- `internal/cmd/lifecycle.go` owns the bound completion transaction and worktree scope enforcement.
- `internal/core/gates/diffscope.go` remains the pure declared-scope check. Its input receives only task-attributable changes.
- `internal/cmd/verify.go` owns the post-verify completion guidance.
- Brain mission reissue stays in `internal/cmd/brain_*.go` and `internal/orchestration`; no second session model is introduced.
- Excluded: weaker scope rules, reusable nonces, evidence migration across HEAD, parallel writers in one worktree.

## Interfaces

- `DriverSession` persists `BaselineHead` plus a sorted snapshot of untracked paths present when the baseline was pinned. Missing snapshot data from older sessions is handled conservatively.
- `session open` reports the ordered `ack → verify → action → complete-task` sequence.
- `session ack <slug> <task> --tokens <n>` atomically refreshes the baseline and pre-existing-untracked snapshot before binding the acknowledged task authority. This existing operation is the R1.2 rotation interface; a new verb is rejected as redundant.
- Diff derivation filters only untracked paths proven present in the session snapshot. Tracked changes and later-created untracked paths retain existing enforcement.
- Bound completion validates the binding before work, spends the nonce only after all non-mutating gates pass, then performs the completion mutation.
- Successful verify output and JSON include the exact bound completion command when a live acknowledged session exists; otherwise existing unbound guidance remains.
- A stale Brain mission refuses with the current and mission HEAD and names one existing deterministic resume/reissue command.

## Invariants

- Baselines come only from resolvable git HEAD and are pinned before task mutation.
- Re-pinning clears task-specific receipt and authority before new authority is issued; prior evidence is never made current.
- A worker cannot exempt an untracked path by naming it after session open.
- Harness-owned artifacts are excluded only when their change is attributable to the current Specd transaction; direct edits still refuse.
- Nonces remain single-use for operations that mutate state. Validation failures do not consume them.
- Task completion still requires current passing evidence pinned to HEAD.

## Failure

- Unresolvable HEAD: ack refuses before changing the session and tells the driver to commit or initialize git.
- Session CAS conflict: ack/completion refuses with revision conflict; reload and retry.
- Stale mission baseline: report refuses before completion and points to deterministic resume/reissue.
- Failure after nonce spend: completion returns the mutation error and the nonce remains spent, because the transaction entered its mutating section.
- Legacy session without an untracked snapshot: no path is assumed pre-existing; enforcement fails closed.

## Integration

- Session JSON gains only optional fields, preserving reads of existing sessions.
- Existing `session action` output remains the nonce source; verify may call the same binding builder rather than duplicate fields.
- Brain and manual completion continue through `runTaskComplete`; controller-owned marker writes must be separated from worker scope before that shared path.
- No runtime dependency or configuration key is added.

## Alternatives

- New `session rotate` verb: rejected; `ack` already marks the next task boundary and must run before authority activation.
- Ignore every `.specd/` change: rejected; it would hide direct artifact edits.
- Ignore all untracked files: rejected; it weakens task scope.
- Spend nonce before scope/evidence checks and print another nonce: rejected; leaving the original unspent on non-mutating refusal is smaller and preserves retry ergonomics.

## Verification

- Session tests prove ack re-pins HEAD, snapshots pre-existing untracked paths, clears prior task binding, and prints ordered guidance.
- Diff-scope/completion tests prove pre-existing untracked paths pass, later paths refuse, direct harness edits refuse, controller marker sync passes, and a refused completion can retry with the same nonce.
- Verify tests prove text and JSON emit the fully bound command.
- Brain tests prove stale HEAD refusal names the deterministic reissue route and serial marker changes do not bleed into the next mission.
- Full race suite and domain regressions preserve evidence, CAS, parser, and orchestration invariants.

## Deployment

- Additive session fields require no migration. Behavior activates on the next open or ack.
- Observe typed refusals and workflow regression cases; no feature flag.

## Rollback

- Revert the three task commits together if scope attribution weakens or session CAS regressions appear.
- Existing session files remain readable after rollback because new fields are optional JSON fields.
