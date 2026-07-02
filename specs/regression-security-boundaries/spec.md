# S10 — Security Boundary Regression

## 1. Purpose and requirement coverage

Guarantee path-traversal prevention, env scrubbing, MCP auth, and sandbox
isolation hold. Covers **R10** (plus R4, R5 security aspects).

## 2. Verified current state

- Slug validation (path-traversal guard): `internal/core/slug.go`, tested by
  `slug_test.go`. Path helpers in `internal/core/paths.go`,
  `runtime_paths.go` (+ `runtime_paths_test.go`).
- Env scrubbing owned by `internal/cmd/verify.go`; env helpers in
  `internal/core/env.go` (`env_test.go`).
- ACP security: `internal/core/acp_security_test.go`.
- MCP auth: `internal/mcp/transport_http_auth_test.go`; constant-time compare in
  `internal/mcp/transport_http.go`.
- Fail-closed sandbox: `internal/runner/runner_sandbox.go` (see S4).
- Threat model + boundaries: `SECURITY.md`.
- Static security scan: `golangci-lint` with `gosec` (`.golangci.yml`);
  `govulncheck` in CI `analyze:` job.

## 3. Proposed design and end-to-end flow

Tests assert: slugs containing `..`, absolute paths, or separators are rejected
before any filesystem access; the verify env is scrubbed of sensitive host vars;
MCP auth uses constant-time comparison and rejects malformed tokens; sandbox
selection fails closed (S4). Supplement with `gosec` + `govulncheck` gates.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** slug validation rules; env scrub allowlist; auth contract;
  fail-closed policy.
- **Dependencies:** S2 (state paths), S4 (sandbox), S5 (MCP auth).

## 5. Invariants, security, errors, observability, compatibility, rollback

- **Security:** path traversal blocked; env leakage blocked; auth constant-time;
  isolation fail-closed (INV4).
- **Errors:** rejections return typed errors, not partial side effects.
- **Rollback:** additive tests + scanner gates.

## 6. Acceptance criteria and validation commands

- `go test ./... -run 'Slug|Security|Auth|Env|Scrub' -race -count=1` passes.
- `golangci-lint run` clean (gosec included).
- `govulncheck ./...` reports no known vulnerabilities.

## 7. Open decisions and deviations

- Deviation R10: analysis plan marked `internal/core/slug.go` "(inferred)" — it
  is verified present with `slug_test.go`. No inference needed.
- F4 open: custom gates run unisolated (trust boundary) — covered by S4; a
  registration-time warning / opt-in trust flag is a candidate follow-up.
