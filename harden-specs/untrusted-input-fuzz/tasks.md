# Tasks — Untrusted-Input Defensiveness (A7)

## Wave 1 — Host-capabilities fuzz
- [ ] T1 — Garbage capabilities degrade safely
  - why: bad host must not panic / must clamp (Req 1)
  - role: verifier
  - files: internal/ host-caps negotiation test (host_caps_fuzz_test.go)
  - contract: Go fuzz + seeded table of malformed payloads (negative, oversized,
    nil, type-mismatch); assert no panic + conservative budget.
  - acceptance: fails if any payload panics or yields non-conservative budget.
  - verify: go test ./... -run HostCaps -fuzz=Fuzz -fuzztime=20s
  - depends: —
  - requirements: 1

## Wave 2 — Progress-timestamp bound
- [ ] T2 — Future timestamp cannot extend wait
  - why: skew/malicious worker must not stall program (Req 2)
  - role: verifier
  - files: internal/ progress-wait test
  - contract: stamp lastReport into the future; assert wait does not extend and
    MaxSteps fires/terminates; decision stays pure over (snapshot, policy).
  - acceptance: fails if future timestamp extends wait past bound.
  - verify: go test ./... -run "ProgressWait|Skew"
  - depends: —
  - requirements: 2
