# V9 Tasks — Deploy & Observe

Plan coverage: P5.1, P5.2, P5.5. Dependencies: V1, V5, V7, V8.
Dependents: V11, V12.

## Wave 1 — Deploy driver runner (P5.1)

- [ ] `.specd/deploy/<env>.json` schema parse/validate (hostile-input
  hardened: unknown fields, oversize, timeouts mandatory).
- [ ] `specd deploy <spec> --env <env>`: precondition checks (complete +
  requiresGates green), sequenced sandbox-recorded steps, `deploy.jsonl`
  appends.
- [ ] `specd deploy rollback`: inverse chain from recorded successful steps,
  reverse order; halt + exit 3 on rollback-step failure.
- [ ] `approve --deploy` human gate; production env hard-requires it.
- [ ] e2e with fake drivers: success, mid-chain failure → rollback, refusal
  matrix; hostile deploy-config adversarial tests (same PR).
- **Validation:** `go test ./internal/core/... ./internal/cmd/... -run Deploy -race -count=2`

## Wave 2 — Observability inbound (P5.2, depends on Wave 1)

- [ ] Payload schema + validation (size cap, traversal rejection); severity
  mapping.
- [ ] Deterministic correlation: stack paths ↔ task `files:` lists + deploy
  ledger → midreq entry with confidence facts; table tests.
- [ ] `specd observe correlate <file>` (offline transform — the core feature).
- [ ] `specd observe --listen`: localhost + token (reuse HTTP transport auth);
  hostile payload suite.
- **Validation:** `go test ./internal/core/... ./internal/cmd/... -run Observe -race`

## Wave 3 — Feedback flywheel (P5.5, depends on Wave 2)

- [ ] Full-loop e2e: observe → midreq → approve → spec → orchestrate → eval →
  review → submit → deploy → observe, fake drivers throughout; wire into
  `make ci`.
- [ ] `docs/flywheel.md` operator guide.
- **Validation:** `make ci` (loop test included)

## Rollout & cleanup

- [ ] Docs: command-reference (deploy/observe verbs), validation-gates
  (deploy preconditions), SECURITY.md listener section (with V8 threat
  model), CHANGELOG; parity green.
- **Rollback:** remove deploy configs; observe unused without invocation.
- **Completion evidence:** `make ci` green incl. flywheel e2e; production
  metric "every observed error → midreq entry with evidence" test committed.
