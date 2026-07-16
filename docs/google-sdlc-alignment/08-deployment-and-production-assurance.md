# Domain 08 — Deployment and Production Assurance

> **Status:** Historical assessment; proposals are non-normative.
> **As of commit:** `f62f16f44f92de5fa59a9304b8b10b0721564eaa` (2026-07-10).
> **Superseded by:** [`specs/11-workflow-coherence`](../../specs/11-workflow-coherence/README.md) and current normative docs.

## Purpose

Define how a `specd`-managed change graduates from evidence-backed source work to an identifiable,
observable, and recoverable production release without teaching a coding agent to bypass the
existing lifecycle. The domain covers CI/CD, release and environment identity, deployment,
health observation, rollback evidence, canaries, agent-driver safety, and install/upgrade/recovery
testing for the `specd` binary itself.

The compatibility boundary is deliberate: `specd` should govern delivery contracts and evidence,
while deployment platforms remain external adapters. The current six-phase spec lifecycle stays
the source-development ratchet; production delivery is a post-submit assurance domain, not a new
way to mark an unverified task complete.

## Paper position

Google's paper defines deployment as one of the essential parts that turns an agent prototype
into a service: hosting, identity, observability, and production infrastructure
(`sdlc-paper.md:95-112`). It expects AI-aware delivery systems to monitor deployment health,
automatically roll back problematic releases, and use production behavior as feedback for future
development (`sdlc-paper.md:228-234`). In the harness view, deterministic hooks and
observability remain active after code is written so engineers can audit deployment decisions
(`sdlc-paper.md:258-317`).

For production agents, the paper raises the bar further: persistent state, scoped permissions,
eval coverage, deployment, observability, governance, and an end-to-end
build-evaluate-deploy-observe-refine loop (`sdlc-paper.md:384-414`). It also warns teams to make
the prototype/production boundary explicit and build the production substrate—CI evals, traces,
permissions, and security review—before scaling (`sdlc-paper.md:468-500`).

The paper does not require every harness to deploy workloads itself. Full adherence can be
achieved by a vendor-neutral, fail-closed evidence protocol that CI/CD and runtime adapters use.

## Current specd handling with evidence paths

### What is already production-oriented

| Current capability | Evidence | Assessment |
|---|---|---|
| Deterministic lifecycle and human phase approvals | `internal/core/phases.go`, `internal/core/state.go`, `internal/cmd/lifecycle.go`, `docs/concepts.md` | Requirements, design, tasks, execution, verification, and reflection form a forward-only ratchet. State uses atomic writes, CAS revision, and per-spec locking. |
| Completion evidence | `internal/core/evidence.go`, `internal/core/task_complete.go`, the evidence gate in `internal/core/gates/core.go` | No task completes without exit-0 verify evidence pinned to a resolvable git HEAD; there is no bypass flag. |
| Acceptance/review/submission surfaces | `internal/core/criteria.go`, `internal/cmd/review.go`, `internal/core/submit.go`, `internal/cmd/submit.go` | Criterion evidence and optional review gates improve pre-merge assurance; submission refuses blockers. |
| CI gate integration | `.github/actions/specd-pr/action.yml`, `docs/github-action.md` | Composite action renders a deterministic PR summary, runs `specd check`, upserts one comment, and lets gate exit status—not comment text—control CI. |
| Repository CI | `.github/workflows/ci.yml`, `TESTING.md` | Lint, vet, dependency checks, race/order tests, coverage floor, security tooling, cross-platform builds, and crash/concurrency stress jobs run before merge. |
| Release pipeline | `.github/workflows/release.yml`, `.goreleaser.yml`, `docs/versioning-policy.md` | Tags rerun the full suite, build static archives, publish SHA-256 checksums and SBOMs, and inject version/commit/date into binaries. |
| Install/update safety baseline | `scripts/install.sh`, `scripts/uninstall.sh`, `scripts/install-scripts-test.sh`, `docs/user-guide.md` | Installer detects OS/arch, verifies archive checksum, refuses overwrite without `--update`/`--force`, supports dry-run, and tests basic install/update/uninstall failure paths. |
| Runtime identity | `internal/version/version.go`, `internal/cmd/version.go` | `specd version --json` exposes version, commit, date, Go, OS, architecture, and dirty state. |
| Agent bootstrap and drift detection | `internal/core/embed_templates`, `internal/core/managed.go`, `internal/core/handshake.go`, `docs/mcp-guide.md` | `init` writes managed `AGENTS.md`/roles/steering. Handshake digests the command palette and effective config, allowing clients to detect tool/config drift. |
| Tool/authority safety | `internal/core/manifest_tools.go`, `internal/mcp/server.go`, `internal/orchestration/authority.go`, `internal/cmd/brain_run.go` | MCP refuses selected state-changing/session tools. Brain dispatch requires opt-in config, an authority flag, and mode preconditions. |
| Orchestration recovery | `internal/orchestration/checkpoint.go`, `internal/orchestration/recover.go`, `internal/orchestration/session.go`, stress scripts | Write-ahead checkpoints, deterministic mission IDs, leases, CAS, and fail-closed conflict handling protect against duplicate dispatch after a crash. |
| Verify isolation | `internal/core/verify/exec.go`, `SECURITY.md` | Scrubbed environment, optional fail-closed `bwrap` sandbox, timeout, output bounds, and revert-on-failure constrain hostile verify commands. |

