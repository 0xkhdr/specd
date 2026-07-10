# Requirements — Deployment and production assurance

## Scope

Add a deterministic, additive production-delivery domain around the existing lifecycle: a
sufficient fail-closed agent bootstrap, a reachable orchestrated mode with a production-wired brake,
release/environment/deployment/health/rollback envelopes with an explicit state machine, immutable
release and deployment ledgers, environment policy and pure delivery gates, an optional stdin/file
adapter protocol, canary/health/promotion/rollback evidence and reports, CI delivery binding with
attested identity, and install/upgrade hardening plus incident/portfolio feedback. Preserve atomic
writes, state CAS, reentrant spec lock, append-only evidence, no-bypass verify, byte-stable tasks
parser, offline/stdlib-only core, `go:embed` templates, backward-compatible decoding, and the
unchanged six-phase status ratchet. No LLM in any delivery gate, transition, or report. No new
runtime dependency. No implicit network in core.

### R1 — Delivery contract envelopes and state transitions

- R1.1: A versioned schema shall define release, environment, deployment, health, and rollback
  envelopes with the contract fields (`spec_id`, `task_evidence_set`, `release_id`, `git_head`,
  `artifact_digest`/`sbom_ref`/`provenance_ref`, `environment`, `deployment_id`/`attempt`,
  `actor`/`authority`/`adapter`, timestamps/`status`, `strategy`/`population`/`window`,
  `health_check`/`threshold`/`observation`, `release_baseline`, `rollback_target`/`reason`/`status`,
  `evidence_ref`/`attestation_ref`, `state_schema`/`spec_revision`). Canonical fixtures shall
  validate with networking disabled.
- R1.2: Deployment status shall be a closed set — `requested`, `started`, `observing`, `healthy`,
  `failed`, `rolling_back`, `rolled_back` — and every transition shall fail closed. Unknown schema
  versions, environment names, state jumps, mismatched HEAD/artifact, stale health, and a missing
  rollback target shall be rejected before any mutation.
- R1.3: Delivery envelopes shall be strictly additive and separate from task completion evidence.
  No delivery record shall satisfy a task evidence gate, and no delivery state shall be added to the
  six lifecycle status values.

### R2 — Agent bootstrap binds all identities (drift-safe)

- R2.1: One bootstrap response shall bind binary version/commit, state/context/template schema
  versions, workspace root, active spec/status/revision, palette/config/managed-asset digests,
  allowed tools, and the exact next valid command(s).
- R2.2: Any pinned mismatch — binary, schema, workspace root, slug, revision, palette, config, or
  managed role/steering content — shall exit non-zero before any state mutation with an actionable
  message naming the failed precondition and current identity.
- R2.3: The bootstrap shall distinguish harness instructions from untrusted requirements, source,
  test output, and adapter observations. External text shall not amend authority or policy. Managed
  role/steering content shall be digested so stale guidance driving a newer binary is caught.

### R3 — Orchestrated-mode reachability and production brake

- R3.1: A supported CLI/config path shall enter `orchestrated` mode through CAS and an approval
  transition. `orchestrated` shall be a declared, schema-validated mode; an invalid mode shall fail
  schema validation. No test or guide shall require hand-editing `state.json`; default-mode behavior
  shall be unchanged.
- R3.2: The orchestration cost/deadline brake shall be wired end to end so a driver cannot keep
  dispatching without the economic controls the types imply, or the dormant public implication shall
  be removed/deferred. `Sense` shall populate cost from accepted telemetry, not leave it unset.
- R3.3: When production policy requires trusted/attested telemetry or authority and it is missing or
  malformed, dispatch shall fail closed with one actionable message; a brake on untrusted data shall
  say so.

### R4 — Installed-lifecycle E2E and regression prerequisites

- R4.1: A `production-smoke.sh` lane shall, from an empty temp git repo, run the installed binary
  through init/new/approvals/context/verify/complete/review/submit using only advertised commands,
  with every mutation through the CLI and a deliberately invalid step failing closed with the
  documented next action.
- R4.2: Every advertised regression invariant shall first prove its input exists and was parsed. A
  missing input (e.g. `specs/progress.md` in `regress-domains.sh` W0) shall fail or be explicitly
  skipped by declared policy, never reported as a pass. The harness shall distinguish "input
  absent", "check not applicable", and "check passed".

### R5 — Release install/upgrade hardening

- R5.1: CI shall install an actual just-built GoReleaser archive, verify checksum and attestation,
  confirm `version --json` commit, and run a handshake/init smoke on each supported OS/architecture
  with networking disabled after installation.
- R5.2: Upgrade shall stage and verify the new binary, retain the previous binary, preview
  managed-asset changes, atomically swap, and auto-restore the previous binary on failed post-install
  smoke. A corrupt staged binary shall never be promoted over the working binary.
- R5.3: A schema compatibility preflight shall run before mutation: an older workspace shall load
  under the new binary with evidence byte-preserved; an unsupported future state/delivery schema and
  an unsafe downgrade shall exit before writes with exact supported versions and recovery guidance.

