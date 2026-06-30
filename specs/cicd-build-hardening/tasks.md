# Tasks — CI/CD & Build Hardening (S6)

## Wave 1

- [ ] T1 — Map make ci's prerequisite data dependencies
  - why: Requirement 3.1-3.2 requires evidence-backed documentation of which `make ci` prerequisites are safely parallelizable, not a guess
  - role: investigator
  - files: Makefile, scripts/coverage-check.sh, scripts/stress*.sh
  - contract: read the `lint`, `test`, `test-order`, `cover-check`, `perf-gate`, and all `stress*` targets in full; identify shared file writes (e.g. `coverage.out`), shared ports, or build-artifact dependencies (e.g. do stress targets require `make build` to have run first?) that would make `make -j` unsafe for any pair of targets. Do NOT modify the Makefile.
  - acceptance: a written dependency map (safe-to-parallelize pairs vs. unsafe pairs with the specific shared resource named)
  - verify: N/A
  - depends: —
  - requirements: 3

## Wave 2

- [ ] T2 — Add -trimpath to goreleaser build flags
  - why: close the confirmed reproducible-builds gap, per Requirement 1
  - role: builder
  - files: .goreleaser.yml
  - contract: add `-trimpath` to the `flags:` (or wherever goreleaser's build-flags section is, distinct from `ldflags`) for the release build, alongside the existing `-s -w` `ldflags`. Do not modify the `-X main.version={{.Version}}` entry or any other goreleaser section.
  - acceptance: `.goreleaser.yml` includes `-trimpath`; a snapshot build's binary contains no local filesystem paths
  - verify: cd /var/www/html/rai/up/specd && command -v goreleaser >/dev/null && goreleaser build --snapshot --clean --single-target || echo "goreleaser not installed locally — verify in CI release dry-run"
  - depends: —
  - requirements: 1

- [ ] T3 — Verify version embedding survives -trimpath
  - why: Requirement 1.2 — explicit check, not an assumption, that -trimpath doesn't interfere with -X-injected version strings
  - role: builder
  - files: N/A (verification task against T2's build output)
  - contract: build the binary produced by T2 (snapshot or local `go build -trimpath -ldflags "-s -w -X main.version=test"`) and run `specd --version`, confirming it reports the injected version correctly.
  - acceptance: `specd --version` (or equivalent) output matches the injected version string exactly
  - verify: cd /var/www/html/rai/up/specd && go build -trimpath -ldflags "-s -w -X main.version=test-trimpath" -o /tmp/specd-trimpath . && /tmp/specd-trimpath --version
  - depends: T2
  - requirements: 1

- [ ] T4 — Document the make ci parallelism finding
  - why: Requirement 3.1-3.2 — turn T1's investigation into a comment future contributors can rely on
  - role: builder
  - files: Makefile
  - contract: add a comment block immediately above the `ci` target summarizing T1's findings: which prerequisites are safe to run with `make -j` locally, and which share a resource (named explicitly) that makes full parallelism unsafe. Do NOT actually change the `ci` target's prerequisite list or add `.NOTPARALLEL`/job-server hints unless T1 found it's fully safe to do so — if there's any doubt, document the constraint rather than changing behavior.
  - acceptance: the `Makefile` comment accurately reflects T1's dependency map; no behavior change unless T1 confirmed full safety
  - verify: cd /var/www/html/rai/up/specd && make ci
  - depends: T1
  - requirements: 3

- [ ] T5 — Write the artifact-signing decision-gate document
  - why: Requirement 2 — surface the signing decision for the maintainer instead of silently implementing or silently dropping it
  - role: builder
  - files: docs/build-reproducibility.md (new) or an addition to docs/contributor-guide.md
  - contract: document the two realistic signing approaches (cosign keyless via GitHub OIDC — no key management, but requires CI workflow permissions changes; vs. a managed GPG/cosign key — requires secret provisioning and rotation policy), what CI changes each would require, and explicitly state this spec does not implement either. Cross-reference the existing `.goreleaser.yml` comment so the two stay consistent.
  - acceptance: the document exists, accurately describes both options without recommending implementation, and matches the existing `.goreleaser.yml` deferral comment's framing
  - verify: cd /var/www/html/rai/up/specd && bash scripts/docs-lint.sh
  - depends: —
  - requirements: 2

## Wave 3

- [ ] T6 — Release dry-run verification
  - why: confirm T2's -trimpath addition doesn't break the release pipeline before this spec is considered done
  - role: verifier
  - files: N/A
  - contract: run a goreleaser dry-run (`--snapshot`) or, if unavailable locally, confirm the `.github/workflows/release.yml` configuration would pick up the change correctly by reading the workflow file against the updated `.goreleaser.yml`
  - acceptance: snapshot release build succeeds (or workflow review confirms compatibility) with no errors attributable to T2
  - verify: cd /var/www/html/rai/up/specd && make ci
  - depends: T3, T4, T5
  - requirements: 1, 2, 3
