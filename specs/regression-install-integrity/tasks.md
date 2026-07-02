# S8 Tasks — Install Integrity Regression

Requirement coverage: R8. Dependencies: none (Wave-1 root).

## Wave 1 — Baseline

- [ ] Run existing installer test and shellcheck; record current behavior.
  Files: `scripts/install.sh`, `scripts/install_test.sh`.
- [ ] Inventory verification branches in `verify_checksum` (sha256sum, shasum,
  neither, `--no-verify`).
- **Validation:** `shellcheck scripts/install.sh && bash scripts/install_test.sh`

## Wave 2 — Core regression tests (depends on Wave 1)

- [ ] Matching checksum → success. File: `scripts/install_test.sh` (extend).
- [ ] Tampered archive → non-zero exit (fail closed). File:
  `scripts/install_test.sh`.
- [ ] `--no-verify` → skip with warning. File: `scripts/install_test.sh`.
- [ ] Neither `sha256sum` nor `shasum` present → abort message (simulate via
  PATH). File: `scripts/install_test.sh`.
- **Validation:** `bash scripts/install_test.sh`

## Wave 3 — Lint & POSIX (depends on Wave 2)

- [ ] Ensure `install.sh` stays shellcheck-clean and POSIX-portable.
- **Validation:** `make shellcheck`

## Rollout & cleanup

- [ ] Remove temporary fixture archives from `scripts/testdata/` if generated.
- **Rollback:** revert test additions; installer unchanged.
- **Completion evidence:** green `install_test.sh` incl. tamper case + clean
  shellcheck.
