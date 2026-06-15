# spec.md — Spec-pack Registry (shareable templates)

**Status:** proposed
**Source:** specd-report.html §8 idea **E1** (impact: high · effort: med · moat: high) · §9 north-star item **#4**
**Date:** 2026-06-16
**Scope:** `specd init --pack <name>` install path + pack format; `internal/cmd/init.go`.

---

## 1. Objective

A registry of reusable spec/steering/role packs (e.g. "Next.js feature", "Go
microservice", "Rust CLI") installable via `specd init --pack go-service`.
Templates are how frameworks win; a community of packs makes specd the obvious
default instead of a blank-stub chore, and the more packs, the more reason to
start in specd (network effects).

> **Hard invariant:** stdlib-only, deterministic, supply-chain-safe. Packs are
> static template bundles (steering docs, role defs, spec stubs, config
> defaults) — they contain **no executable code** specd runs. Remote pack fetch
> is **opt-in and integrity-verified** (pinned ref + SHA256, fail-closed) using
> the same discipline as `update.go`/`install.sh`. Local/embedded packs need no
> network.

## 2. Context

- `specd init` (`internal/cmd/init.go`) scaffolds `.specd/` — steering, roles,
  skill pack, AGENTS.md — likely via `internal/core/embed.go` embedded assets.
- `update.go`/`install.sh` already implement fail-closed SHA256 verification —
  reuse that pattern for remote packs.

## 3. Requirements (EARS)

- **R1 (H)** WHERE `specd init --pack <name>` is given, THE SYSTEM SHALL scaffold
  `.specd/` from the named pack's templates instead of (or layered over) the
  default bundle.
- **R2 (H)** THE SYSTEM SHALL ship a set of built-in packs embedded in the binary
  (no network required for those), discoverable via `specd init --list-packs`.
- **R3 (H)** WHERE a pack is fetched remotely, THE SYSTEM SHALL verify it against
  a pinned SHA256 and SHALL fail closed on mismatch — never install unverified
  content (mirroring `update.go`).
- **R4 (M)** A pack SHALL be a declarative bundle (templates + a manifest of
  config defaults/roles) and SHALL contain no code specd executes during init.
- **R5 (M)** IF a requested pack is unknown or its manifest is invalid, THE
  SYSTEM SHALL error clearly and SHALL NOT partially scaffold.
- **R6 (M)** WHERE `--pack` is omitted, `specd init` SHALL behave exactly as
  today (default bundle, backward compatible).
- **R7 (L)** THE pack manifest format SHALL be documented as a stable contract so
  third parties can author packs.

## 4. Design / approach

1. **Pack format** — `pack.json` manifest (name, version, files[], config
   defaults, roles) + a template tree. Pure data.
2. **Built-in packs** — embed a few packs via `internal/core/embed.go`;
   `--list-packs` enumerates them.
3. **Resolver** — `internal/core/pack.go`: resolve name → embedded pack, or a
   remote source with pinned SHA256 (reuse the `update.go` verify helper),
   fail-closed.
4. **Apply** — render the pack's templates into `.specd/` transactionally (no
   partial scaffold on error).

## 5. Non-goals

- No code execution from packs; declarative templates only.
- No central hosted registry service in this spec (format + local/remote fetch
  first; a hosted index can follow).
- No change to the spec lifecycle or gates.

## 6. Acceptance criteria

- `specd init --pack <built-in>` scaffolds from that pack; `--list-packs`
  enumerates embedded packs; no network needed for built-ins.
- Remote pack with wrong SHA256 ⇒ fail-closed, nothing written.
- Unknown/invalid pack ⇒ clear error, no partial scaffold.
- `--pack` omitted ⇒ identical to today; manifest documented; `make ci` green;
  stdlib-only.
