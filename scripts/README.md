# scripts/ — repository tooling inventory

Every script here is wired into a workflow, a release gate, or the documented
install path. Nothing in this directory is stage-only scaffolding.

| Script | Purpose | Wired into |
|--------|---------|------------|
| `install.sh` | End-user install (release binary or `go build` fallback) | README, docs/user-guide.md, release assets |
| `uninstall.sh` | End-user uninstall | README, docs/user-guide.md |
| `install-scripts-test.sh` | Black-box tests of install/uninstall | CI lint job, release gate |
| `test-lint.sh` | Test-suite structural lint (banned suffixes, subtest names, dup helpers) | CI lint job, release gate |
| `docs-lint.sh` | Checks generated command-reference and documented invariants | CI lint job, release gate |
| `coverage-check.sh` | Enforces the total-coverage floor | CI coverage job |
| `perf-gate.sh` | A4: disabled-mode context budget does no work | CI |
| `regress-domains.sh` | Per-domain black-box invariants against a fresh binary in a throwaway tree | CI regression job, release gate |
| `adapter-conformance.sh` | Adapter envelope conformance probes | `regress-domains.sh` |
| `production-smoke.sh` | End-to-end production smoke of a fresh scaffold | CI |
| `release-smoke.sh` | Smoke-tests published release artifacts | release workflow |
| `upgrade-matrix.sh` | Upgrade-path matrix across released versions | upgrade-matrix workflow |
| `dep-evidence.sh` | Offline `dep-evidence/v1` producer for the opt-in security gate (`security.ScanDepEvidence`) | operator-run, out of band |
| `stress.sh <domain>` | Parameterized state, ACP, orchestration, program, recovery, and checkpoint-fault stress | CI stress matrix |

Coverage floor policy: the `FLOOR` value lives in `coverage-check.sh` and only
ratchets up. `govulncheck` is version-pinned in `ci.yml`.
