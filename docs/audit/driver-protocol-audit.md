# Audit — agent-driver-protocol (T10, second pass)

Role: auditor. Supersedes the first pass, which returned **fail** on two
blocking findings (F1, F2). Both are resolved and re-verified by probe, not by
inspection alone.

Verdict: **pass**, with two minor findings recorded below. Neither is a bypass,
neither touches evidence integrity, and both are named rather than deferred
silently.

## The three checks this task exists to make

**No bypass added.** No flag, config key, or code path added by this spec can
satisfy a gate, skip evidence, or complete a task without a passing verify
record. `CheckDiffScope` returns findings only and has no "satisfied" path.
`internal/core/evidence.go`, `task_complete.go`, and `internal/core/verify/`
remain untouched. The one place enforcement yields — a spec with no open
session — yields to the *absence* of a governing session, never to an argument,
and closing a session is a visible act rather than a hidden flag.

**No LLM in any gate path.** No network, model, or prompt reference in
`diffscope.go`, `session.go`, `contextreceipt.go`, `conformance.go`,
`hostcontract.go`, `decide.go`, or `lease.go`. Every added decision is a pure
function of on-disk state and the git worktree. (Sweeps hit
`gates/review.go` and `gates/security/*`, which are pre-existing comments and a
test fixture, not live code.)

**No silently upgraded assurance.** Both surfaces route through `AssuranceFor` /
`AssuranceCeiling`, which only lower. `EvaluateHostContract` caps a full
declaration at advisory when any of the seven controls is unmet, and the
re-export in `internal/integration` is asserted to return the same answer as the
core policy so the two cannot drift.

## Resolved since the first pass

**F1 — production had lost a refusal.** Fixed. `enforceDiffScope` now refuses
`BASELINE_UNPINNED` when `ProductionTaskAuthorityRequired()` is true and nothing
pinned a baseline; the graduated proceed-anyway behaviour is scoped to the
default profile, which is what was actually decided.
`TestDiffScopeProductionRefusesUnpinnedTask` pins both branches and fails if the
profile stops arming.

**F2 — R2, R3, R5, R6.1 were libraries, not enforcement.** Fixed. All nine entry
points now have live call sites in the command layer, and the chain was driven
end to end against a real repository:

    session open   -> pins the baseline
    complete-task  -> BINDING_MISSING (session open, no bindings supplied)
    session ack    -> records the receipt, binds authority
      --partial    -> 7 lanes unacknowledged, authority withheld
    session action -> mints a single-use nonce
    complete-task  -> completed
      (replay)     -> NONCE_REPLAYED, conformance event recorded

`ValidateOperation` shows no direct caller by design: it runs inside
`SpendNonce`, so validate-and-spend is one CAS write under one lock and two
concurrent operations cannot both observe a nonce unspent.

**F3 — `drive --sandbox` bought a governed-looking label.** Fixed as a
consequence of routing `drive` through `EvaluateHostContract`. A CLI invocation
can assert one of eight controls, so it stays advisory and reports the seven
unmet clauses. A flag is not an attestation.

**F4 — a test in this spec was vacuous.** Fixed. `TestDiffScopeHasNoBypass` now
writes `project.yml` with a top-level `profile:` key and asserts the profile
actually armed before drawing a conclusion.

## N1 (fixed during this pass) — authority was bound by a hand-rolled digest

`internal/cmd/session.go` bound authority using
`Digest(PolicyDigest + TaskID + Mode)`. `AuthorityV1` carries `Digest`, computed
by `FinalizeAuthority` over the canonical whole packet, and that is what a
binding must pin: the ad-hoc value covered three fields, so two packets differing
in actor, expiry, declared paths, or allowed tools would have produced the same
binding. Now binds `guarded.Digest`. The `AuthorizeWithReceipt` call in that
branch is also kept rather than short-circuited — it is the single function that
decides whether authority activates, and letting a caller skip it because it
already checked is how a second answer to that question gets introduced.

## N2 (minor, open) — binding enforcement lapses at session expiry

`enforceSessionBinding` returns nil for an expired session, so after
`DriverSessionTTL` (2h) a host that had declared itself governed silently returns
to the unenforced path. Probed:

    bindings, live session:      BINDING_MISSING ...
    bindings, expired session:   <nil>
    diff-scope, expired session: OUTSIDE_SCOPE ...

Bounded rather than serious: diff-scope does not check expiry and keeps
refusing, so the load-bearing gate still holds, and evidence integrity is
untouched. It is also consistent with the documented design that a crashed host
should self-heal without manual repair. But the two checks disagree about what an
expired session means, and "wait out the TTL" should not be a way to reduce
enforcement. Worth a follow-on decision: either expiry refuses mutable
operations outright, or both checks treat expiry identically.

## N3 (minor, open) — the CLI context ack is operator-asserted, not host-reported

`specd session ack` sets `supplied = required`, i.e. the operator asserts on the
host's behalf that every required lane was loaded. R3.1 describes a
*host-reported* receipt, and over a real transport the host sends the digests it
actually holds. The CLI cannot know what a human read. This is disclosed in the
code and is why a CLI session stays advisory, but the receipt recorded on that
path is a rubber stamp and should not be read as evidence the context was
loaded. `--partial` exists to exercise the withholding branch.

## Scope deviations

T2, T4, T7, T9 and this remediation pass touched files outside their declared
`files` cells; each was approved by the human at the time and is recorded in
WORKFLOW-FEEDBACK.md. One structural move is worth restating: the host-contract
policy now lives in `internal/core/hostcontract.go`, because
`internal/integration`'s tests import `internal/cmd` and `drive.go` needs the
contract — importing it the other way was an import cycle. T6's declared file
remains as the host-facing surface, now type aliases plus a drift test.

## Verification

`gofmt`, `go vet`, `go test ./... -race -count=1`, `go test ./... -count=2`,
`scripts/test-lint.sh`, `scripts/docs-lint.sh`, and `scripts/regress-domains.sh`
all pass. Zero runtime dependencies preserved; no `go.sum`. The `AD-R8` domain
probe now drives the full protocol chain (ack, mint, complete, replay) and was
validated by sabotage: restoring the pre-R4.5 profile gate makes it fail with
`completion accepted an undeclared file on the default profile`.
