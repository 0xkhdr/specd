# Tasks — Custom-Gate Trust Boundary & Optional Sandbox (A5)

## Wave 1 — Documentation (ships independently)
- [ ] T1 — Document custom-gate trust boundary
  - why: make the sandbox asymmetry explicit (Req 1)
  - role: builder
  - files: docs/custom-gates.md, docs/validation-gates.md
  - contract: state gate command = trusted operator input, host-run, no sandbox
    by default; contrast with verify's fail-closed sandbox.
  - acceptance: doc describes boundary + contrast.
  - verify: N/A (doc review)
  - depends: —
  - requirements: 1

## Wave 2 — Opt-in sandbox
- [ ] T2 — Route custom gate through verify sandbox runner
  - why: parity option with verify (Req 2,3)
  - role: builder
  - files: internal/core/customgate.go, internal/spec/runner_sandbox.go (reuse)
  - contract: `--sandbox` runs gate via verify sandbox; unset = current host path;
    backend-unavailable fails closed; scrubbed env in both modes.
  - acceptance: sandboxed and host paths both scrub env; fail-closed honored.
  - verify: go test ./internal/core/ ./internal/spec/ -run "CustomGate|Sandbox"
  - depends: T1
  - requirements: 2,3

- [ ] T3 — Parity + env-scrub tests
  - why: lock both modes' guarantees (Req 2,3)
  - role: verifier
  - files: internal/core/customgate_test.go
  - contract: assert no secret env leaks in host or sandbox mode; assert
    fail-closed when backend absent.
  - acceptance: fails if env leaks or opt-in fails open.
  - verify: go test ./internal/core/ -run "CustomGate"
  - depends: T2
  - requirements: 2,3
