# Tasks — Deterministic Config Migration Command

## Wave 1 — Core conversion
- [ ] T1 — Implement legacy JSON to v2 YAML renderer
  - why: migration output must be stable and reviewable (Req 1,5)
  - role: builder
  - files: internal/core/config_migrate.go (new), internal/core/config_migrate_test.go
  - contract: decode legacy JSON into effective config/partial config and render v2 YAML with deterministic ordering; include template-style comments if feasible.
  - acceptance: same input bytes produce same YAML; output parses as v2 YAML; equivalent runtime config is preserved.
  - verify: go test ./internal/core -run ConfigMigrate
  - depends: schema-v2-namespacing/T7,yaml-loader-cascade/T5
  - requirements: 1,5

- [ ] T2 — Add migration safety validator
  - why: invalid or unsafe config must not be converted into trusted YAML (Req 5)
  - role: builder
  - files: internal/core/config_migrate.go, internal/core/config_validate.go, internal/core/config_migrate_test.go
  - contract: validate source JSON, reject unsupported/secret-bearing config, parse rendered YAML, and compare effective config equivalence.
  - acceptance: invalid JSON and invalid enum values return errors; valid legacy fixtures migrate; rendered YAML round-trips.
  - verify: go test ./internal/core -run ConfigMigrate
  - depends: T1
  - requirements: 5

## Wave 2 — CLI command
- [ ] T3 — Add `specd migrate config` command
  - why: users need an explicit project migration path (Req 1,4)
  - role: builder
  - files: internal/cmd/migrate.go (new), internal/cmd/registry.go, internal/core/commands.go, internal/cmd/migrate_test.go
  - contract: implement subcommand parsing; locate project root for project migration; refuse if YAML exists; write YAML atomically; rename JSON to `.bak` after validation.
  - acceptance: happy path writes config.yml and creates config.json.bak; no root returns exit 3; YAML-existing case fails closed.
  - verify: go test ./internal/cmd -run Migrate
  - depends: T2
  - requirements: 1,4,5

- [ ] T4 — Add `--dry-run`
  - why: users should review migration without mutations (Req 2)
  - role: builder
  - files: internal/cmd/migrate.go, internal/cmd/migrate_test.go
  - contract: print rendered YAML only or with clearly separated metadata; perform no writes/renames.
  - acceptance: filesystem unchanged; stdout parses as YAML if pure mode chosen; invalid source exits non-zero.
  - verify: go test ./internal/cmd -run 'Migrate.*DryRun'
  - depends: T3
  - requirements: 2

- [ ] T5 — Add `--global`
  - why: global legacy config needs migration outside project roots (Req 3)
  - role: builder
  - files: internal/cmd/migrate.go, internal/core/config_migrate.go, internal/cmd/migrate_test.go
  - contract: find legacy global JSON candidate, write canonical global YAML path, backup global JSON, and avoid requiring `.specd` root.
  - acceptance: isolated HOME/XDG test migrates global JSON; project root absence is allowed only with `--global`; collision handling is fail-closed.
  - verify: go test ./internal/cmd -run 'Migrate.*Global'
  - depends: T3
  - requirements: 3

## Wave 3 — Help/contracts
- [ ] T6 — Update command registry/help JSON tests
  - why: command metadata must stay aligned with dispatch (Req 4)
  - role: verifier
  - files: internal/core/commands.go, internal/cmd/registry.go, internal/cmd/*help*_test.go, internal/cmd/json_contract_test.go
  - contract: add migrate command metadata, usage, examples, and JSON schema considerations.
  - acceptance: registry/help drift tests pass; invalid flags/subcommands exit 2.
  - verify: go test ./internal/cmd ./internal/core -run 'Registry|Help|JSON|Migrate'
  - depends: T3
  - requirements: 4
