# 05-security-suite — Replace the cosmetic security gate with a real one

Wave 1. FINDINGS refs: B.4, C.4, D-tier1 item 7.

## Problem

The current opt-in security gate is a handful of hardcoded literal
substring patterns. It will catch neither a real key nor a typosquatted
dependency, and `check --security` passing therefore reads as assurance it
cannot provide — **worse than absent** (C.4). v1 shipped a real
deterministic suite: entropy-based secrets scanner with a reasoned
allowlist (`.specd/security/allow.json`), injection heuristics, `slopsquat`
(manifest parsing + edit distance against an embedded popular-package
list), per-scanner `off/warn/error` config, findings recorded in
`state.security`. FINDINGS verdict: **port (adapt)** the suite's design.

This is a defensive-security feature: it scans the operator's own
repository for accidentally committed secrets, prompt-injection payloads in
tracked files, and typosquatted dependencies.

## Requirements (EARS)

- R1 (secrets): WHEN the secrets scanner runs, THE SYSTEM SHALL flag
  high-entropy string literals and known-format credentials (prefix +
  length + charset heuristics, e.g. AWS `AKIA…`, GitHub `ghp_…`, PEM
  blocks) in tracked files, reporting `file:line`, matched rule, and a
  redacted excerpt — never the full candidate secret.
- R2 (allowlist): WHEN a finding matches an entry in
  `.specd/security/allow.json`, THE SYSTEM SHALL suppress it; every
  allowlist entry SHALL require a non-empty `reason` and pin the exact
  fingerprint (file + rule + content hash), and an entry missing a reason
  SHALL invalidate the allowlist load (fail closed).
- R3 (injection): WHEN the injection scanner runs, THE SYSTEM SHALL flag
  prompt-injection heuristics in tracked text/markdown (e.g. imperative
  override phrases, hidden-instruction markers, zero-width/homoglyph
  smuggling) as documented, versioned rules.
- R4 (slopsquat): WHEN the slopsquat scanner runs, THE SYSTEM SHALL parse
  dependency manifests present in the repo (at minimum `go.mod`; design for
  `package.json`, `requirements.txt` additions) and flag names within small
  edit distance of an embedded popular-package list, excluding exact
  matches.
- R5 (config): THE SYSTEM SHALL support per-scanner severity
  `off|warn|error` in config; `error` findings fail the gate (exit 1),
  `warn` findings print but pass, `off` skips the scanner; defaults:
  secrets=error, injection=warn, slopsquat=warn.
- R6 (recording): WHEN the gate runs, THE SYSTEM SHALL record findings
  (rule, fingerprint, severity, allowlisted-or-not) under `state.security`
  so reports and history can consume them.
- R7 (determinism): scanners SHALL be pure functions of tracked file
  contents + embedded rule data + allowlist — no network, no LLM, stable
  ordering of findings.

## Design notes / best practice

- Entropy: Shannon entropy over candidate tokens (length ≥ 20, charset
  base64/hex) with per-charset thresholds; combine with format rules to
  keep false positives tolerable. Calibrate thresholds against this repo's
  own tree as the fixture (must run clean or allowlisted).
- Redaction (R1): show first/last 4 chars max. A secrets scanner that
  prints secrets into terminals/CI logs creates the leak it exists to
  prevent.
- Fingerprint = SHA-256 of (rule id + relative path + matched content);
  content moving lines does not invalidate an allowlist entry, editing the
  match does — that is the point of a *reasoned* allowlist.
- Popular-package list: embedded via `go:embed`, versioned, with provenance
  comment (source + date). Edit distance: Damerau-Levenshtein ≤ 1 for
  short names, ≤ 2 for length ≥ 8; exact match = not a finding.
- Scan tracked files only (`git ls-files` semantics) — untracked scratch
  must not fail CI; document this boundary.
- v1's suite under `reference/` is design input only; re-implement, never
  copy (museum rule).
- Fixture-driven tests: one fixture repo per scanner under `testdata/`,
  with true-positive, true-negative, and allowlisted cases; injection and
  secrets fixtures use synthetic keys (e.g. `AKIA` + padding) never real
  material.
- If the placeholder gate ships anywhere meanwhile, its docs must say
  "placeholder" until this spec lands (interim honesty per FINDINGS).

## Out of scope

- Network-fed advisory databases, dependency CVE scanning.
- Scanning git history (working tree only, this pass).

## Acceptance

- Synthetic AWS-format key in fixture → error finding, redacted output;
  allowlisting with reason suppresses it; reason-less entry fails closed.
- `go.mod` fixture with `golang.org/x/tolls` (sic) → slopsquat warn.
- Suite runs clean on this repository itself; findings land in
  `state.security`; severity matrix honored; full suite green.
