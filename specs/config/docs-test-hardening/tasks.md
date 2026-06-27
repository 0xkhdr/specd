# Tasks — Documentation and Test Hardening

## Wave 1 — Documentation
- [ ] T1 — Update command reference
  - why: users need exact CLI/config contracts (Req 1)
  - role: builder
  - files: docs/command-reference.md
  - contract: document config locations, lookup order, v2 YAML schema, legacy JSON support, env precedence, doctor config health, and `specd migrate config` flags.
  - acceptance: examples include global and project config; migration command usage matches command metadata.
  - verify: N/A
  - depends: scaffold-global-init/T6,migrate-config/T6,env-precedence/T5
  - requirements: 1

- [ ] T2 — Update user guide
  - why: users need a conceptual guide, not only command syntax (Req 1)
  - role: builder
  - files: docs/user-guide.md
  - contract: add "Global vs Project Config" with YAML examples and precedence ladder; note machine JSON files stay JSON.
  - acceptance: section explains when to use global vs project and how to migrate legacy config.
  - verify: N/A
  - depends: T1
  - requirements: 1

- [ ] T3 — Update contributor guide/security notes
  - why: maintainers need architecture and safety constraints (Req 2)
  - role: builder
  - files: docs/contributor-guide.md, SECURITY.md or docs/validation-gates.md if appropriate, internal/core/config_loader.go comments
  - contract: explain loader layers, schema translation, merge presence semantics, validation, diagnostics, and secret-bearing orchestration rejection.
  - acceptance: contributor docs mention compatibility wrappers and how to add a config field safely.
  - verify: N/A
  - depends: yaml-loader-cascade/T5,schema-v2-namespacing/T6,env-precedence/T3
  - requirements: 2

## Wave 2 — Unit/regression tests
- [ ] T4 — Add YAML/JSON compatibility fixtures
  - why: legacy and v2 config must stay equivalent (Req 3)
  - role: verifier
  - files: internal/core/testdata/config/ or inline tests, internal/core/config_loader_test.go
  - contract: fixtures cover default verify, report, roles, gates, verify sandbox, orchestration transport/program/resilience, MCP, custom gates.
  - acceptance: equivalent v1 JSON and v2 YAML produce identical effective runtime configs after normalization.
  - verify: go test ./internal/core -run 'Config.*Compat|Config.*RoundTrip'
  - depends: schema-v2-namespacing/T8,yaml-loader-cascade/T5
  - requirements: 3

- [ ] T5 — Add docs sample parse tests
  - why: docs examples should not rot (Req 1,3)
  - role: verifier
  - files: internal/core/config_docs_test.go, docs/user-guide.md, docs/command-reference.md
  - contract: parse embedded/sample YAML snippets or mirrored fixtures used in docs.
  - acceptance: examples from docs decode and validate; snippets remain synchronized or tests clearly name fixture source.
  - verify: go test ./internal/core -run ConfigDocs
  - depends: T1,T2,T4
  - requirements: 1,3

- [ ] T6 — Add machine JSON invariant tests
  - why: config YAML migration must not rename machine state files (Req 1,3)
  - role: verifier
  - files: internal/core/paths_test.go, internal/core/program_state_test.go, internal/core/state_test.go
  - contract: assert state/program/session/integrations runtime paths retain `.json` and serializers remain deterministic.
  - acceptance: tests would fail if machine files switched to YAML.
  - verify: go test ./internal/core -run 'JSONInvariant|Path|State|Program'
  - depends: yaml-loader-cascade/T7
  - requirements: 3

## Wave 3 — End-to-end command coverage and CI
- [ ] T7 — Add init/doctor/migrate E2E tests
  - why: user-facing workflow is the release contract (Req 4)
  - role: verifier
  - files: internal/cmd/init_test.go, internal/cmd/doctor_test.go, internal/cmd/migrate_test.go, internal/testharness as needed
  - contract: isolated HOME/XDG; fresh init creates project/global YAML; doctor validates; legacy JSON migrates to YAML with backup; JSON outputs ANSI-free.
  - acceptance: tests are deterministic and do not touch real user config.
  - verify: go test ./internal/cmd -run 'Init|Doctor|Migrate|JSON'
  - depends: scaffold-global-init/T6,migrate-config/T6
  - requirements: 4

- [ ] T8 — Run full local gates and backfill regressions
  - why: broad config changes can break unrelated commands (Req 5)
  - role: verifier
  - files: any failing tests/docs as needed
  - contract: run `make test`; preferably run `make ci`; fix legitimate regressions or record blockers.
  - acceptance: race tests pass; coverage policy passes; no real HOME writes; evidence attached to implementation task.
  - verify: make test && make ci
  - depends: T4,T5,T6,T7
  - requirements: 5