### What does not exist yet

There is no first-class release/deployment ledger, environment registry, canary policy,
post-deploy health gate, rollback command/evidence, or production observation feedback into a
spec. Current CI validates the source workflow and current release automation publishes the
`specd` binary; neither proves that an arbitrary workload was safely deployed and observed.

## Common contract and fields

The production delivery contract should be additive and separate from task completion evidence.
An external deploy system performs actions; `specd` validates identities, required transitions,
and recorded evidence.

| Contract field | Paper meaning | Current specd mapping | Target rule |
|---|---|---|---|
| `spec_id` / `slug` | Intent and change boundary | State/report scope | Required for release candidacy and links back to approved requirements/design/tasks. |
| `task_evidence_set` | Build/eval readiness | Git-pinned verify and criterion records | Content digest of all required passing evidence; immutable after release creation. |
| `release_id` | Deployable release identity | Version/commit for `specd` binary only | Required, unique, immutable. Prefer artifact digest plus human version; never environment name alone. |
| `git_head` | Source revision | Verify, state records, version info | Required and must match the release candidate's evidence set. |
| `artifact_digest`, `sbom_ref`, `provenance_ref` | Supply-chain identity | Checksums/SBOM in GoReleaser | Required according to environment policy; references must be content-addressed or workspace-relative. |
| `environment` | Target risk boundary | Absent | Closed config-defined name (`dev`, `staging`, `production`, etc.); policies are environment-specific. |
| `deployment_id`, `attempt` | One delivery action/retry | ACP mission/attempt only | Adapter-provided deployment identity plus harness monotonic attempt; idempotency key required. |
| `actor`, `authority`, `adapter` | Who/what may deploy | Host actor, Brain authority | Actor is a label; authority comes from config/CI identity and optional external attestation. Production cannot be inferred from a CLI flag alone. |
| `started_at`, `finished_at`, `status` | Deployment state | Timestamps/status elsewhere | States: requested, started, observing, healthy, failed, rolling_back, rolled_back; transitions fail closed. |
| `strategy`, `population`, `window` | Canary/progressive delivery | Absent | Policy-declared values, not agent prose. Adapter reports actual rollout fraction/window. |
| `health_check`, `threshold`, `observation` | Production acceptance | Verify commands/criterion evidence are pre-deploy only | Deterministic criteria identify metric/query/result; raw monitoring remains external. Missing or stale observation is not healthy. |
| `release_baseline` | Comparison target | Absent | Explicit prior healthy release/environment state for canary comparison. |
| `rollback_target`, `rollback_reason`, `rollback_status` | Recovery | Brain recovery is controller recovery, not release rollback | Required on failed production assurance. Rollback must itself have artifact and health evidence. |
| `evidence_ref`, `attestation_ref` | Auditable external fact | Evidence/ACP references | Store bounded summary plus local/content-addressed reference and trust source; do not ingest cloud credentials or full logs. |
| `palette_digest`, `config_digest`, `template_digest`, `binary_version` | Agent/harness compatibility | Palette/config digests; template version; binary version separately | One bootstrap response should bind all relevant identities before an agent drives a production workflow. |
| `state_schema`, `spec_revision` | Workspace compatibility | State schema and CAS revision | Required in deployment/upgrade preflight; unsupported newer schema fails before mutation. |

