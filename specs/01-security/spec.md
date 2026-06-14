# Stage 01 — Security Hardening

## Scope

Threat-facing surfaces: arbitrary shell execution (`verify.go`), self-updating
binary (`update.go`), `curl | bash` installer (`install.sh`), env-var inputs,
and file/temp permissions (`io.go`, `lock.go`). These are the highest blast
radius issues in the codebase and have no upstream code dependencies, so they
go first.

## Project axiom

"The agent reasons. The harness enforces." `tasks.md` is **agent-authored
input**, not trusted config. The harness must treat every `verify:` line and
every env var as hostile until validated.

## Current state & findings

### F1 — [HIGH, by-design but undocumented] Shell injection via `verify:`
`internal/cmd/verify.go:96`
```go
cmd := exec.CommandContext(ctx, "sh", "-c", command)
```
`command` is `docTask.Meta["verify"]` read verbatim from `tasks.md`. A
compromised/hostile `tasks.md` runs arbitrary code as the invoking user.

This is partly intentional — verify lines *are* shell pipelines. The problem is
it is **silent and unbounded**: no working-directory confinement beyond
`cmd.Dir = root`, no env scrubbing, no opt-in, no audit line. Threat model is
acceptable *only if* explicit and documented.

**Intent:** keep `sh -c` (pipelines are a real need) but make it a *conscious,
auditable* capability:
- Scrub the child environment to an allowlist (`PATH`, `HOME`, `SPECD_*`),
  dropping inherited secrets from the parent shell.
- Print the exact command + cwd before running (already partially done at
  `verify.go:143`, but only *after*).
- Document the threat model in `AGENTS.md` and `docs/validation-gates.md`.
- Add `SPECD_VERIFY_SHELL` override (default `sh`) so locked-down environments
  can point at a restricted shell, and reject a command containing a NUL byte.

### F2 — [CRITICAL] Self-update has no integrity verification
`internal/cmd/update.go:45-95`
`downloadBinary` fetches a tarball over HTTPS and `os.Rename`s it over the
running binary. **No checksum, no signature.** TLS protects transport, but a
compromised release asset, a CDN cache poisoning, or a tag-name confusion
silently installs an attacker binary that runs on next invocation.

Secondary bugs in the same function:
- `update.go:48-50` dead code: `if goarch == "amd64" { goarch = "amd64" }`.
- `update.go:126-131` the "retry with .exe" path calls `downloadBinary(tag,
  self)` with **identical arguments** — it cannot succeed where the first
  failed; the comment claims it tries a different extension but it does not.
- No atomic rollback note: rename over a running binary is fine on Unix, breaks
  on Windows (file locked) — unhandled.

**Intent:** download and verify a `SHA256SUMS` (and optionally a `.sig`) before
rename. Fail closed: if checksum file is missing or mismatched, abort and keep
the current binary. Delete the dead `goarch` block and the bogus retry.

### F3 — [HIGH] Installer trusts unverified artifact
`scripts/install.sh:133-159`
Downloads `specd_${GOOS}_${GOARCH}.tar.gz` and `chmod +x` with no checksum.
Same gap as F2 but at install time. `curl | bash` is the documented entry
(`install.sh:3`), so this is the first-contact trust boundary.

**Intent:** fetch `SHA256SUMS` from the same release, verify with
`sha256sum -c` (fallback `shasum -a 256`), abort on mismatch. Provide a
`--no-verify` escape hatch that loudly warns.

### F4 — [MEDIUM] Env vars parsed without validated bounds / messaging
- `verify.go:25-33` `SPECD_VERIFY_TIMEOUT_MS`: silently ignores non-numeric and
  ≤0; a typo (e.g. `1000ms`) silently falls back to 600s with no warning.
- `lock.go:56-66` `numEnv` similarly silent on bad input.
No injection risk (all integer-parsed), but **silent fallback hides
misconfiguration**.

**Intent:** centralize env-int parsing in one `core.EnvInt(name, def, min, max)`
helper that clamps and emits a one-line `core.Warn` on malformed input. Reuse
for all three vars.

### F5 — [LOW] Temp-file & lock-file permissions
- `io.go:36` `os.CreateTemp` → mode `0600` (Go default) — good, but the final
  `os.Rename` inherits the temp's `0600`, **dropping the intended `0644`** for
  `state.json`/`tasks.md`. Files become owner-only readable, surprising for
  shared CI checkouts.
- `lock.go:82` lock file created `0644` and contains PID + epoch ms — not
  sensitive, acceptable; document that it is non-secret.

**Intent:** `AtomicWrite` should `f.Chmod(0o644)` (honoring umask) before
rename so written artifacts get the documented `0644`. Keep `0600` only if a
caller opts in (none today).

### F6 — [INFO] Path traversal is already well-defended
`slug.go` `ValidateSlug` + `SlugRE = ^[a-z0-9][a-z0-9-]*$` is correct and
called via `RequireSpec`. **No change** — record as a passing control and add a
table test asserting `..`, `/`, `\`, leading `-`, empty all reject (Stage 07).

## Non-goals
- Replacing `sh -c` with arg-vector parsing (breaks legitimate pipelines).
- Code-signing infrastructure (cosign/notary) — out of scope; checksum first.

## Acceptance criteria
1. `specd update` aborts with a clear error if `SHA256SUMS` is missing or the
   downloaded binary's digest mismatches; the existing binary is untouched.
2. `install.sh` verifies checksum by default; `--no-verify` warns and proceeds.
3. `specd verify` runs with a scrubbed env (allowlist) and prints command+cwd
   before execution; documented in `AGENTS.md`.
4. Malformed `SPECD_*` int env vars emit one warning and use clamped defaults.
5. `state.json`/`tasks.md` land as `0644` (minus umask).
6. `go test -race ./...` green; new tests cover checksum-mismatch and env-clamp.
