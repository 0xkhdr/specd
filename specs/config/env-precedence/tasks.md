# Tasks — Environment Precedence and Format Control

## Wave 1 — Apply env as final layer
- [x] T1 — Inventory existing `SPECD_*` config-affecting env vars
  - why: preserve behavior before moving override order (Req 1)
  - role: investigator
  - files: internal/core/env.go, internal/core/specfiles.go, internal/cmd/*.go, docs/command-reference.md
  - contract: list env vars that alter config/effective policy versus command output; identify existing tests.
  - acceptance: implementation notes in PR/task evidence enumerate vars and target fields.
  - verify: rg 'SPECD_' internal docs
  - depends: yaml-loader-cascade/T5
  - requirements: 1

- [x] T2 — Apply config env overrides after cascade merge
  - why: env must be top precedence (Req 1)
  - role: builder
  - files: internal/core/config_loader.go, internal/core/env.go, internal/core/config_env_test.go
  - contract: introduce final env overlay step after project/global merge; preserve `EnvInt` clamping/warnings.
  - acceptance: env beats project YAML and global YAML; absent env leaves merged value unchanged; existing env tests pass.
  - verify: go test ./internal/core -run 'Config.*Env|Env'
  - depends: T1
  - requirements: 1

- [x] T3 — Validate env-overridden effective config
  - why: env must not bypass safety checks (Req 4)
  - role: builder
  - files: internal/core/config_validate.go, internal/core/config_env_test.go
  - contract: run effective config validation after env overlay; unsafe orchestration values are rejected/diagnosed consistently with file config.
  - acceptance: invalid env value produces diagnostic/fallback per strict/permissive behavior; secret-bearing policy remains impossible.
  - verify: go test ./internal/core -run 'Config.*Env|ConfigStrict'
  - depends: T2
  - requirements: 1,4

## Wave 2 — Diagnostics and optional format preference
- [x] T4 — Add env source diagnostics
  - why: agents need to understand why effective config differs from files (Req 2)
  - role: builder
  - files: internal/core/config_loader.go, internal/core/fusion.go, internal/cmd/doctor.go, internal/core/config_env_test.go
  - contract: record env var name, target field path, and layer `env` in load metadata; expose in strict diagnostics/policy/doctor without dumping sensitive raw values.
  - acceptance: doctor/fusion JSON can identify env overrides; text mode remains concise.
  - verify: go test ./internal/core ./internal/cmd -run 'Env|FusionPolicy|Doctor'
  - depends: T2
  - requirements: 2,4

- [x] T5 — Implement or explicitly reject `SPECD_CONFIG_FORMAT`
  - why: action plan proposes format control; behavior must be clear (Req 3)
  - role: builder
  - files: internal/core/config_loader.go, internal/core/env.go, internal/core/config_env_test.go, docs/command-reference.md
  - contract: either implement `yaml|json` as a candidate filter/preference with diagnostics, or document and test that it is unsupported for this release.
  - acceptance: invalid value warns/diagnoses; machine JSON paths unaffected; tests cover chosen behavior.
  - verify: go test ./internal/core -run 'ConfigFormat|Config.*Env'
  - depends: T4
  - requirements: 3

## Wave 3 — Precedence regression tests
- [x] T6 — Add full precedence ladder tests
  - why: cascade behavior is the core contract (Req 1,2)
  - role: verifier
  - files: internal/core/config_precedence_test.go
  - contract: test embedded defaults < global YAML < project YAML/JSON < env; include explicit false/zero and list replacement cases.
  - acceptance: all precedence examples from action plan are encoded as tests.
  - verify: go test ./internal/core -run ConfigPrecedence
  - depends: T2,T4
  - requirements: 1,2