## Gaps and failure modes

### 1. The lifecycle stops at submission, not production assurance

`specd` can demonstrate that planned source tasks passed their verify commands and gates. It
cannot demonstrate that the exact resulting artifact entered an environment, met health
criteria, or was rolled back. Treating `complete` or `submit` as “deployed successfully” would
misrepresent the paper and operational reality.

### 2. CI checks state, but release identity is not a general contract

The PR action runs `specd check` against a checkout. It does not bind the verdict to a built
artifact digest, SBOM, provenance, target environment, or deployment attempt. The repository's
own tag workflow does this partially for `specd` archives, not for projects using `specd`.

### 3. No canary/health/rollback evidence model

An agent could claim “deployment healthy” in prose without a policy-defined observation window,
baseline, thresholds, freshness, or source. There is no deterministic rule that turns a monitor
result into a deployment verdict, and no rollback record tied to the failed and restored release.

### 4. Production authority is not yet an authenticated boundary

Brain's `--authority` is an explicit safety switch, but a local flag is not production identity.
MCP denies high-risk verbs, which is safer, but leaves host-specific CLI execution as the path.
Production deployment needs allowlisted environments/adapters and CI/workload identity evidence;
the LLM must never be able to self-approve or manufacture an observation.

### 5. Agent bootstrap does not bind all required knowledge

Palette and effective-config digests catch useful drift, while managed assets carry a template
version. The handshake does not currently bind binary release/commit, state schema, managed role
and steering content, active root/spec/revision, or context-manifest version in one packet. An
agent can therefore drive a newer binary using stale role guidance, or drive the wrong workspace,
without one fail-closed preflight.

### 6. Orchestrated mode has an inconsistent public path

`brain` requires `state.Mode == "orchestrated"`, while `internal/core/state.go` declares only
`default` and `agent` constants and there is no evident CLI transition that sets orchestrated
mode. Tests create that value directly. Because agents are instructed never to edit `state.json`,
the documented production path can be unreachable or invite forbidden manual mutation. This
must be resolved before Brain is represented as production-ready.

### 7. Cost/timeout brakes are not production-wired

Orchestration types include cost/deadline brakes, but `brain_run` only supplies retry limits and
`Sense` does not populate cost. A driver can continue dispatching without the economic controls
the types imply. This is both an observability and production-assurance gap.

### 8. Release installer tests do not test a real published artifact path

`install-scripts-test.sh` uses a synthetic shell script archive. This is valuable parser/safety
coverage, but it does not smoke-test a just-built GoReleaser archive, embedded version metadata,
SBOM/checksum naming, actual handshake, managed-asset refresh, or a workspace created by an older
binary.

### 9. Upgrade/recovery is incomplete

The installer verifies a checksum downloaded from the same release channel, then writes the
binary in place. There is no explicit signature/attestation verification, staged atomic swap,
retained previous binary, automatic rollback on smoke-test failure, schema compatibility
preflight, managed-asset diff preview as part of upgrade, or downgrade guard. Windows
self-replacement is documented as limited.

### 10. Production feedback has no safe route back to planning

Incidents and health regressions can be manually turned into requirements/tasks, but no domain
links a production observation to the responsible release/spec and a new refinement record. An
agent may paste noisy production logs into context, creating overload and prompt-injection risk.

### 11. One advertised regression invariant fails open on missing input

`scripts/regress-domains.sh` W0 reads `specs/progress.md`. In the current tree that file is
absent; `awk` reports the missing file, the status loop receives no rows, and W0 still prints
success because `w0_bad` remains zero. The script exits successfully even though the advertised
wave-ordering invariant was not evaluated. Production assurance must distinguish “input absent,”
“check not applicable,” and “check passed.”

## Target best-practice workflow

1. The coding agent begins with one compact bootstrap: binary version/commit, workspace root,
   active spec/status/revision, state/context/template schema versions, palette/config/managed
   asset digests, allowed commands, and the exact next valid command. A mismatch stops before any
   mutation.
2. The existing loop remains intact: requirements → design → tasks → execute → verify → review →
   submit. Only passing git-pinned evidence and configured eval/review/security gates can create a
   release candidate.
