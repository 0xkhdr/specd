# Security Policy

## Reporting a vulnerability

Please report security issues **privately**, not via public issues or PRs.

- Preferred: open a [GitHub private security advisory](https://github.com/0xkhdr/specd/security/advisories/new).
- Alternative: email **0xkhdr@gmail.com** with details and, ideally, a minimal
  reproduction.

We aim to acknowledge a report within a few days and will coordinate a fix and
disclosure timeline with you.

## Supported versions

specd is currently at v0.1.0; only the latest tagged release receives security fixes.

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
- **Verify isolation (opt-in, fail-closed).** `verify.sandbox` (config) or
  `specd verify --sandbox <none|bwrap|container>` selects an isolation backend
  for the `verify:` command. `none` (the default) is byte-identical to running
  `sh -c` directly. `bwrap` runs under bubblewrap with a read-only root, a
  writable bind of only the workspace, private `/proc`, `/dev`, `/tmp`, and
  **no network** (`--unshare-all`). `container` runs in a throwaway docker/podman
  container with `--network none`, only the workspace bind-mounted, and just the
  scrubbed env forwarded. **Isolation is fail-closed:** if the requested backend's
  tool is absent from `PATH` (or `container` has no pinned `SPECD_SANDBOX_IMAGE`),
  `specd verify` refuses to run rather than silently falling back to an
  unisolated shell — a verify that asked for isolation never quietly runs without
  it. There is no separate pre-check that advises on a missing `bwrap`/container
  dependency ahead of time (the `specd doctor` command that used to report it has
  been removed); `SelectRunner`'s fail-closed refusal at `specd verify` time is
  now the only signal that the requested isolation backend is unavailable.
- **Custom gates (unisolated).** Custom gates configured under `config.yml` (or legacy `config.json`) execute external programs on the host. Although their environment is scrubbed and execution is bounded by a timeout (`SPECD_CUSTOM_GATE_TIMEOUT_MS`), **custom gates do not run within bubblewrap or container sandbox isolation**. Only run `specd check` on projects where the custom gate commands are trusted.
- **Config precedence.** Human-authored config is untrusted policy input. Effective config is embedded defaults → global config → project config → supported `SPECD_*` env overrides, then validation. Env diagnostics expose variable names and target fields, never an environment dump; secret-bearing orchestration keys remain rejected.
- **Path safety.** Spec slugs are validated (`internal/core/slug.go`) to prevent
  path traversal under `.specd/`.
- **Install integrity.** `install.sh` fetches a release archive and fails
  closed if the `SHA256SUMS` digest does not match (`--no-verify` skips with a
  loud warning).
- **MCP `--http` is loopback-by-design.** The MCP HTTP/SSE transport
  (`specd mcp --http`) defaults to a loopback bind (`127.0.0.1`); a bare or
  host-less address is rewritten to loopback so spec contents never leave the
  host implicitly. `/rpc` and `/sse` expose full workflow control (dispatch,
  phase transitions) and ship with **no TLS**. Binding a non-loopback interface
  (e.g. `--http 0.0.0.0:8765`) is **at operator risk**: without a token it
  exposes unauthenticated workflow control to anyone who can reach the port. The
  server prints a loud stderr warning on such a bind. To require auth, set
  `SPECD_MCP_TOKEN`; `/rpc` and `/sse` then demand a matching
  `Authorization: Bearer <token>` header (constant-time compared, `401` on a
  miss). Terminate TLS at a reverse proxy. See
  [`docs/mcp-guide.md`](docs/mcp-guide.md#exposure--auth).
- **MCP argument-shape validation.** Every raw-passthrough `tools/call`
  (`internal/mcp/argschema.go`) is checked against that tool's declared
  argument schema — derived from the same `core.Commands` data used to
  advertise `tools/list`, so the two can't drift apart — before any argv is
  built or dispatched. An undeclared argument key, or a value whose shape
  doesn't match the declared flag (e.g. an array/object where a scalar is
  expected), is rejected with a JSON-RPC error ahead of command dispatch and
  any per-command validation (e.g. `ValidateSlug`).
- **No runtime dependencies, no network at rest.** The shipped binary is
  stdlib-only, makes no LLM calls, and reads no on-disk templates at runtime.

If you find a way to bypass any of these controls, that is a security issue —
please report it as above.
