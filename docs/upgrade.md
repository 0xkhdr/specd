# Upgrading configuration

Legacy `project.yml` and `project.yaml` remain readable, with a stable
deprecation diagnostic, for at least two minor releases after canonical
configuration was introduced.

Before upgrading, run `specd config migrate --dry-run`. Review the reported
source, target, permissions, backup path, conflicts, and effective-value
comparison. Then run `specd config migrate`; Specd writes and validates
`.specd/config.yaml` atomically before preserving the legacy source as
`<source>.specd-v1.bak`.

Migration is replay-safe. If canonical installation completed before an
interruption, rerunning finishes the backup step. If both legacy spellings
exist with equal values, choose one with `--source`; differing values must be
reconciled first. Existing backups are never overwritten or deleted.