### R6 — Release-candidate and deployment ledgers

- R6.1: `specd release candidate` shall freeze an immutable, reproducible candidate identity from
  spec revision, git HEAD, required evidence-set digest, artifact digest, SBOM/provenance refs, and
  bootstrap digest into `.specd/specs/<slug>/releases.jsonl`. It shall not build or upload artifacts.
- R6.2: Deployment attempts shall append to `.specd/specs/<slug>/deployments.jsonl` under the spec
  lock. Append/replay shall survive concurrency and crash — a torn write yields the prior complete
  record or one complete new record, never a partial line or duplicate deployment/attempt.
- R6.3: No deployment record shall satisfy a task evidence gate or change `complete` retroactively.
  The evidence gate shall pass or fail identically whether delivery ledgers are present or absent.

### R7 — Environment policy and pure delivery gates

- R7.1: Environment policy shall declare, per closed environment name, the strategy, required
  approver/identity, health criteria, observation window, freshness, and rollback target. The same
  policy and evidence shall always yield the same verdict.
- R7.2: Production shall require an explicit adapter/authority, artifact identity, observation
  freshness, and rollback target; lower environments may opt out without weakening any task gate.
  Production policy shall not be relaxable by task text, agent prompt, or adapter response.
- R7.3: Delivery gates shall be pure and offline. An artifact substituted after candidate creation
  shall fail a digest check; PR checks shall not be able to impersonate production delivery.

### R8 — Deployment adapter envelope protocol

- R8.1: An optional stdin/file adapter protocol shall let an external deploy system return a
  versioned, bounded evidence envelope that core validates and appends under the spec lock. The
  adapter shall receive no implicit credentials from `specd`; provider credentials shall never enter
  `.specd/`. Core shall gain zero new dependencies and make no network call.
- R8.2: A duplicate idempotency key shall be a no-op or conflict, never a second deployment.
  Conflicting payloads under one key shall fail closed and preserve both audit facts safely.
- R8.3: Malformed or untrusted envelopes shall be rejected. A hostile prose or credential in an
  envelope shall be treated as data, bounded/redacted in storage, and never placed in standing agent
  instructions. Trust source/attestation shall always be visible and allowlisted.

### R9 — Canary/health/promotion/rollback evidence and reports

- R9.1: A canary shall stay `observing` until every required criterion has fresh evidence from an
  allowed adapter for the exact deployment/artifact/environment over the full declared window.
  Missing, malformed, stale, or wrong-release observations shall fail closed — never healthy by
  timeout default.
- R9.2: Passing criteria shall permit promotion recording the original evidence references and
  baseline. A failed criterion shall require either a separately governed human-recorded exception
  or rollback; an exception shall never rewrite task evidence or claim the deployment was healthy.
- R9.3: Rollback shall record the failed release, target release, reason, adapter identity, action
  result, and post-rollback health; it is complete only after target health passes. An explicit
  rollback target and a capability classification shall be required; where reversal is not
  demonstrably safe, a human-required strategy shall be supported. Repeated reports shall be
  byte-identical.

### R10 — CI delivery binding and attested identity

- R10.1: An optional CI delivery action shall bind source evidence to a built artifact digest, SBOM,
  provenance, target environment, and deployment attempt. Swapping the artifact after candidate
  creation shall fail the digest check.
- R10.2: PR checks shall not impersonate production delivery, and fork PRs shall receive no
  production credentials.
- R10.3: Externally attested CI/runtime identity shall be verifiable offline using only Go stdlib
  cryptography. A tampered envelope, wrong repository/environment/audience, expired assertion, or
  untrusted key shall fail.

### R11 — Incident feedback, portfolio views, and recovery drills

- R11.1: An incident/observation reference shall seed a new spec recording source
  release/deployment/criterion and bounded evidence refs. The raw external payload shall not be
  loaded by default; original ledgers shall stay immutable.
- R11.2: A deterministic portfolio view shall identify the deployed/healthy/failed/rolled-back
  release per environment and cross-spec blockers, performing no discovery network calls.
- R11.3: A compatibility matrix and recovery drills shall prove N-1 → N upgrade preserves
  state/evidence, unsupported future schema and unsafe downgrade fail before writes, and a crash at
  each swap/checkpoint boundary recovers old or new complete state — never a partial installation.

## Non-goals

- No cloud/Kubernetes/GitHub/model-runtime deployment engine in core — optional adapters only, with
  explicit credentials and audit boundaries.
- No change to task completion authority; deployment/health/telemetry/rollback evidence never
  substitutes for verify evidence at a git HEAD.
- No autonomous production approval by an LLM; policy/human/CI identity governs promotion and
  exceptions.
- No assumption that `complete` means deployed; source completion and environment health are
  separate states in every report.
- No raw production telemetry in default context or state; store bounded facts and references.
- No implicit network access; core stays deterministic, local, stdlib-only, air-gappable.
- No new deploy state on the six lifecycle status values; delivery is a parallel ledger.
- No `reference/` edits.
