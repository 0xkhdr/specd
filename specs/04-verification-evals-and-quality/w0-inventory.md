# W0 T01 — Domain 04 baseline inventory (scout, read-only)

Maps requirements R1–R7 (`specs/04-verification-evals-and-quality/requirements.md`) to the
current evidence / criteria / completion / gates / ACP / context / report surfaces, records the
gap, and names the boundary domain that owns the adjacent contract. Line refs are to `main` at
scout time. Finding: the eval/quality plane is **greenfield** — a repo-wide search for
`evidence_class`, `quality_contract`, `dataset`, `rubric`, `trajectory` in non-test `internal/`
code returns **zero** hits. The base test-evidence plane is strong and no-bypass.

## Current envelope (what exists today)

`EvidenceRecord` (`internal/core/evidence.go:15-30`) json fields: `task_id`, `command`,
`exit_code`, `git_head`, `evidence_ref?`, `timestamp?`, `actor?`, `telemetry?`. No
`evidence_class`, `producer`/`producer_version`/`config_digest`, `subject_revision` distinct from
`git_head`, `verdict`/`score`, or dataset/rubric/output/trace ids/digests.

`TaskRow` (`internal/core/tasksparser.go:9-17`): `ID, Marker, Role, Files, DependsOn, Verify,
Acceptance` — no required-evidence-class/check-id column.

## R1–R7 → current surface → gap → boundary domain

