# Security Policy

## Reporting a vulnerability

Please report security issues **privately**, not via public issues or PRs.

- Preferred: open a [GitHub private security advisory](https://github.com/0xkhdr/specd/security/advisories/new).
- Alternative: email **0xkhdr@gmail.com** with details and, ideally, a minimal
  reproduction.

We aim to acknowledge a report within a few days and will coordinate a fix and
disclosure timeline with you.

## Supported versions

specd is pre-1.0; only the latest tagged release receives security fixes.

## Threat model

specd runs **agent-authored input** (`tasks.md`) — it is treated as untrusted
until validated. The full, authoritative threat model lives in
[`docs/validation-gates.md`](docs/validation-gates.md#security-model); the
highlights:

- **Shell execution.** `specd verify` runs each task's `verify:` line via
  `sh -c` (override with `SPECD_VERIFY_SHELL`) as the invoking user — real code
  execution. **Only run `specd verify` on a `tasks.md` you trust.** Mitigations:
  the child environment is scrubbed to an allowlist (`PATH`, `HOME`, `LANG`,
  `LC_ALL`, `TMPDIR`, `SPECD_*`), dropping inherited secrets; commands containing
  a NUL byte are rejected; the command and cwd are printed before execution.
- **Path safety.** Spec slugs are validated (`internal/core/slug.go`) to prevent
  path traversal under `.specd/`.
- **Self-update integrity.** `install.sh` and `specd update` fetch a release
  archive and fail closed if the `SHA256SUMS` digest does not match
  (`--no-verify` skips with a loud warning).
- **No runtime dependencies, no network at rest.** The shipped binary is
  stdlib-only, makes no LLM calls, and reads no on-disk templates at runtime.

If you find a way to bypass any of these controls, that is a security issue —
please report it as above.
