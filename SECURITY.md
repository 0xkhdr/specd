# Security Policy

`specd` is a local, single-binary coding harness with **zero runtime dependencies** and **no
network calls** in any gate, DAG, or report path. Its security posture rests on one idea: the
harness is deterministic and the agent is not, so every trust decision is made by pure,
locally-auditable code — never by an LLM. This document states the threat model those decisions
defend against, the isolation contract for the one place `specd` runs untrusted commands, and how
to report a vulnerability.

## Attacker model

`specd` is typically driven by an AI agent over content the operator did not fully author (spec
text, task tables, dependency names, verify commands). We assume that content can be hostile.

### 1. Hostile spec / tasks content

- **Threat:** a `requirements.md` / `design.md` / `tasks.md` crafted to smuggle instructions to
  the driving agent (prompt injection), or to steer the harness into reading/writing outside the
  spec tree.
- **Defenses:**
  - The **opt-in security gate** (`specd check --security`) scans tracked files with the
    `injection` scanner (override-instruction, role-override, hidden-instruction, and zero-width
    smuggling rules) and reports hits deterministically.
  - **Slug validation** (`core.ValidateSlug`, regex `^[a-z0-9][a-z0-9-]*$`) rejects `..`,
    absolute paths, and any separator, so a spec slug can never escape `.specd/specs/<slug>/`.
    Every spec-resolving verb validates the slug before touching the filesystem.
  - No LLM sits in any gate, DAG, or report path — injected text cannot change a pass/fail
    decision, only the human-facing prose an agent later reads.

### 2. Hostile verify lines (arbitrary shell by design)

- **Threat:** a task's `verify:` command is arbitrary shell, executed by the harness. This is
  the largest attack surface: a malicious verify line could read secrets from the environment,
  reach the network, or scribble outside the repo.
- **Defenses — the isolation contract (`internal/core/verify`):**
  - **Scrubbed environment.** Verify commands run with an allowlisted environment of `HOME`,
    `PATH`, and `TMPDIR` only. Secrets exported into `specd`'s own process (cloud keys, tokens)
    never cross into the verify subprocess, so they cannot be echoed into evidence logs or CI
    output.
  - **Optional sandbox (`--sandbox`, bwrap-compatible).** With `--sandbox` the command is wrapped
    in a `bwrap`-style jail: `--unshare-all` (no network, no host namespaces), `--ro-bind / /`
    (read-only root), a private `--tmpfs /tmp`, and only the repo directory bind-mounted writable
    as the working dir. A missing/unresolvable sandbox binary **fails closed** (exit 127,
    `sandbox binary … unavailable`) — it never silently downgrades to an unsandboxed run.
  - **`--revert-on-fail`.** On a failing verify the working tree is restored to its pre-run state
    via git diff/apply over stdin (no temp patch files), leaving a clean tree and no stray
    artifacts.
  - **Redacted evidence.** The secrets scanner masks candidate secrets (first/last 4 chars only)
    so a scanner that exists to find leaks cannot itself print one into CI logs. The same central
    redactor also collapses an absolute home directory (`/home/<u>`, `/Users/<u>`, `/root`) to `~`,
    and guards the one free-form telemetry field (`attestation_ref`) so a secret or home path in
    worker-reported telemetry never reaches the ledger (spec 07 R5.2/R5.4).
  - **Metadata-only telemetry, bounded metric labels.** Telemetry is metadata-only by schema — no
    prompt, response, chain-of-thought, file content, or raw output field exists. Metric series
    carry only allowlisted labels (`spec`/`status`/`verdict`/`task`); high-cardinality correlation
    stays in the trace JSONL (R5.1). See `docs/telemetry-schema.md`.
  - **Workspace-scoped evidence references.** `evidence_ref` must be workspace-relative or
    content-addressed; a URL, absolute path, or `..` traversal is rejected in the core schema on
    both append and decode, so an evidence locator can never point off the machine or off the
    network (R5.3).

### 3. Hostile dependency names (typosquatting)

- **Threat:** a `go.mod` requiring a typosquatted lookalike of a popular module
  (`golang.org/x/tolls` for `…/tools`) to pull in malicious code.
- **Defenses:**
  - The **`slopsquat` scanner** flags module paths within a small Damerau–Levenshtein distance of
    a known-popular package that are not an exact match.
  - CI runs **`govulncheck`** (pinned to `v1.5.0` in `.github/workflows/ci.yml`, never `@latest`)
    against the whole module, and the release workflow generates an SBOM (Syft).
  - Zero runtime dependencies keeps this surface minimal by construction.

## Trust anchor: evidence integrity

A task completes **only** against a passing verify record whose exit code is `0` and which is
pinned to a resolvable git HEAD. **There is no bypass flag, and one must never be added.** The
escalation ratchet can reset a failure counter but never substitutes for a passing verify. This
is the property every other guarantee ultimately rests on.

## Allowlist fails closed

The security gate's fingerprint allowlist (`.specd/security/allow.json`) requires a reasoned
entry per waived finding. A missing file is an empty allowlist; a **corrupt, unparseable, or
reason-less** allowlist fails closed — it suppresses nothing and surfaces an error-severity
finding, so a broken allowlist can never silently hide a real leak.

## Scan boundary

The gate scans git-tracked files but excludes paths that only yield false positives: dependency
lockfiles (`go.sum`, `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `Cargo.lock`), and the
`testdata/`, `.specd/`, `reference/`, `vendor/`, and `.git/` trees.

## Current enforcement boundary

Security hardening remains opt-in and incomplete in prototype mode. Roles describe intended
capability but do not grant or revoke host tools. Declared task files are not yet compared with a
harness-derived git diff. The security scan excludes runtime `.specd/` content and untracked files,
and verification is unsandboxed unless `--sandbox` is supplied. Use `specd check --security` and
`specd verify --sandbox` during migration; neither substitutes for production profile enforcement,
scope validation, runtime-context scanning, or host isolation planned by Domain 06.

## Reporting a vulnerability

Report suspected vulnerabilities **privately** — do not open a public issue for an unfixed flaw.

- **GitHub:** open a private advisory at
  <https://github.com/0xkhdr/specd/security/advisories/new>.
- **Email:** `0xkhdr@gmail.com` (put `specd security` in the subject).

Please include a description, affected version/commit, and a minimal reproduction. Expect an
acknowledgement within a few days; fixes are prioritized by severity and coordinated with you
before public disclosure.
