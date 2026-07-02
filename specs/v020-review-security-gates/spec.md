# V8 — Review Workflow & Security Gate Suite

## 1. Purpose and requirement coverage

AI-first review under the "demand the artifact" pattern (§5.5) plus a
deterministic built-in security gate suite, checklist generator, and the
release-gating threat-model refresh. Covers plan tasks **P4.1–P4.4** (P0/P1).
The binary never judges code — it makes skipping the judgment impossible.

## 2. Verified current state

- Approve ratchet: `internal/cmd/approve.go`; gate pipeline
  `internal/core/gates.go`; check command `internal/cmd/check.go`.
- Roles: `internal/core/role-adjacent` templates (see `internal/spec/role.go`),
  skills shipped via init/pack machinery.
- Fuzz precedent: `internal/mcp/host_caps_fuzz_test.go`; bench-test pattern
  exists for large-repo scans.
- `SECURITY.md`, `docs/validation-gates.md` current as of v0.1.x surfaces.
- V2 guardrails, V5 eval `command`, V7 submit exec surfaces exist and need
  threat-model coverage here.

## 3. Proposed design and end-to-end flow

- **Review workflow (P4.1):** new role template `roles/reviewer.md` (read-only,
  adversarial checklist) + `specd-review` skill. `specd review <spec>`
  scaffolds `review_report.md` (mandatory sections: Summary, Bugs, Security,
  Hallucinated Dependencies, Style, Verdict `approve|revise`), prints the role
  brief. New `review` gate: report exists, structurally valid, verdict
  present, **newer than latest task completion** (staleness check).
  `specd approve` verifying→complete blocked without it
  (`config.review.required`: on for new inits, off for migrated repos).
  Human approval stays final — the report is evidence, not decision.
- **Security suite (P4.2):** `internal/core/security/` — all deterministic,
  all stdlib, pure functions over file contents:
  - `secrets`: entropy + known-format patterns (AWS keys, PEM, JWT) over
    changed files; allowlist `.specd/security/allow.json`, entries require a
    reason string.
  - `injection`: pattern heuristics (concatenated SQL, exec of interpolated
    input); advisory by default, blocking via config.
  - `slopsquatting`: manifest dep names (go.mod, package.json,
    requirements.txt — stdlib-parsed) edit-distance-checked against a shipped
    popular-package list.
  - `deps` CVE scan: plugin gate (`command` kind) → osv-scanner/grype; no CVE
    database embedded.
  `specd check <spec> --security` runs the suite; findings recorded in
  `state.json` + rendered in reports/PR summary.
- **Checklist generator (P4.3):** `specd review checklist <spec>` —
  deterministic extraction of `design.md` sections + `tasks.md` contracts into
  a human checklist; extraction only, zero interpretation.
- **Threat-model refresh (P4.4):** update `SECURITY.md` +
  `docs/validation-gates.md` for every new exec/network surface (eval
  `command`, guardrail scans, deploy drivers, submit command, observe
  listener): env allowlist, sandbox, input validation, listeners
  localhost+token by default. Adversarial fixtures: hostile rubric, hostile
  deploy config, hostile webhook payload (oversize, path traversal). Fuzz all
  new parsers. **Release gate for v0.2.0.**

## 4. Interfaces, contracts, data, configuration, dependencies

- **New artifacts:** `review_report.md` (agent-authored, structure-gated),
  `.specd/security/allow.json` (reasons mandatory).
- **New commands:** `review`, `review checklist`, `check --security`.
- **Dependencies:** V1 (findings in state), V2/V5/V7 (surfaces to
  threat-model). **Dependents:** V9 (deploy requires review+security green),
  V12 (release gate).

## 5. Invariants, security, errors, observability, compatibility, rollback

- Security gates deterministic + benchmarked on large repos — a noisy gate
  gets disabled, which is worse than a modest one (plan risk 2): advisory
  defaults where noted, allowlists-with-reasons, false-positive workflow
  documented prominently.
- Review gate staleness prevents rubber-stamping old reports.
- Hostile fixtures must never execute without explicit config.
- Compat: `review.required` off for migrated repos.
- **Rollback:** config flags off; report files inert.

## 6. Acceptance criteria and validation commands

- Review gate table tests (missing/stale/malformed report, absent verdict);
  e2e verifying→complete path with and without required flag.
- Security fixture corpus: **>90% catch rate tracked in a test**; zero
  findings on this repo itself; allowlist entry without reason rejected.
- Checklist: content assertions from fixtures (no golden files).
- Adversarial suite green: hostile rubric/deploy/webhook fixtures rejected;
  fuzz targets for every new parser.
- `go test ./internal/core/... ./internal/cmd/... -run 'Review|Secur|Secrets|Inject|Slop' -race -count=2`

## 7. Open decisions and deviations

- Path deviation DV1 (`internal/core/security/`).
- Open: popular-package list size/refresh cadence for slopsquatting.
  Decision: shipped static list embedded at build, refreshed per release —
  no network fetch (invariant 2/3).
