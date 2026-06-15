# specd roadmap specs

One folder per recommendation from `specd-report.html` §8 (Missing features &
ideas). Each holds a `spec.md` (objective, EARS requirements, design, non-goals,
acceptance) and a `tasks.md` (dependency-waved execution plan with
role/depends/requirements/verify per task).

Every spec preserves specd's hard invariants: **stdlib-only, zero runtime deps,
no LLM calls in the binary, deterministic output, evidence-gated completion.**

## Recommended build order (report §9 north star)

| # | Spec | Idea | Impact / Moat |
|---|------|------|---------------|
| 1 | [mcp-server](mcp-server/) | A1 | very high / high |
| 2 | [semantic-acceptance-gate](semantic-acceptance-gate/) | B1 | very high / very high |
| 3 | [open-spec-format](open-spec-format/) | E2 | very high / very high |
| 4 | [prompt-scaffolding](prompt-scaffolding/) | A2 | high / low |
| 4 | [spec-pack-registry](spec-pack-registry/) | E1 | high / high |
| 5 | [watch-daemon](watch-daemon/) | C1 | high / med |
| 5 | [github-native-integration](github-native-integration/) | E3 | high / med |

## Full set by theme

**A · Frictionless adoption** — [mcp-server](mcp-server/) (A1) ·
[prompt-scaffolding](prompt-scaffolding/) (A2) · [ide-dashboard](ide-dashboard/) (A3)

**B · Closing the trust loop** — [semantic-acceptance-gate](semantic-acceptance-gate/) (B1) ·
[coverage-diff-scope-evidence](coverage-diff-scope-evidence/) (B2) ·
[verify-sandboxing](verify-sandboxing/) (B3)

**C · Scale & orchestration** — [watch-daemon](watch-daemon/) (C1) ·
[distributed-state-backend](distributed-state-backend/) (C2) ·
[cost-telemetry-ledger](cost-telemetry-ledger/) (C3)

**D · Smarter gates** — [verify-revert-on-fail](verify-revert-on-fail/) (D1) ·
[custom-gate-api](custom-gate-api/) (D2) · [replay-spec-diff](replay-spec-diff/) (D3)

**E · Ecosystem & standardization** — [spec-pack-registry](spec-pack-registry/) (E1) ·
[open-spec-format](open-spec-format/) (E2) ·
[github-native-integration](github-native-integration/) (E3)
