# Gaps and Recommendations

This report consolidates the gaps previously recorded in
`AGENT-DRIVEABILITY-ANALYSIS.md` and `SPECD-FIELD-NOTES.md` with findings from
the Agent-Driveability remediation. Evidence below refers to the current
worktree. “Resolved” means Waves 1–3 implemented and tested the recommendation;
it does not erase the historical field report.

## Previously reported gaps

| Priority | Status | Gap and evidence | Impact | Recommended fix |
|---|---|---|---|---|
| P0 | Resolved | Quality declarations formerly failed only at completion. The declaration parser now emits the enum/shape hint (`internal/core/quality_contract.go:20-34`), and the gate is registered (`internal/core/gates/core.go:144`). | Agents discovered malformed `evidence` cells after implementation and verification. | Keep the early gate armed for `check` and tasks approval, and retain the black-box refusal probe (`scripts/regress-domains.sh:85-94`). |
| P0 | Resolved for `test/*` | Passing verify runs now project declared test checks into envelopes (`internal/core/eval.go:108-145`, `internal/cmd/registry.go:1050`). Non-test classes remain external by design. | The documented `verify -> complete-task` loop was structurally unable to satisfy declared test evidence. | Preserve HEAD-pinned envelope stamping for `test/*`; continue to require and signpost `eval import` for `output_eval`, `trajectory_eval`, and `review`. |
| P1 | Resolved | Brain waits have distinct authority, frontier, and worker reasons with recovery commands (`internal/orchestration/decide.go:37-39`). | A single opaque wait message made operators guess whether authority, work, or workers were missing. | Keep exact reason strings covered by unit and domain tests (`internal/orchestration/decide_test.go:52-70`, `scripts/regress-domains.sh:110-144`). |
| P1 | Resolved | `agents inspect` resolves to the palette operation (`internal/core/commands.go:825-828`) and dispatch strips the alias token (`internal/cmd/dispatch.go:50-52`); multi-operation help is domain-tested (`scripts/regress-domains.sh:146-151`). | Natural CLI spellings failed closed even though operation metadata existed. | Treat palette metadata as the single source for aliases/help and retain fail-closed behavior for genuinely unknown operations. |
| P1 | Resolved | Coverage errors name the tasks.md `refs` column and both remedies (`internal/core/gates/approval.go:112`); the domain probe verifies an uncovered criterion (`scripts/regress-domains.sh:153-162`). | Coverage defects stayed latent and refusals did not identify the field being read. | Keep tasks-phase advisory coverage and executing-phase blocking coverage; do not widen matching silently to `acceptance`. |
| P2 | Resolved | Doctor reports missing or mismatched harness workers with `specd init --repair` (`internal/core/doctor.go:51-54`), and Brain has a separate no-worker wait reason (`internal/orchestration/decide.go:39`). | Stale scaffolds could enter orchestration with no usable worker and no actionable diagnosis. | Continue checking the configured handshake agent against on-disk worker definitions before dispatch. |
| P2 | Resolved | MCP rejects flag-shaped positional arguments and names the property form (`internal/mcp/server.go:163`). | Agents copied CLI flags into MCP `args`, producing positional-usage failures. | Keep rejection rather than silent normalization so malformed tool calls remain visible and correctable. |
| P2 | Resolved | Verify lint detects non-interactive job-control patterns (`internal/core/gates/verifylint.go:11-28`). | `kill %N` and uncaptured background jobs can hang verification and orphan processes. | Retain warning-level authoring feedback and unit coverage (`internal/core/gates/verifylint_test.go:10-49`). |
| P1 | Open, out of scope | The investigation plan and field notes are historical and contain paths/producer claims superseded by the implementation (`AGENT-DRIVEABILITY-ANALYSIS.md:168-177`, `SPECD-FIELD-NOTES.md:56-73`). | An agent treating those files as current operating instructions can infer obsolete roles, paths, or evidence producers. | Mark historical documents with commit/version scope, or generate/lint operational guidance against `specd help --json`; do not silently rewrite preserved notes. |

## New findings from Wave 3

| Priority | Status | Finding and evidence | Impact | Recommended fix |
|---|---|---|---|---|
| P1 | Corrected in T13 | Generated design fixtures require bullet-form references (`scripts/regress-domains.sh:50-63`). The first fixture used an unrecognized bare `references:` line. | Approval failed before the intended assertion, masking the behavior under test. | Centralize a minimal known-valid spec fixture and validate its lifecycle before running feature-specific probes. |
| P2 | Partially mitigated | The eval probe previously asserted a nonexistent `protocol_version` and compact JSON. It now uses whitespace-tolerant checks for the actual `schema_version`, class, check, and producer (`scripts/regress-domains.sh:103-106`; contract at `internal/core/eval.go:12,31-34`). | Text-shape drift can fail a semantic regression even when the JSON contract is correct. | Prefer structural JSON validation in a small Go test/helper; if shell remains, keep whitespace-tolerant matching and cite canonical schema constants. |
| P1 | Corrected in T13 | Craftsman wait fixtures now use a nontrivial verify command (`scripts/regress-domains.sh:119,135`) because the quality gate rejects shallow write-task verification. | Cross-domain gates can stop a fixture before it reaches the domain behavior being tested. | Give fixture builders role-aware valid defaults, and override only the field the probe intends to invalidate. |
| P1 | Corrected in T13 | The worker-removal probe now configures Claude and removes the matching Claude worker (`scripts/regress-domains.sh:111-118,139-143`). | Hard-coding a different harness tests configuration mismatch instead of missing-worker behavior. | Derive the worker path from the fixture’s configured agent, or provide one helper per supported harness and test both deliberately. |
| P1 | Corrected in T13 | The coverage fixture now keeps valid requirement trace `R1` while omitting only `R1.1` (`scripts/regress-domains.sh:153-160`). | An invalid synthetic ID can trigger schema/trace gates first and falsely appear to test coverage messaging. | Build negative fixtures by changing one contract dimension at a time and assert the expected gate/code before checking message details. |

## Recommendation order

1. P1: annotate or lint historical agent-facing documents so obsolete guidance
   cannot be mistaken for the current CLI contract.
2. P1: factor regression fixture construction into known-valid, role-aware
   helpers that isolate one failure per probe.
3. P2: replace shell text matching of JSON with structural validation where a
   stable helper can be used without adding runtime dependencies.