3. `specd release candidate` freezes a candidate identity from spec revision, git HEAD, required
   evidence-set digest, artifact digest, SBOM/provenance references, and harness bootstrap digest.
   It does not build or upload artifacts.
4. An external CI/CD adapter deploys that immutable artifact using a deterministic idempotency
   key. It returns a versioned, bounded evidence envelope. `specd` validates and appends it under
   the spec lock; provider credentials never enter `.specd/`.
5. Environment policy determines strategy, required approver/identity, health criteria,
   observation window, freshness, and rollback target. Production policy cannot be relaxed by
   task text, agent prompt, or deployment response.
6. A canary stays `observing` until every required criterion has fresh evidence from an allowed
   adapter for the exact deployment/artifact/environment. Missing, malformed, stale, or mismatched
   evidence fails closed.
7. Passing criteria permit promotion. Failed criteria require either a human-recorded exception
   under a separately governed policy or rollback. An exception never rewrites task evidence or
   claims the deployment was healthy.
8. Rollback records the failed release, target release, reason, adapter identity, action result,
   and post-rollback health. “Command issued” is not “rollback succeeded.”
9. A concise production observation summary can open a new incident/refinement spec by reference.
   Raw logs remain outside model context unless a scoped diagnostic task explicitly requests
   bounded excerpts.
10. Installing/upgrading `specd` stages and verifies a release, runs `version`, handshake, and
    workspace compatibility checks, previews managed-asset changes, atomically swaps binaries,
    and retains a rollback candidate until post-install smoke tests pass.

## Recommended action plan

### P0 — Establish a trustworthy production boundary

| Action | Likely code/artifact surfaces | Deterministic acceptance check |
|---|---|---|
| Specify release/environment/deployment/health/rollback envelopes and explicit state transitions before adding cloud integrations. | New `docs/delivery-contract.md`, `docs/open-spec-format.md`, example `project.yml` delivery policy | Canonical fixtures validate without network; unknown schema versions, environment names, state jumps, mismatched HEAD/artifact, stale health, and missing rollback target fail closed. |
| Make the agent bootstrap sufficient and drift-safe. | `internal/core/handshake.go`, `internal/cmd/registry.go`, `internal/core/managed.go`, `internal/version/version.go`, `docs/mcp-guide.md`, managed `AGENTS.md` template | One JSON response binds binary version/commit, state/context/template versions, workspace/spec/revision, palette/config/managed digests, allowed tools, and next valid commands. Any pinned mismatch exits non-zero before state mutation. |
| Resolve orchestrated-mode reachability and validation. | `internal/core/state.go`, `internal/core/config_*`, `internal/cmd/lifecycle.go`, `internal/cmd/brain_run.go`, `docs/agent-integration.md` | A supported CLI/config path enters orchestrated mode through CAS and approval; invalid mode fails schema validation; no test or guide requires hand-editing `state.json`; default mode behavior is unchanged. |
| Add an installed-binary lifecycle E2E lane that follows the generated agent guide. | New `scripts/production-smoke.sh`, `.github/workflows/ci.yml`, `internal/integration` | From an empty temp git repo, the installed binary runs init/new/approvals/context/verify/complete/review/submit using only advertised commands; every mutation occurs through CLI and a deliberately invalid step fails closed with the documented next action. |
| Make regression harness prerequisites explicit and fail closed. | `scripts/regress-domains.sh`, regression fixtures, `TESTING.md` | Every advertised invariant first proves its input exists and was parsed; missing `progress.md` fails or is explicitly skipped by declared policy, never reported as a pass. |
| Harden the repository's own release install/upgrade path. | `scripts/install.sh`, `scripts/install-scripts-test.sh`, new `scripts/release-smoke.sh`, `.github/workflows/release.yml`, `.goreleaser.yml` | CI installs an actual just-built archive, verifies checksum/attestation, checks `version --json` commit, runs handshake/init smoke, performs staged update and rollback-on-failed-smoke, and leaves the previous binary/workspace usable. |

### P1 — Add deployment evidence without owning deployment platforms