| Requirement | Current code surface | Gap | Boundary domain |
|---|---|---|---|
| **R1** Evidence contract (versioned class envelope, full identity binding, dataset/output/trace pins, legacy = test-only) | `EvidenceRecord` struct `internal/core/evidence.go:15-30`; append/write path in same file; legacy `verify` record is the only class. Completion via `CompleteTask` `internal/core/task_complete.go:16-25`, `HeadPinned` `:12-14` | No `evidence_class` enum (R1.1); envelope lacks run/attempt, producer/version/config digest, verdict, artifact digest (R1.2); no dataset/rubric/output/trace pins (R1.3); no fail-closed on unknown enum/version/field. R1.4 (legacy = test only) is *implicitly* true because only one record type exists, but not asserted. | 01 (requirement/phase + task-metadata provenance for `spec_slug`/acceptance ids); 07 (`mission_id`/run identity in ACP) |
| **R2** Task quality declaration (declare classes+check ids; byte-stable round trip; no cross-class satisfaction) | `TaskRow` `internal/core/tasksparser.go:9-17`; `ParseTasksMd` `:28-62`; byte-stable `RewriteTaskStatusLine` `:64-95`; `verify` presence gate `internal/core/gates/core.go:144-152` | No place to declare required evidence classes/check ids (R2.1). Byte-stable parser exists and must be preserved through any new column/companion artifact (R2.2 = constraint, not gap). No mechanism preventing cross-class/prose satisfaction (R2.3) since classes don't exist. | 01 (task metadata schema) |
| **R3** Offline import, provenance, freshness (deterministic local import; fail-closed on malformed/wrong-*; reject stale-for-subject; required test failure always blocks) | Freshness today = `HeadPinned` presence check only (`task_complete.go:12-25`) — does NOT compare to *current* HEAD or any digest. No-bypass: `verify` gate `gates/core.go:144-152`, `evidence` gate `:154-162`, `HasPassingEvidence` `evidence.go:113`; escalation only resets counter (per domain doc `internal/cmd`) | No adapter import path (R3.1); no malformed/duplicate/wrong-task/wrong-check/wrong-digest ordered failure (R3.2); freshness does not reject records stale vs current subject/output/dataset/rubric/trace digest (R3.3) — this is highest-risk defect #8 in the alignment README. R3.4 (test failure blocks despite score) holds today only because no score path exists. | 06 (production profile decides which digests are required) |
| **R4** Observable trajectory (ordered sanitized run-scoped events; fail on dup/non-monotonic/secret/missing/forbidden; worker self-report not authority) | `ACPEvent` ledger `internal/orchestration/acp.go:23-44`: lifecycle events (dispatch/claim/report) with `ChangedFiles` (worker-reported `:41`), `VerifyRef` `:42`, `Telemetry` `:43`. `Seq` is ledger *position* only (`ReadACP` `:146-171`), monotonic per-file not per-run-event. `AppendDispatch` dup-mission guard `:106-118` | No normalized observable tool/action trace envelope (R4.1); no per-run monotonic sequence, no secret/reasoning-field rejection, no missing/forbidden-step policy (R4.2); `ChangedFiles` is explicitly worker-reported and **not validated** against a harness diff (R4.3) — highest-risk defect #7. | 05 (worker/lease event identity — trajectory source); 06 (trusted diff scope authority) |
| **R5** Risk coverage & verify quality (map critical criteria → check ids; reject trivial/compile-only where risk needs stronger; read-only exception narrow; fail-closed on unknown/unmapped/threshold-less/waiver) | `verify` gate checks **presence only** (`gates/core.go:144-152`) — an exit-zero `printf ok` certifies a write task. `files` gate presence-only `:134-142`. Per-criterion records: `internal/core/criteria.go`; criterion ids `internal/core/gates/criteria.go:37-82`; criteria gate opt-in via `CheckCtx.CriteriaRequired`/`CriteriaUnmet` `gates/core.go:34-35` (operator-declared pass/fail, no coverage measurement) | No production profile / risk classification (R5.1); no verify-quality lint rejecting trivial/compile-only vs declared risk (R5.2) — alignment defect #9; no acceptance-criterion→required-check mapping, no fail-closed on unknown id/unmapped critical/threshold-less/prohibited waiver (R5.3). | 06 (profile + security policy authority) |
| **R6** Eval policy & governance (scorer types incl optional LM metadata recorded-not-executed; dataset/rubric manifest owner/version/digest/labels/critical/threshold/redaction/review; edits invalidate; pure aggregation, insufficient≠pass) | None. No `scorer_type`, no dataset/rubric manifest, no aggregation. Closest existing primitive is human review gate `internal/core/gates/review.go` + `CheckCtx.Review*` `gates/core.go:42-47` (approve verdict fresh at expected HEAD) | Entire governance layer absent (R6.1–R6.3). Constraint carried in: LM/model/network must stay **out of** `CoreRegistry()` gates (`gates/core.go:56-73` is pure over `CheckCtx`, touches no disk). | 10 (adapter transport/capability/data boundary); 07 (export) |
| **R7** Quality context, reports, learning (compact quality-contract packet not raw corpus; report separates passed/missing/stale/score/review; append-only redacted failure/regression ledger) | Context manifest has **no** quality-contract entry (grep: 0 hits in `internal/context/`). Report projection `internal/core/report.go` projects state/task status; no evidence-class or stale/score dimension. No quality ledger. | No compact `quality_contract` context field with refs/digests/freshness (R7.1); report cannot distinguish passed vs missing vs stale vs score vs review (R7.2) — free-form evidence can masquerade as proof (alignment defect #10); no append-only redacted failure/promotion ledger (R7.3). | 02 (context selection/budget); 07 (telemetry/report export); 09 (recurring learning/incident program) |

## P0 gaps (block truthful production quality claim; from R1/R3/R4/R5 and alignment §P0-C)

1. **No evidence-class envelope (R1).** One record type; `verify` exit code carries no class, so
   reports cannot state which part of the quality contract is covered. Root of the tests/evals
   conflation. `internal/core/evidence.go:15-30`.
2. **Freshness is presence-only, not current-subject (R3.3).** `CompleteTask`/`HeadPinned`
   accept any resolvable non-`unknown` HEAD; a later commit leaves prior evidence logically stale
   with no gate detecting it. `internal/core/task_complete.go:12-25`. (Alignment defect #8.)
3. **No enforced trajectory; `ChangedFiles` is unverified worker self-report (R4).** ACP records
   lifecycle events, not a normalized sanitized tool trace; no monotonic per-run sequence, no
   secret/forbidden-step rejection. `internal/orchestration/acp.go:23-44,41`. (Defect #7.)
4. **No verify-quality / coverage lint (R5).** Presence-only `verify` and `files` gates let a
   trivial `printf ok` certify a risk-critical write task; no critical-criterion→check mapping.
   `internal/core/gates/core.go:134-152`. (Defect #9.)

## Load-bearing constraints for downstream waves (not gaps)

- Keep scoring/model/network **out of** `CoreRegistry()` — gates are pure over `CheckCtx`, touch
  no disk (`internal/core/gates/core.go:56-73`). Adapters do execution; gates validate pinned
  local artifacts only (design.md "two planes").
- Preserve byte-stable tasks parser (`tasksparser.go:64-95`), append-only evidence, atomic/CAS,
  and legacy `verify` = `test`-class-only at the read boundary (R1.4 / program rule 1).
- Cross-domain links stay README program links, not local task ids (tasks.md cross-wave rules).
