# Tasks — Config Corruption & Secret-Diagnostic Matrix (A6)

## Wave 1 — Corruption matrix
- [ ] T1 — Negative parse matrix
  - why: corrupt config must fail loud (Req 1)
  - role: verifier
  - files: internal/config/config_corruption_test.go
  - contract: table-driven cases — truncated YAML, duplicate JSON keys; assert
    clear error / defined resolution, no partial apply.
  - acceptance: each case red without handling, green with.
  - verify: go test ./internal/config/ -run Corruption
  - depends: —
  - requirements: 1

## Wave 2 — Dual-file conflict
- [ ] T2 — doctor flags dual config files
  - why: stale file must not silently win (Req 2)
  - role: builder
  - files: internal/config/ (doctor check), internal/cmd/ as needed
  - contract: detect both config.json + config.yml; deterministic precedence;
    doctor reports conflict + winner.
  - acceptance: doctor flags dual-file; precedence tested.
  - verify: go test ./internal/config/ ./internal/cmd/ -run "Doctor|DualFile"
  - depends: —
  - requirements: 2

## Wave 3 — Secret diagnostic guard
- [ ] T3 — Secret-name never echoed
  - why: secrets must not leak to diagnostics (Req 3)
  - role: verifier
  - files: internal/config/config_secret_diag_test.go
  - contract: feed SPECD_<SECRET> override through diagnostic + rejection paths;
    assert value never printed; key name allowed.
  - acceptance: fails if any diagnostic echoes a secret value.
  - verify: go test ./internal/config/ -run "Secret|Diag"
  - depends: —
  - requirements: 3
