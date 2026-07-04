# regression-specs — gap audit & wave manager

> **Purpose.** Records the regression audit of the *implemented* specd rebuild against the
> **original MVP intent** (`fresh-start/00-scope-triage.md` — the minimal accurate surface of
> **16 verbs**) and the frozen v1 in `reference/`. Where the implementation fell short of an
> intended-KEEP capability, this directory carries a remediation spec in the same blueprint as
> `specs/<NN-domain>/{spec.md,tasks.md}`.
>
> **Method.** Built the current binary, enumerated `internal/core/commands.go` (the live
> palette) and `reference/internal/core/commands.go` (32 v1 verbs), classified every
> reference-only verb through `fresh-start/00-scope-triage.md`, and grep-verified feature
> presence in `internal/` for each MVP-KEEP capability. Findings below are reproduced from
> source + the running binary, not read from `specs/progress.md`'s checkboxes.
>
> **Guardrails inherited:** determinism first (no LLM in any decision/gate/render path);
> evidence integrity (ADR-8); zero runtime deps; subtractive bias.

---

## 1. Audit result (headline)

The MVP-16 surface is **substantially implemented** and the CLI-seam regression
(`specs/13-cli-regression`, findings F1–F8) **has landed**: the dispatcher fails closed, the
parity guard `TestEveryCommandHasHandler` exists, and the end-to-end golden `TestLifecycleE2E`
exercises `init → new → check → approve → next → verify → report` with on-disk assertions.

**One MVP-KEEP subsystem was never built, and one MVP surface flag is absent:**

| Gap | Intent verdict | Evidence | Spec |
|---|---|---|---|
| **Steering Memory & Promotion** (`memory <slug> add\|promote`, per-spec `memory.md`, `PromotionThreshold`) | **KEEP** — P8; `00-scope-triage` "memory KEEP add/promote; absorbs `promote`" | `.specd/steering/memory.md` template ships, but no `memory` verb, no `PromotionThreshold` config, no per-spec `memory.md` flywheel. `reference/internal/cmd/memory.go` has no counterpart in `internal/`. | **R1** |
| **Context HUD** (`context <task> --hud`) | **KEEP-lite** — Domain 08 human surface over the existing estimator | `internal/context/{manifest,estimate,budget}.go` compute the data; `context` exposes only `--json`. `--hud` (files + byte/token cost + mode/tier) absent. | **R2** |

## 2. What is verified present (no spec needed)

`init new approve midreq decision next status task check verify context mcp handshake brain
report` + `pinky` — all registered with non-nil handlers. Feature spot-checks that passed:
`next --dispatch` (dispatch packets in `orchestration/`), `verify` sandbox fail-closed
(`verify/exec.go`), `report --pr` (`report`'s PR-summary renderer) and `--metrics`
(`report_metrics.go`), `check --security` (`gates/security/`), config cascade
(`config_loader.go`). The steering **store** `.specd/steering/memory.md` is scaffolded by
`init` — only the flywheel that writes to it (R1) is missing.

## 3. Accepted deferrals (reference features intentionally NOT specced here)

Classified CUT/DEFER by `fresh-start/00-scope-triage.md`; listed so a reviewer can pull any
into MVP by request. **Not** gaps — deliberate subtraction.

`conductor · dashboard · deploy · eval · harness · ingest · migrate · observe · orchestrate ·
program (cross-spec tier) · review · submit · watch/--webhook streams · spec packs
(init --pack/--registry) · prototype specs (new --prototype)`. Also minor: `version` verb,
`check --schema` (open-spec JSON-Schema of `state.json`) — folded out with ADR-2's schema reset.

> **Reviewer action:** if any deferral above should be MVP, say which and a regression-spec
> will be authored for it.

---

## 4. Regression waves (implementation)

Legend: ⬜ pending · 🟡 in progress · ✅ done (verify passed + record written). Both specs are
opt-in-neutral additions; neither changes an existing byte-identical `state.json`.

### Wave R-A — Steering Memory (R1)
| task | files | verify | state |
|---|---|---|---|
| TR1.1 | `internal/core/memory.go` | `go test ./internal/core -run TestMemoryBlock` | ✅ |
| TR1.2 | `internal/core/config_loader.go`, `config_validate.go` | `go test ./internal/core -run TestPromotionThreshold` | ✅ |
| TR1.3 | `internal/core/paths.go` | `go test ./internal/core -run 'TestSpecMemoryPath\|TestListSpecs'` | ✅ |
| TR1.4 | `internal/cmd/memory.go`, `internal/core/commands.go`, `internal/cmd/registry.go` | `go run . memory demo add --key k --pattern p --body b --source s --criticality minor && grep -q '## k' .specd/specs/demo/memory.md` | ✅ |
| TR1.5 | `internal/cmd/lifecycle.go` | `go run . new demo && test -f .specd/specs/demo/memory.md` | ✅ |
| TR1.6 | `internal/cmd/memory_test.go` | `go test ./internal/cmd -run TestMemoryPromoteFlywheel` | ✅ |

### Wave R-B — Context HUD (R2)
| task | files | verify | state |
|---|---|---|---|
| TR2.1 | `internal/context/hud.go` | `go test ./internal/context -run TestHUDRender` | ⬜ |
| TR2.2 | `internal/core/commands.go`, `internal/cmd/registry.go` | `go run . context <task> --hud \| grep -qi total` | ⬜ |
| TR2.3 | `internal/context/hud_test.go` | `go test ./internal/context -run TestHUDMatchesJSON` | ⬜ |

## 5. Progress rollup
| Wave | Tasks | Done | State |
|---|---|---|---|
| R-A | 6 | 6 | ✅ |
| R-B | 3 | 0 | ⬜ |
| **Total** | **9** | **6** | **67%** |

## 6. Definition of done (per task — ADR-8)
- [ ] Its `verify:` command passes with a written record (exit code + git HEAD).
- [ ] It touches only the `files:` its task declares.
- [ ] Guardrails hold: determinism (promotion count + HUD render are pure functions of on-disk
      state), zero deps, atomic write / CAS / lock discipline preserved, parity guard
      `TestEveryCommandHasHandler` stays green.
