# V9 — Deploy Driver Runner & Production Observability

## 1. Purpose and requirement coverage

Close the SDLC loop past `complete`: evidence-gated deploy driver runner with
rollback bookkeeping, inbound production-error correlation into
mid-requirements, and the feedback-flywheel e2e composition. Covers plan tasks
**P5.1, P5.2, P5.5** (P0/P1). Canary/blue-green live in the user's commands;
specd contributes gating, sequencing, evidence, rollback.

## 2. Verified current state

- Webhook machinery: `watch_webhook.go` (outbound); SSE/serve surfaces.
- Midreq machinery: `internal/cmd/midreq.go` + core midreq handling.
- Sandboxed exec path: custom-gate runner (shared surface, plan risk 3).
- Human approval: `internal/cmd/approve.go`.
- V5 eval gate, V8 review/security gates — deploy preconditions; V7 submit
  precedent for user-configured command exec.

## 3. Proposed design and end-to-end flow

- **Deploy (P5.1):** `.specd/deploy/<env>.json` —
  `{env, requiresGates: [...], steps: [{name, command, rollbackCommand,
  timeoutSeconds}], approvalRequired}`. `specd deploy <spec> --env <env>`:
  refuses unless spec `complete` + required gates (eval/security/review)
  recorded green; runs steps sequenced through the shared sandboxed exec path;
  appends every step result to `deploy.jsonl`; `specd deploy rollback` runs
  recorded inverse chain in reverse order. `--env production` additionally
  requires `specd approve --deploy` (human gate). No CD logic embedded.
- **Observe (P5.2):** `specd observe --listen` (localhost + token auth by
  default, reusing HTTP transport auth) accepts schema-validated, size-capped
  error payloads. Correlation is deterministic: payload file paths/stack
  frames matched against task `files:` lists and the recent deploy ledger →
  structured `mid-requirements.md` entry (severity mapped from payload) with
  correlation-confidence facts. `specd observe correlate <file>` for
  batch/offline (CI pipes Sentry exports) — the listener is optional, the
  transform is the feature.
- **Flywheel (P5.5):** composition, not new code: observe → midreq → human
  approve → spec → orchestrate → eval + review → submit → deploy → observe.
  Deliverables: full-loop e2e integration test with fake drivers (in
  `make ci`) + `docs/flywheel.md` operator guide.

## 4. Interfaces, contracts, data, configuration, dependencies

- **New artifacts:** `.specd/deploy/<env>.json` (human), `deploy.jsonl`
  (CLI-owned, append-only).
- **New commands:** `deploy`, `deploy rollback`, `approve --deploy`,
  `observe --listen`, `observe correlate` (registry discipline).
- **Dependencies:** V5, V7, V8 (required gates), V1. **Dependents:** V11
  (deploy templates in harness bundle), V12.

## 5. Invariants, security, errors, observability, compatibility, rollback

- Deploy config is hostile input: schema validation, timeout per step, env
  scrub, sandbox; hostile-config adversarial tests in the same PR (P4.4).
- Mid-chain failure: recorded, execution stops, rollback chain computed from
  *recorded* successful steps only — no partial ambiguity.
- Production path impossible without a human approval record (invariant:
  human at boundaries).
- Listener: localhost bind + token by default; payload size cap; path
  traversal in correlation hints rejected; malformed payloads rejected with
  reasons.
- Correlation deterministic (invariant 6): same payload + state → same midreq
  entry; entries carry the evidence trail.
- **Rollback (of the feature):** no deploy configs → commands refuse; ledger
  inert.

## 6. Acceptance criteria and validation commands

- Deploy e2e with fake driver scripts (testharness sandbox): success chain,
  mid-chain failure → correct rollback chain, gate-refusal cases,
  production-without-approval impossible.
- Correlation table tests; hostile payload suite (oversize, traversal,
  malformed) rejected.
- Flywheel loop e2e in `make ci`.
- `go test ./internal/core/... ./internal/cmd/... -run 'Deploy|Observe|Flywheel' -race -count=2`

## 7. Open decisions and deviations

- Path deviation DV1.
- Open: rollback of a *partially rolled back* chain (rollback step fails).
  Decision: record failure, halt, exit 3 (blocked) — human resolves; no
  automatic retry of rollback steps.
