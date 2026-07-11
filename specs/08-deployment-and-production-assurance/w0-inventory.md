# W0 T01 — Current-behavior inventory & P0 gap map (Domain 08)

Read-only scout inventory of the CURRENT handshake / mode / installer / regress
behavior against the P0 action plan in
`docs/google-sdlc-alignment/08-deployment-and-production-assurance.md` (§"P0 —
Establish a trustworthy production boundary") and `requirements.md` R1–R5.
Evidence is `file:line` in the working tree at HEAD `41f7137`. No product code edited.

## P0 scope

The six P0 actions map to requirements R1–R5 (P1/P2 = R6–R11, out of W0 scope):

| P0 action | Requirement | W0 tasks |
|---|---|---|
| Specify release/env/deploy/health/rollback envelopes + fail-closed transitions | R1 | T02, T03, T16–T19 |
| Make agent bootstrap sufficient & drift-safe | R2 | T06–T10 |
| Resolve orchestrated-mode reachability & validation | R3 | T11–T15 |
| Installed-binary lifecycle E2E lane | R4.1 | T20, T21, T23 |
| Make regression prerequisites explicit & fail closed | R4.2 | T04, T22, T24 |
| Harden repo install/upgrade path | R5 | T25–T30 |

## Current behavior → P0 gap table

| Behavior / Requirement | Current code surface (file:line) | P0 gap |
|---|---|---|
| **Delivery envelopes / state machine (R1.1, R1.2)** | ABSENT. No `internal/core/delivery.go`, no release/deployment/health/rollback types anywhere. Only lifecycle status values exist in `internal/core/state.go` (`Phase` records, six-phase). | No versioned release/environment/deployment/health/rollback schema; no closed status set `{requested,started,observing,healthy,failed,rolling_back,rolled_back}`; no fail-closed transition table. Nothing rejects unknown schema, bad env name, state jump, HEAD/artifact mismatch, stale health, or missing rollback target. |
| **Delivery is additive, no gate crossover (R1.3)** | Evidence gate `internal/core/gates/*` + `internal/core/evidence.go` operate only on verify records; nothing delivery-aware exists to cross over yet. | Additivity is vacuously true today only because delivery does not exist; must be proven once ledgers land so no delivery record can satisfy a task gate or mutate `complete`. |
| **Bootstrap binds ALL identities (R2.1)** | `internal/core/handshake.go`: `Handshake` struct binds `Version` (a hardcoded handshake-schema string "1", NOT the binary version), `Tools` (`json:"tools"`), `PaletteDigest`, `ConfigDigest` (handshake.go:9-33, digest helpers ~37-50). | Handshake does NOT bind: binary version/commit (`internal/version`), state schema (`StateSchemaVersion`, state.go:14), context/template schema versions, workspace root, active spec/status/revision, or managed role/steering content digest. One packet cannot preflight a production driver. |
| **Pinned mismatch exits non-zero pre-mutation (R2.2)** | No mismatch/verify path. Handshake only emits digests for a client to compare; no `specd` verb consumes a pinned handshake and refuses to proceed. | No fail-closed preflight: an agent can drive a newer binary with stale roles or the wrong workspace with no exit-non-zero-before-mutation guard. |
| **Harness vs untrusted separation (R2.3)** | Handshake struct is a flat digest packet (handshake.go:9-16); no typed distinction between harness instructions and untrusted requirements/source/test-output/adapter-observation. | Bootstrap does not type its fields, so external text could be treated as authority-bearing. |
| **Orchestrated mode is a validated, reachable mode (R3.1)** | `internal/core/state.go:60-64` declares only `ModeDefault="default"` and `ModeAgent="agent"`. `orchestrated` is NOT a declared constant. Schema validation `Validate` (state.go:161-163) checks only `SchemaVersion`, not `Mode` enum. Brain requires `Mode=="orchestrated"` (per alignment doc:122-128) but no CLI transition sets it. | `orchestrated` is unreachable via any supported CLI/config path and is not schema-validated; tests set it by hand-editing `state.json`, which agents are forbidden to do. Mode enum is not enforced at all. |
| **Cost/deadline brake wired end-to-end (R3.2)** | Orchestration types carry cost/deadline brakes but `brain_run`/`Sense` do not populate cost (alignment doc:131-134; `internal/orchestration/`). | A driver keeps dispatching without the economic brake the types imply; brake is dormant. |
| **Fail-closed on missing trusted telemetry/authority (R3.3)** | `internal/orchestration/decide.go` has no fail-closed branch for missing trusted telemetry/authority under production policy. | No labeled fail-closed halt when production requires attested telemetry/authority and it is absent. |
| **Installed-lifecycle E2E lane (R4.1)** | ABSENT. No `scripts/production-smoke.sh`; `internal/integration` has no installed-binary lifecycle lane; `.github/workflows/ci.yml` compiles/tests but never runs the documented agent guide end-to-end from an empty repo. | No lane proves the installed binary runs init→...→submit via only advertised commands, with a deliberately-invalid step failing closed with the documented next action. |
| **Regression proves input exists, never fails open (R4.2)** | `scripts/regress-domains.sh:36-50` — W0 block feeds `awk ... "$RUN/specs/progress.md"` into a loop; if the file is absent, `awk` errors to stderr, the loop gets zero rows, `w0_bad` stays 0, and the block prints `pass W0`. `progress.md` currently EXISTS (`specs/progress.md`, 126 lines) so the check evaluates today, but the fail-open path is unchanged. | No invariant first proves its input exists and was parsed. "input absent", "not applicable", and "passed" are indistinguishable; a missing `progress.md` is reported as a pass. |
| **Install stages/verifies before atomic swap; retains prev binary; rollback-on-smoke (R5.2)** | `scripts/install.sh:213` — `run_privileged install -m 0755 "$tmp/specd" "$BIN"` writes directly over the target. Overwrite is refused only without `--update`/`--force` (install.sh:188-190). Checksum is verified (install.sh:127-134, 209) against `checksums.txt` from the SAME release channel. | No staged temp-path + atomic `rename` swap; no retained previous binary; no rollback-on-failed-smoke; a corrupt-but-checksum-matching or post-swap-broken binary can replace a working one. |
| **Attestation / version-commit / handshake smoke on install (R5.1)** | Only same-channel SHA-256 checksum (install.sh:114-134). No signature/attestation, no `version --json` commit confirmation, no handshake/init smoke, no `scripts/release-smoke.sh`, no real just-built-archive install in `.github/workflows/release.yml`. | Checksum-only supply-chain confidence (detects corruption, not a compromised publisher); no post-install smoke gate. |
| **Schema compatibility preflight (R5.3)** | `internal/core/state.go:118-128` migrates schema 0→1→2 and returns `unsupported state schema %d` for anything higher; `Validate` (state.go:161-163) re-checks. This is load-time, not an install-time preflight, and there is no downgrade guard. | No install/upgrade-time schema preflight and no unsafe-downgrade guard before any write; delivery schema does not exist to preflight. |
| **Managed-asset diff preview on upgrade (R5.2)** | `install.sh` installs only the binary; managed `AGENTS.md`/roles/steering refresh happens via `init`/`managed.go`, not previewed at upgrade. | Upgrade cannot preview managed-asset changes before applying them. |

## 15 planned fixtures checklist (one failing fixture per production-validation scenario)

Each scenario in `08-deployment-and-production-assurance.md:236-270` must land a
deterministic, offline, RED fixture in W0 (T03 `internal/core/delivery_fixtures_test.go`
+ `testdata/delivery/*.json`), turning GREEN in the wave that implements it. All must
run with networking disabled.

- [ ] **S1 Fresh production-like install** — real archive install per OS/arch, checksum/attestation, version/commit, init, documented lifecycle offline. (R5.1; W5)
- [ ] **S2 Agent-guide conformance** — driver using only AGENTS.md/handshake/help/status/next/context reaches each phase; stale palette/config/template/managed digest stops before an invalid command. (R2.1, R2.2; W1)
- [ ] **S3 Wrong workspace/spec** — pinned root/slug/revision differs from bootstrap; driver refuses, mutates no other `.specd/` tree. (R2.2; W1)
- [ ] **S4 Unauthorized production attempt** — task text asks to deploy but env/adapter/approval absent; no ledger entry, no adapter call. (R7.2, R3.3; W2/W7)
- [ ] **S5 Artifact substitution** — candidate approved for digest A, pipeline presents digest B; ingestion fails, not waivable by prose. (R1.2, R7.3; W3/W7)
- [ ] **S6 Canary success** — exact artifact hits declared fraction, all criteria fresh for full window; promotion records baseline + evidence refs. (R9.1, R9.2; W9)
- [ ] **S7 Canary failure & rollback** — required criterion fails; promotion refused; rollback targets last healthy digest; success only after post-rollback health. (R9.1, R9.3; W9)
- [ ] **S8 Monitoring outage / stale data** — no/expired observation; state stays observing/failed per policy, never healthy-by-timeout. (R9.1; W9)
- [ ] **S9 Duplicate/racing callbacks** — repeated envelopes, one idempotency key → one transition; conflicting payloads fail closed, preserve both audit facts. (R8.2; W8)
- [ ] **S10 Agent/controller crash** — crash before/after candidate, adapter request, deployment record, or rollback record; recovery converges, no double deploy/rollback or orphaned authority. (R6.2, R11.3; W6/W11)
- [ ] **S11 N-1 upgrade** — old workspace + managed assets load under new binary; dry-run shows migrations/refresh; evidence byte-preserved; failed smoke restores old binary. (R5.2, R5.3, R11.3; W5/W11)
- [ ] **S12 Future-schema / downgrade attempt** — binary cannot understand state/delivery schema; exits before mutation with supported versions + recovery guidance. (R5.3; W5)
- [ ] **S13 Corrupt installed binary** — staged binary fails checksum/version/handshake; never promoted over working binary. (R5.1, R5.2; W5)
- [ ] **S14 Air-gapped CI** — gates, candidate creation, evidence validation, reports, rollback plan work locally; only configured adapters need network. (R1.1, R7.3; W3/W7)
- [ ] **S15 Secret / prompt injection in production output** — adapter payload with hostile prose/credential is treated as data, bounded/redacted, never placed in standing agent instructions. (R2.3, R8.3; W1/W8)
