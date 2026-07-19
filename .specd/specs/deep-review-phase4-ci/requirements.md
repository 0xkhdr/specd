# Requirements — deep-review-phase4-ci

> Source: DEEP-REVIEW.md §2 finding #10, §3.1, §4 Phase 4.

## R1 — Tiered CI

- owner: 0xkhdr
- priority: should
- risk: medium

- R1.1: When a pull request is pushed, the system shall run only the fast tier — gofmt, go vet, staticcheck, test-lint, `go test ./... -race -count=1`, docs check — as parallel jobs targeting under five minutes.
- R1.2: When a merge to `main` lands, the system shall run the heavy tier — `go test ./... -count=2`, `regress-domains.sh`, the stress/contention suite, perf-gate, and install-script tests — in a separate workflow.
- R1.3: When a release is cut, the system shall keep cross-compiles, the production smoke, and the upgrade matrix in the existing release workflow.
- edge: If a heavy-tier job fails on `main`, the system shall surface the failure on the merge commit (workflow status), not silently pass the PR tier.

## R2 — One parameterized stress script

- owner: 0xkhdr
- priority: should
- risk: low

- R2.1: When a stress domain is exercised, the system shall run `scripts/stress.sh <domain>` where `<domain>` covers the current six variants (default, acp, orchestration, program, brain-recovery, checkpoint-fault) with shared setup factored once.
- R2.2: When the consolidation lands, the system shall reference only `stress.sh` from CI, and the five `stress-*.sh` scripts shall be deleted.
- edge: If an unknown domain argument is given, `stress.sh` shall exit non-zero with a usage message listing valid domains.

## Non-goals

- No change to what is validated — only when and in which job it runs.
- No new CI dependencies beyond what the workflows already install.