| Action | Likely code/artifact surfaces | Deterministic acceptance check |
|---|---|---|
| Add release-candidate and deployment ledgers, separate from task evidence and lifecycle status. | New `internal/core/delivery.go`, `internal/core/delivery_ledger.go`, `internal/cmd/release.go`, `internal/cmd/deploy.go`; `.specd/specs/<slug>/releases.jsonl` and `deployments.jsonl` | Candidate identity is immutable and reproducible; append/replay survives concurrency/crash; no deployment record can satisfy a task evidence gate or change `complete` retroactively. |
| Add environment policy and pure delivery gates. | `internal/core/config_loader.go`, `internal/core/config_validate.go`, new `internal/core/gates/delivery.go`, `project.yml` template | Same policy/evidence always yields same verdict; production requires explicit adapter/authority, artifact identity, observation freshness, and rollback target; lower environments may opt out without weakening task gates. |
| Define stdin/file-based optional adapter protocol. | `docs/adapters/deployment.md`, new `internal/core/adapter_envelope.go`, command handlers | Adapter receives no implicit credentials from `specd`; duplicate idempotency key is a no-op or conflict, never a second deployment; malformed/untrusted envelopes are rejected; core has zero new dependencies/network calls. |
| Implement canary/health/promotion/rollback evidence and reports. | `internal/core/delivery.go`, `internal/cmd/deploy.go`, `internal/cmd/report.go`, `internal/core/prometheus.go` | A canary cannot promote before its full window; missing/stale/wrong-release observations fail; failed canary requires rollback/exception; rollback is complete only after target health passes. Repeated reports are byte-identical. |
| Add CI example that binds source evidence to artifact and environment. | `.github/actions/specd-pr`, new optional `.github/actions/specd-delivery`, `docs/github-action.md` | Swapping an artifact after candidate creation fails digest check; PR checks cannot impersonate production delivery; fork PRs receive no production credentials. |

### P2 — Close the observe/refine and fleet-assurance loop

| Action | Likely code/artifact surfaces | Deterministic acceptance check |
|---|---|---|
| Add incident/production-observation references that can seed a new spec without copying raw logs. | New `internal/core/incident.go`, `internal/cmd/incident.go`, scaffold templates | New spec records source release/deployment/criterion and bounded evidence refs; raw external payload is not loaded by default; original ledgers remain immutable. |
| Add portfolio release/environment views. | `internal/core/program.go`, status/report renderers | View deterministically identifies deployed/healthy/failed/rolled-back release per environment and cross-spec blockers; it performs no discovery network calls. |
| Add compatibility matrix and recovery drills across supported release lines/platforms. | `.github/workflows/release.yml`, scheduled workflow, `scripts/upgrade-matrix.sh`, versioned test fixtures | N-1 → N upgrade preserves state/evidence; unsupported future schema and unsafe downgrade fail before writes; crash at each swap/checkpoint boundary recovers old or new complete state, never a partial installation. |
| Support externally attested CI/runtime identity. | `internal/core/attestation.go`, adapter docs/config | Tampered envelope, wrong repository/environment/audience, expired assertion, or untrusted key fails; offline fixture verification uses only Go stdlib cryptography. |

Delivery verbs/flags must be declared once in `internal/core/commands.go`, derived into MCP/help,
and mirrored in both `docs/command-reference.md` and `docs/CHEATSHEET.md`. High-risk production
mutations should remain absent from the general MCP palette unless a distinct capability-scoped
server is designed and audited.

## Production validation scenarios

1. **Fresh production-like install:** install a real archive on each supported OS/architecture,
   verify checksum/attestation, confirm version/commit, initialize a repo, and run the documented
   lifecycle with networking disabled after installation.
2. **Agent guide conformance:** a driver using only generated `AGENTS.md`, handshake, help, status,
   next, and context reaches each phase correctly; stale guidance or palette/config/template
   digest stops before an invalid command.
3. **Wrong workspace/spec:** pinned root, slug, or revision differs from bootstrap. Driver refuses
   rather than creating or mutating another `.specd/` tree.
4. **Unauthorized production attempt:** task text asks the agent to deploy, but the environment,
   adapter identity, or approval is absent. No deployment ledger entry or external adapter call
   occurs.
5. **Artifact substitution:** candidate was approved for digest A; pipeline presents digest B at
   deployment. Ingestion fails and cannot be waived by agent prose.
6. **Canary success:** exact artifact reaches the declared fraction; all criteria are fresh for
   the full window; promotion records the original evidence references and baseline.
