# Tasks — Coverage-Floor Extension (A8)

## Wave 1 — Add modest floors
- [x] T1 — Floor unguarded packages
  - why: regressions must not hide under overall (Req 1,3)
  - role: builder
  - files: scripts/coverage-check.sh
  - contract: add floors for internal/spec, internal/runner, internal/pack,
    internal/schema at/just-below current measured; lower nothing.
  - acceptance: coverage-check passes today with new floors.
  - verify: scripts/coverage-check.sh
  - depends: —
  - requirements: 1,3

## Wave 2 — Raise internal/spec
- [x] T2 — Test role/phase/status
  - why: cover substantive untested logic (Req 2)
  - role: verifier
  - files: internal/spec/role_test.go, internal/spec/phase_test.go,
    internal/spec/status_test.go
  - contract: add tests covering role.go and phase/status logic.
  - acceptance: internal/spec coverage rises meaningfully above ~50%.
  - verify: go test ./internal/spec/ -cover
  - depends: —
  - requirements: 2

- [x] T3 — Ratchet internal/spec floor + document
  - why: lock the gain on the documented path (Req 2,3)
  - role: builder
  - files: scripts/coverage-check.sh, docs/TESTING.md
  - contract: raise internal/spec floor to new measured; note additions in
    TESTING.md as ratchet steps toward 85/90/95.
  - acceptance: floor raised; TESTING.md updated; CI green.
  - verify: scripts/coverage-check.sh
  - depends: T1,T2
  - requirements: 2,3
