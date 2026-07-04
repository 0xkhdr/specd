# review-specs — Production waves for the fresh-start build

> **Purpose.** Translate the audited gap between concept (PROJECT.md §1–§7) and shipped
> functionality (PROJECT.md §8 / BUILD_REVIEW.md findings F1–F14) into executable specs.
> Each wave is one spec (`spec.md` + `tasks.md`) in the same shape as `specs/*`:
> EARS requirements, design notes, 6-key task DAG. When all seven close with evidence,
> the fresh-start build is production-complete with no concept↔functionality gap left.

## Sources & precedence

1. PROJECT.md §3 guardrails + §4 ADRs (binding).
2. BUILD_REVIEW.md §3 findings + §5 action plan (the gap inventory these specs cover).
3. `specs/*/spec.md` (the original domain contracts these waves finish enforcing).

## Wave DAG

```
W0 restore-truth ──► W1 close-the-loop ──► W2 seal-trust-boundary ──► W5 surface-config ──► W6 hardening-release ──► W7 regression-acceptance
                          │                                              ▲
                          ├──► W3 records-integrity ─────────────────────┤
                          └──► W4 gates-and-constitution ────────────────┘
```

- **W0 blocks everything** — it is honesty, not features.
- W1 and W2 in series (share `lifecycle.go`/`registry.go`).
- W3 ∥ W4 after W1.
- W5 after W4 (memory-verb decision depends on P4.3 making memory functional).
- W6 last; its dogfood gate requires every prior wave closed via `specd task complete` with real evidence.
- **W7 standing regression** (`07-regression-acceptance`) — after W6; re-runs every wave's verify at HEAD and re-asserts each wave's per-domain best-practice invariant (REGRESSION_REVIEW.md audit method) so no later wave silently regresses an earlier one, and no wave is marked done ahead of live evidence.

## Dogfood rule

Every task in W1–W6 must be driven through specd itself: a real spec under this repo's
`.specd/specs/`, verified via `specd verify`, completed via `specd task complete`.
W0 alone is exempt (the loop cannot close before W1 ships `task complete`) — its
evidence is the literal verify commands passing.

## Finding coverage matrix (no gap unowned)

| Finding | Severity | Covered by |
|---|---|---|
| F1 falsified progress.md | 🔴 | W0 R0.1, R0.3 |
| F2 loop cannot close | 🔴 | W1 R1.1 |
| F3 approvals gate nothing | 🔴 | W1 R1.2 |
| F4 MCP self-approval | 🔴 | W2 R2.1 |
| F5 ADR-7 mode enum missing | 🔴 | W1 R1.3 |
| F6 hollow decision/midreq records | 🔴 | W3 R3.1 |
| F7 18 verbs vs 16 | 🟠 | W5 R5.1 |
| F8 missing content gates | 🟠 | W4 R4.1, R4.2 |
| F9 steering/memory inert | 🟠 | W4 R4.3 |
| F10 config wrong name / fail-silent / dead flag | 🟠 | W5 R5.2 |
| F11 brain not fail-closed; pinky unregistered | 🟠 | W2 R2.2, R2.3 |
| F12 repo .specd/ contradicts scaffold | 🟠 | W0 R0.2 |
| F13 progress.md files: wrong | 🟡 | W0 R0.1 |
| F14 small gaps (git_head unknown, timestamps, check summary, task slug, manifest paths, --unverified) | 🟡 | W1 R1.4 · W3 R3.1–R3.2 · W5 R5.3 |
| standing regression (no wave silently regresses another; nothing marked done ahead of live evidence) | 🔵 | W7 R7.1–R7.5 (`scripts/regress-{all,lint,domains}.sh`) |

Paper-principle closure: P3 ← W0+W1 · P6 ← W1+W2 · P8 ← W4 · P2/P5/P7 ← W3+W5.

## Global guardrails (apply to every task here)

- No new packages, no dependencies, no "task engine" abstraction — fixes land in
  existing files (`lifecycle.go`, `registry.go`, `gates/core.go`, `manifest.go`, `state.go`).
- Don't restore flywheel features to fix F7 — subtract the stub.
- Don't move task status back into `tasks.md` markers — ADR-1 chose `state.json`; finish that choice.
- ADR-8 hard invariants preserved verbatim; any change needs a new recorded ADR.
- **Do not ship before W0 + W1 + task P2.1 land.**

## Definition of done

- **A wave:** all its EARS requirements demonstrably enforced (its verify commands pass);
  its findings row above is closed; §3 guardrails still hold; `go build ./... && go vet ./... && go test ./...` green.
- **A task:** its `verify:` passes with the record written (exit code + git HEAD); touches
  only its declared `files:`.
