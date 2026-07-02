# S10 Tasks — Security Boundary Regression

Requirement coverage: R10 (+R4, R5). Dependencies: S2, S4, S5.

## Wave 1 — Baseline (after S2/S4/S5 green)

- [ ] Run `golangci-lint run` and `govulncheck ./...`; record clean baseline.
- [ ] Inventory security tests: `slug_test.go`, `env_test.go`,
  `acp_security_test.go`, `transport_http_auth_test.go`.
- **Validation:** `golangci-lint run && govulncheck ./...`

## Wave 2 — Core regression tests (depends on Wave 1)

- [ ] Path traversal: `..`, absolute, and separator slugs rejected pre-FS. File:
  `internal/core/slug_test.go` (extend).
- [ ] Env scrub: sensitive host vars absent from verify env. File:
  `internal/core/env_test.go` (extend).
- [ ] MCP auth: constant-time reject of bad/malformed token. File:
  `internal/mcp/transport_http_auth_test.go` (extend).
- [ ] Fail-closed sandbox reference test (cross-links S4). File:
  `internal/runner/runner_sandbox_test.go`.
- **Validation:** `go test ./... -run 'Slug|Security|Auth|Env|Scrub' -race -count=1`

## Wave 3 — Static scanners (depends on Wave 2)

- [ ] Confirm gosec finds no new issues after test additions.
- **Validation:** `golangci-lint run && govulncheck ./...`

## Rollout & cleanup

- [ ] Confirm affected package floors still met (`make cover-check`).
- **Rollback:** revert test extensions.
- **Completion evidence:** green security tests + clean gosec/govulncheck.
