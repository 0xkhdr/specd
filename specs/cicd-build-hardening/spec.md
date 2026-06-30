# Spec — CI/CD & Build Hardening (S6)

## Introduction

Live evidence (see `../discrepancies.md` D13) shows `.goreleaser.yml` already
generates a CycloneDX SBOM via syft — the analysis plan's generic "SBOM
generation" recommendation is already satisfied. The two real gaps are:
`-trimpath` is absent from the release `ldflags` (low-risk, high-value, no
decision required), and artifact signing is absent and **explicitly deferred**
per an existing comment in `.goreleaser.yml` ("Artifact signing remains a
documented deferral until signing-key management is in place"). Implementing
signing unilaterally would mean generating/managing a signing key as a side
effect of a code-quality review — out of this review's authority. This spec
adds `-trimpath`, documents the local `make ci` sequential-execution tradeoff
(F8, confirmed true but the GitHub Actions workflow already parallelizes at
the job level), and raises signing as an explicit decision gate for the user
rather than silently implementing or silently ignoring it.

## Requirement 1 — Reproducible builds

**User story:** As a security-conscious user verifying a released specd
binary matches its source, I want the build to strip local path information,
so two independent builds from the same commit produce comparable output.

**Acceptance criteria:**
1. THE SYSTEM SHALL add `-trimpath` to the `go build` flags in
   `.goreleaser.yml`, alongside the existing `-s -w` `ldflags`.
2. THE SYSTEM SHALL confirm the resulting release binary still embeds the
   correct `main.version` (the existing `-X main.version={{.Version}}`
   `ldflags` entry) — `-trimpath` affects file paths in the binary, not
   `-X`-injected variables, but this must be verified, not assumed.
3. THE SYSTEM SHALL NOT change any other `.goreleaser.yml` build
   configuration (target OS/arch matrix, archive format) as part of this
   requirement.

## Requirement 2 — Artifact signing decision gate

**User story:** As the project maintainer, I want to make an informed,
explicit decision about artifact signing (cosign/gpg) rather than have a
coding-review process either silently implement it (creating a key-management
obligation I haven't agreed to) or silently drop it (leaving the existing
"deferred" comment stale).

**Acceptance criteria:**
1. THE SYSTEM SHALL NOT implement artifact signing as part of this spec.
2. THE SYSTEM SHALL document the specific decision required (which signing
   mechanism — cosign keyless via OIDC vs. a managed key — and what CI
   secret/identity provisioning each requires) so the maintainer can make
   the call in a follow-up, separately-scoped change.
3. THE SYSTEM SHALL confirm the existing "deferred" comment in
   `.goreleaser.yml` remains accurate and is not contradicted by any other
   change in this spec.

## Requirement 3 — Local CI execution time (F8)

**User story:** As a contributor running `make ci` before pushing, I want to
understand why it's sequential and whether safe parallelism is available, so
I'm not left assuming it's an oversight.

**Acceptance criteria:**
1. THE SYSTEM SHALL document, in the `Makefile` near the `ci` target, which
   of its prerequisite targets (`lint test test-order cover-check perf-gate
   stress stress-acp stress-orchestration stress-program
   stress-brain-recovery stress-checkpoint-fault`) have data dependencies on
   each other (e.g., do stress targets depend on a built binary that `test`
   also needs?) versus which are independent and could run under `make -j`.
2. WHERE targets are confirmed independent, THE SYSTEM SHALL note that
   `make -j<N> ci` is a safe local speedup, OR, IF dependencies make full
   parallelism unsafe, THE SYSTEM SHALL document why (e.g., shared
   `coverage.out` file writes, shared test ports) so a future contributor
   doesn't attempt an unsafe parallel rewrite.
3. THE SYSTEM SHALL NOT change `.github/workflows/ci.yml`'s job structure —
   it already parallelizes at the job level (`lint`, `analyze`, `test`,
   `coverage-floor`, `stress*`, `build` run as separate GitHub Actions jobs),
   confirmed by live evidence; this requirement is about local `make ci`
   documentation only.

## Design

### Overview
One concrete build-flag change (`-trimpath`), one documentation-only
decision-gate writeup (signing), and one documentation-only dependency
analysis (`make ci` parallelism). No new CI jobs, no new secrets, no new
external services.

### Architecture
No architecture change.

### Components and interfaces
- `.goreleaser.yml` — `ldflags` gains `-trimpath`.
- `docs/contributor-guide.md` or a new `docs/build-reproducibility.md` —
  signing decision writeup (Requirement 2.2).
- `Makefile` — comment block above the `ci` target (Requirement 3.1-3.2).

### Data models
No changes.

### Error handling
No changes.

### Verification strategy
- `goreleaser build --snapshot --clean` (or equivalent dry-run) after the
  `-trimpath` change, confirming the binary still reports the correct
  version via `specd --version` and contains no local filesystem paths
  (`strings <binary> | grep $(pwd)` should return nothing).
- Manual review of the signing decision writeup against
  `.goreleaser.yml`'s existing comment for consistency.

### Risks and open questions
- Open question (for the user, not the builder to decide): cosign keyless
  signing via GitHub OIDC vs. a managed GPG key — this spec's Requirement 2
  surfaces the question; it does not answer it.
- Risk: `-trimpath` combined with `-X main.version=...` needs verification
  that version embedding still works — Requirement 1.2's explicit check
  exists because this is a real (if small) risk, not a hypothetical.
