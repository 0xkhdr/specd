# Implementation Plan — RECOMMENDATIONS.md follow-ups

Derived from `RECOMMENDATIONS.md` (2026-07-09). Six items: three P1 (schedule now),
three P2 (polish/hardening). Ordered below by **do-first leverage**, not by the doc's
listing order. Each item names the code touched, the approach, the verify line, and the
guardrails it must not violate (determinism, evidence integrity, zero deps, docs-sync).

Standing invariants apply to every item: no new runtime deps, atomic writes + CAS on
`state.json`, no LLM in any gate/DAG/decision path, `reference/` untouched, and
`docs/command-reference.md` ↔ `docs/CHEATSHEET.md` kept in sync (`docs-lint.sh`).

---

## Sequencing

```
Phase A (cheap, unblocks / de-risks):
  R3  scaffold project.yml + bounded verify default
  R6  cmd-level test: timed-out verify → failing evidence record   (validates R3 wiring)
  R5  Go test: completion-path lease release
  R4  decide ConfigPaths.Global: wire XDG global OR drop the field

Phase B (design + doc, medium):
  R2  brain run autonomy boundary — decide, then doc or implement

Phase C (large, own spec):
  R1  dogfood: migrate top-level specs/ into .specd/specs/ pipeline
```

Rationale: R3 makes the timeout bound visible by default; R6 pins that the config→exec→
evidence loop actually records exit 124, so ship them together. R4/R5 are small and
independent. R2 is a decision first (one wave vs run-to-done) — cheap if we pick "doc",
larger if "implement". R1 is the highest-value but biggest item; it should itself be
driven through the pipeline it migrates, so it goes last.

---

## R3 (P1) — Scaffold a bounded `project.yml` in `specd init`

**Problem.** `verify.timeout_seconds` defaults to `0` (unbounded — `VerifyConfig.TimeoutSecs`,
`config_loader.go:57-63`) and `specd init` writes no `project.yml` at all (`WriteScaffold`,
`scaffold.go:11` writes managed assets + `AGENTS.md` + optional pinky, never `project.yml`).
So in a fresh project the safety bound is both off and invisible.

**Approach.** Add a `writeProjectConfig(root)` step to `WriteScaffold` that writes a
**commented** `project.yml` — idempotent, do not clobber an existing one (mirror the
`writeAgents` existing-file check). Ship a conservative `verify.timeout_seconds: 600`
active, and the other opt-in gates (`gates.verify`, `escalation.max_verify_fails`,
`orchestration.enabled`, `security.*`, `criteria.required`, `review.required`) documented
as commented lines so operators see every knob. Keep it parseable by `parseSimpleYAML`
(two-space indent, `key: value`, `#` comments — already supported, `config_loader.go:169`).

- Files: `internal/core/scaffold.go` (new writer + call in `WriteScaffold`); embed the
  template body — a `const` string or a `go:embed` file under `embed_templates/`
  (prefer `go:embed` to match how AGENTS.md/roles are handled).
- `previewManaged` / `--dry-run` init should report the `project.yml` it would write.
- **Do not** change the loader default from `0`; unbounded stays the non-breaking default
  for projects with no file. The scaffold is what makes fresh projects bounded.
- Docs-sync: `specd init` behavior note in `command-reference.md` + `CHEATSHEET.md`.

**Verify.** `go build -o specd . && cd $(mktemp -d) && /path/specd init && grep -q
'timeout_seconds' project.yml && rerun init && diff` (idempotent, second run no-ops).
Add a `cmd`-level test asserting `init` writes `project.yml` with the bound and that a
second `init` preserves an operator-edited file.

**Effort.** Small. Highest discoverability win.

---

## R6 (P2, do with R3) — Test that a timed-out verify is *recorded* as failing evidence

**Problem.** `verify/timeout_test.go` proves `Run` returns exit 124 on deadline, but the
wiring `runVerify` (`registry.go:556`, `TimeoutSecs: verifyTimeoutSecs(root)` at :599) →
`AppendEvidence` has no end-to-end test. Config→exec→evidence is unproven as a loop.

**Approach.** New `cmd`-level test: seed a spec with a task whose `verify:` is `sleep 5`,
write `project.yml` with `verify.timeout_seconds: 1`, run `specd verify <slug> <task>`,
assert (a) the evidence record carries exit 124 and (b) the task does **not** reach
`TaskComplete`. Deterministic; no sleep-race because the timeout fires well before 5s.

- Files: `internal/cmd/*_test.go` (new test alongside existing verify tests).
- Reuse the existing e2e harness/fixtures pattern (`TestLifecycleE2E` and friends).

**Verify.** `go test ./internal/cmd -run TestVerifyTimeout -count=1 -race`.

**Effort.** Small. Pairs naturally with R3 (both about the timeout path).

---

## R5 (P2) — Focused Go test for the completion-path lease release

**Problem.** `runBrainStep` releases leases for tasks that reached `TaskComplete`
(`brain_run.go:89-101`, gap 5.4), but the only coverage is transitive via
`stress-orchestration.sh`, which exercises `brain resume`'s clearing — not the
in-step completion release.

**Approach.** New Go test: seed a session with a live lease on `T1`, mark `T1` complete
in `tasks.md` (or in the status source `taskStatus` reads), run one `runBrainStep`, assert
`session.Leases` no longer contains `T1`. Pin exactly the `status[lease.TaskID] ==
core.TaskComplete` branch.

- Files: `internal/cmd/brain_run_test.go` (or existing brain test file).

**Verify.** `go test ./internal/cmd -run TestBrainStepReleasesCompletedLease -count=1 -race`.

**Effort.** Small, deterministic.

---

## R4 (P2) — Resolve dead `ConfigPaths.Global`

