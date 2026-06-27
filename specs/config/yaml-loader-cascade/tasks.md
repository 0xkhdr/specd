# Tasks — YAML Loader and Config Cascade

## Wave 1 — Path and decode foundation
- [ ] T1 — Add config path resolvers
  - why: every loader and command needs one source of truth for project/global config discovery (Req 2,3)
  - role: builder
  - files: internal/core/paths.go, internal/core/paths_test.go
  - contract: add `ConfigPaths(root)`, `LegacyConfigPath(root)`, and `GlobalConfigPaths()`; keep `ConfigPath(root)` as a deprecated JSON-path compatibility wrapper unless all callers are updated in this task.
  - acceptance: project paths are ordered yml/yaml/json; global paths honor `os.UserConfigDir()`/XDG and home fallback; no filesystem mutation occurs.
  - verify: go test ./internal/core -run 'Path|ConfigPath'
  - depends: —
  - requirements: 2,3

- [ ] T2 — Implement format-aware single-file decode
  - why: YAML and JSON must share one parse/diagnostic contract (Req 1,5)
  - role: builder
  - files: internal/core/config_loader.go (new), go.mod, go.sum
  - contract: implement `LoadConfigFromPath(path string)` or equivalent; detect `.yml`, `.yaml`, `.json`; return partial config plus diagnostics; reject unsupported extensions.
  - acceptance: valid YAML and valid JSON decode; invalid syntax identifies source path and format; missing file is represented distinctly from invalid file.
  - verify: go test ./internal/core -run ConfigLoader
  - depends: T1
  - requirements: 1,5

- [ ] T3 — Select active config candidates with diagnostics
  - why: duplicate config files must be deterministic and visible (Req 2,3,5)
  - role: builder
  - files: internal/core/config_loader.go, internal/core/config_loader_test.go
  - contract: choose first existing project candidate and first existing global candidate; emit warning/info diagnostics for ignored lower-priority candidates and deprecated JSON candidates.
  - acceptance: `.specd/config.yml` wins over `.specd/config.json`; missing global/project configs default cleanly; path diagnostics are stable.
  - verify: go test ./internal/core -run 'ConfigCandidate|ConfigLoader'
  - depends: T2
  - requirements: 2,3,5

## Wave 2 — Merge and compatibility wrappers
- [ ] T4 — Implement presence-aware deep merge
  - why: absent values must fall through while explicit false/zero must override (Req 4)
  - role: builder
  - files: internal/core/config_merge.go (new), internal/core/config_merge_test.go
  - contract: merge embedded defaults → global partial → project partial; recursively merge structs; replace slices; preserve explicit booleans, zero ints, and empty strings when present.
  - acceptance: global `maxWorkers: 4` plus project `enabled: true` yields both; project `enabled: false` overrides global true; project empty custom list clears global custom gates.
  - verify: go test ./internal/core -run ConfigMerge
  - depends: T2
  - requirements: 4

- [ ] T5 — Wrap legacy `LoadConfig` and strict loader around cascade
  - why: existing callers need compatible behavior while new callers get diagnostics (Req 4,5)
  - role: builder
  - files: internal/core/specfiles.go, internal/core/config_validate.go, internal/core/config_loader.go
  - contract: keep `LoadConfig(root) Config` permissive; add/adjust diagnostic loader; make strict loader validate selected global/project files and merged effective config.
  - acceptance: existing tests expecting default-on-missing/invalid still pass or are intentionally updated; strict invalid YAML/JSON fails with source-aware diagnostics.
  - verify: go test ./internal/core -run 'Config|Specfiles'
  - depends: T3,T4
  - requirements: 4,5

- [ ] T6 — Update config digest/source users
  - why: fusion policy and doctor cannot assume `.specd/config.json` (Req 5)
  - role: builder
  - files: internal/core/fusion.go, internal/cmd/doctor.go, internal/core/config_loader.go
  - contract: expose effective config digest and selected source paths; update fusion policy digest and doctor diagnostics to use loader metadata.
  - acceptance: digest changes when selected YAML changes; legacy JSON still produces a digest; missing config reports defaults.
  - verify: go test ./internal/core ./internal/cmd -run 'FusionPolicy|Doctor|Config'
  - depends: T5
  - requirements: 5

## Wave 3 — Regression and safety checks
- [ ] T7 — Protect machine JSON state from config format changes
  - why: action plan explicitly keeps runtime state JSON (Req 1)
  - role: reviewer
  - files: internal/core/*_test.go, internal/cmd/*_test.go
  - contract: add assertions that `StatePath`, program/session/runtime paths, and integration metadata paths remain `.json` and are not routed through config path logic.
  - acceptance: tests fail if machine-owned files are renamed to YAML.
  - verify: go test ./internal/core ./internal/cmd -run 'Path|State|Program|Session|Config'
  - depends: T5
  - requirements: 1