7. **Canary failure and rollback:** a required criterion fails. Promotion is refused, rollback
   targets the last healthy digest, and success is recorded only after post-rollback health passes.
8. **Monitoring outage/stale data:** no observation or an expired one arrives. State remains
   `observing`/failed according to policy, never healthy by timeout default.
9. **Duplicate/racing callbacks:** repeated adapter envelopes with one idempotency key create one
   logical transition. Conflicting payloads fail closed and preserve both audit facts safely.
10. **Agent/controller crash:** crash before/after candidate, adapter request, deployment record,
    or rollback record. Recovery converges without double deploy/rollback or orphaned authority.
11. **N-1 upgrade:** old workspace and managed assets load under the new binary; dry-run shows
    migrations/refresh; evidence stays byte-preserved; failed smoke restores old binary.
12. **Future-schema/downgrade attempt:** binary cannot understand state/delivery schema. It exits
    before mutation with exact supported versions and recovery guidance.
13. **Corrupt installed binary:** staged binary fails checksum/version/handshake. It is never
    promoted over the working binary.
14. **Air-gapped CI:** gates, candidate creation, evidence validation, reports, and rollback plan
    work locally. Only explicitly configured adapters need network access.
15. **Secret/prompt injection in production output:** adapter payload includes hostile prose or a
    credential. Schema treats it as data, bounds/redacts storage, and never places it in standing
    agent instructions.

## Context-safety considerations

- The agent should receive one small phase packet: current status/revision, allowed next verbs,
  relevant environment policy summary, immutable release/artifact identity, blockers, and exact
  evidence needed. Do not preload CI logs, deployment logs, traces, dashboards, or old incidents.
- Bootstrap must distinguish harness instructions from untrusted requirements, source files, test
  output, and adapter observations. External text cannot amend authority or policy.
- Production observations should enter context as typed fields and bounded excerpts with source,
  timestamp, release/environment identity, and digest. Raw logs stay behind explicit references.
- Managed role/steering refresh is an operator-reviewed action. Upgrade may preview changes, but
  must not silently rewrite user-owned guidance or teach an agent new authority.
- `context` for a delivery/incident task should include the approved criteria and relevant design
  decision, not every historical spec. Cross-spec information should be summarized by the
  deterministic program view.
- Error messages must be actionable but non-expansive: exact failed precondition, current
  identity/state, and the next safe operator command. Avoid dumping entire envelopes into model
  context.
- The production driver should never need native memorization of undocumented state transitions.
  Help/handshake/status must derive their command and next-step knowledge from the same registry
  and state machine used by enforcement.

## Non-goals and risks

- **No cloud deployment engine in core.** Kubernetes, GitHub, cloud, and model-runtime support
  belong in optional adapters with explicit credentials and audit boundaries.
- **No change to task completion authority.** Deployment, health, telemetry, or rollback evidence
  can never substitute for passing verify evidence at a git HEAD.
- **No autonomous production approval by an LLM.** Agents may prepare plans and invoke explicitly
  authorized adapters; policy/human/CI identity governs promotion and exceptions.
- **No assumption that `complete` means deployed.** Source completion and environment health are
  different states and reports must label them separately.
- **No raw production telemetry in default context or state.** Store bounded facts and references.
- **No implicit network access.** Core remains deterministic, local, stdlib-only, and usable in
  air-gapped environments.
- **Risk: lifecycle bloat.** Adding deploy states to the existing six status values could break
  the ratchet and confuse agents. Prefer a parallel delivery ledger/domain.
- **Risk: adapter trust laundering.** A JSON envelope is not proof merely because it parses.
  Reports must expose adapter/source/attestation trust and production policy must allowlist it.
- **Risk: automatic rollback harm.** Rollback can be destructive or unsafe with irreversible data
  migrations. Require an explicit rollback target and capability classification; support a
  human-required strategy where reversal is not demonstrably safe.
- **Risk: checksum-only supply-chain confidence.** A checksum fetched beside an artifact detects
  corruption, not a compromised publisher. Add attestations/signatures without making online
  verification mandatory for local core operations.
- **Risk: platform overclaim.** CI compile coverage is not install/update/recovery coverage.
  Publish and document only combinations exercised by real artifact smoke tests.