**Problem.** Every CLI config load sets only `Project: <root>/project.yml`
(`registry.go:540`, and `brain_run.go:167`, plus `criteria.go`, `escalation.go`,
`memory.go`). `LoadConfig` honors `paths.Global` (`config_loader.go:146`) but no CLI path
populates it. A documented layering feature has no way to reach it.

**Decision required — two options (pick one, do not leave both):**

- **(A) Drop it (subtractive bias, recommended default).** Remove the `Global` field from
  `ConfigPaths`, collapse the loop in `LoadConfig` to project + env, delete the
  `project-diagnostics-config` decision note's "global YAML layered under" claim. Smallest
  diff; removes a phantom feature.
- **(B) Wire it.** Populate `Global` from `$XDG_CONFIG_HOME/specd/config.yml` (fallback
  `~/.config/specd/config.yml`) in a single shared config-path helper, and route every CLI
  caller through it so the helper is the one source of truth. Only if machine-wide defaults
  are a real user need.

Recommend **(A)** unless a user asks for machine-wide defaults. If (B), add a loader test
that global is layered *under* project (project wins) and env wins over both.

- Files: `internal/core/config_loader.go`; all CLI callers listed above (introduce one
  `configPaths(root)` helper either way to kill the duplication).
- Docs-sync only if the config surface is documented in the command reference.

**Verify.** `go test ./internal/core -run TestConfig -count=1` + `go vet ./...`. For (A),
grep confirms no remaining `.Global` references.

**Effort.** Small.

---

## R2 (P1) — Define `brain run`'s autonomy boundary

**Problem.** `runBrainRun` (`brain_run.go:147`) loops `runBrainStep` until the decision
action is not `ActionDispatch`, dispatching every currently-ready unleased task, then
stopping. Workers are **external** `pinky-*` subagents; the controller never blocks on a
worker report and re-senses. So `run` is "dispatch the current wave," not "run to
completion" — honest but easy to misread.

**Step 1 — decide (this is a fork, resolve before coding):**

- **(A) `run` = one wave (doc-only, recommended default).** Keep the code; fix the mental
  model in `command-reference.md` so operators don't expect run-to-done. Cheapest, ships
  now, honest.
- **(B) `run` = run-to-completion.** Add a worker round-trip inside the loop: block on
  evidence for the leased task, then re-sense the frontier. **Constraint:** the worker call
  is the *only* non-deterministic step — the LLM must stay out of Decide/Sense (invariant).
  This is a real design (how the controller invokes/awaits an external worker, timeout,
  crash recovery of a mid-flight wait). Larger; likely its own spec.

Recommend **(A)** now; revisit (B) only if operators actually want unattended run-to-done.

**Step 2 — execute the choice:**
- (A) Update `command-reference.md` `brain` section (line ~419-423, "Run the opt-in…") to
  state `run` dispatches the ready wave and returns; operators re-invoke after workers
  report. Mirror into `CHEATSHEET.md`. Optionally tighten the `runBrainRun` doc-comment.
- (B) Design doc / spec first, then implement behind the same fail-closed `--authority`.

**Verify.** (A) `./scripts/docs-lint.sh` green. (B) new brain suite tests + stress script.

**Effort.** (A) trivial. (B) large.

---

## R1 (P1) — Dogfood: migrate top-level `specs/` into `.specd/specs/`

**Problem.** The nine gap-closure specs live in top-level `specs/` as hand-maintained
markdown (`specs/{agent-workflow-mcp,cli-contracts,concurrency-isolation,context-manifest,
evidence-security,init-host-scaffold,orchestration-workers,project-diagnostics-config,...}`
plus `AUDIT-FINDINGS.md`, `progress.md`). No `state.json`, no verify record, no CAS, no
lock. The tracking drift the audit fixed (`progress.md` over-claiming) is the exact failure
mode of manual tracking — what the product exists to prevent.

**Approach (own spec — do last).**
1. **Schema map.** Top-level uses `wave`/`deps`; runtime expects the on-disk `.specd/`
   layout (`.specd/specs/<slug>/{requirements.md,design.md,tasks.md,state.json,.lock}`).
   Write the translation once: `wave`→DAG `deps`, section headers → EARS/design/tasks
   schema the gates expect (`internal/core/gates/`).
2. **Migrate per slug** into `.specd/specs/`, then drive each through `specd check` →
   `specd verify` → `specd approve` so per-task evidence is captured automatically and
   `progress.md` stops being hand-maintained (it becomes generated from `state.json`).
3. **Retire** the top-level hand-tracking once the pipeline is authoritative; keep
   `AUDIT-FINDINGS.md` as a historical record if useful, but drive live work from `.specd/`.
4. Watch `regress-lint.sh` smell "A" (verify lines targeting the wrong specs tree) — the
   split between repo-planning `specs/` and runtime `.specd/specs/` is exactly what it
   guards; migration must not blur it incorrectly.

**Verify.** Each migrated task carries a passing verify record pinned to git HEAD; `specd
status` reflects real state; `./scripts/regress-all.sh` and `regress-domains.sh` green.

**Cost.** Non-trivial (schemas differ). Do it once, as its own spec — this is the single
highest-leverage change: it makes the product prove itself.

---

## Definition of done (all items)

- `gofmt -l .` empty, `go vet ./...` clean, `go mod tidy` no diff (zero new deps).
- `go test ./... -race -count=1` and `-count=2` (iteration-order) green.
- `./scripts/docs-lint.sh` green for any CLI/flag/doc change (R2, R3).
- `./scripts/regress-all.sh`, `regress-domains.sh`, `regress-lint.sh` green.
- Evidence integrity preserved: no bypass flag added anywhere.
