# V8 Tasks — Review Workflow & Security Gates

Plan coverage: P4.1–P4.4. Dependencies: V1, V2, V5, V7. Dependents: V9, V12.

## Wave 1 — Review workflow (P4.1)

- [ ] `roles/reviewer.md` template + `specd-review` skill (same PR as
  machinery).
- [ ] `specd review <spec>`: scaffold `review_report.md` (Summary, Bugs,
  Security, Hallucinated Dependencies, Style, Verdict) + print role brief.
- [ ] `review` gate: exists, structurally valid, verdict present, newer than
  latest task completion; `config.review.required` (new inits on, migrated
  off); approve verifying→complete enforcement.
- [ ] Gate table tests (missing/stale/malformed) + e2e approve path.
- **Validation:** `go test ./internal/core/... ./internal/cmd/... -run Review -race`

## Wave 2 — Security gate suite (P4.2, depends on Wave 1)

- [ ] `internal/core/security/secrets.go`: entropy + format patterns, changed-
  file scope, allowlist with mandatory reasons.
- [ ] `injection.go`: heuristics, advisory default, blocking via config.
- [ ] `slopsquatting.go`: stdlib manifest parsing (go.mod, package.json,
  requirements.txt), edit distance vs embedded popular-package list.
- [ ] `deps` plugin gate via shared sandboxed `command` path.
- [ ] `specd check --security`; findings → `state.json` + report/PR summary
  sections (completes V7 stubs).
- [ ] Fixture corpus with >90% catch-rate tracking test; zero findings on this
  repo; large-repo benchmark.
- **Validation:** `go test ./internal/core/security/... -race -count=2 && make bench`

## Wave 3 — Checklist + threat model (P4.3 + P4.4, depends on Wave 2)

- [ ] `specd review checklist <spec>`: extract design sections + task
  contracts into checklist; fixture content assertions.
- [ ] Threat-model refresh: SECURITY.md + validation-gates.md cover eval
  command, guardrails, deploy drivers, submit, observe listener.
- [ ] Adversarial fixture suite: hostile rubric/deploy config/webhook payload
  never executed without explicit config; fuzz tests for all new parsers.
- **Validation:** `go test ./... -run 'Checklist|Adversarial|Fuzz' -count=2`

## Rollout & cleanup

- [ ] Docs: validation-gates (review + security gates, false-positive
  workflow prominent), command-reference, AGENTS.md review discipline,
  CHANGELOG; parity green.
- **Rollback:** `review.required` off; security gates advisory/off via config.
- **Completion evidence:** `make ci` green; catch-rate test committed;
  threat-model docs merged (v0.2.0 release gate).
