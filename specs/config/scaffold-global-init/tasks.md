# Tasks — Scaffold and Global Init Migration

## Wave 1 — Project scaffold YAML
- [x] T1 — Switch scaffold manifest to `config.yml`
  - why: new projects should be YAML-first (Req 1)
  - role: builder
  - files: internal/core/scaffold.go, internal/core/embed_templates/config.yml, internal/core/embed.go, internal/core/scaffold_test.go
  - contract: replace canonical scaffold entry `.specd/config.json` with `.specd/config.yml`; source content from embedded YAML template.
  - acceptance: fresh init creates config.yml; scaffold missing checks no longer require config.json.
  - verify: go test ./internal/core -run Scaffold
  - depends: schema-v2-namespacing/T7
  - requirements: 1

- [x] T2 — Update `specd init` project config write path
  - why: CLI bootstrap must match scaffold manifest (Req 1)
  - role: builder
  - files: internal/cmd/init.go, internal/cmd/init_test.go, internal/testharness as needed
  - contract: init writes `.specd/config.yml` for new repos; does not overwrite existing config; preserves existing init options/outputs.
  - acceptance: init test sees YAML config; rerunning init is idempotent; legacy JSON-only repo is not mutated.
  - verify: go test ./internal/cmd -run Init
  - depends: T1
  - requirements: 1

## Wave 2 — Global config scaffold
- [x] T3 — Add global config creation helper
  - why: user-level defaults need a safe first-run write path (Req 2)
  - role: builder
  - files: internal/core/config_scaffold.go (new) or internal/cmd/init.go, internal/core/config_loader.go
  - contract: detect any existing global config candidate; if none exists, create canonical global YAML path using embedded template; create parent directories.
  - acceptance: helper is idempotent; failure returns a warning/error value without partial corrupt writes.
  - verify: go test ./internal/core -run 'GlobalConfig|ConfigScaffold'
  - depends: yaml-loader-cascade/T1, schema-v2-namespacing/T7
  - requirements: 2

- [x] T4 — Invoke global scaffold from `specd init`
  - why: users should get global config automatically on first init (Req 2)
  - role: builder
  - files: internal/cmd/init.go, internal/cmd/init_test.go
  - contract: call helper after/around project scaffold; print one-line notice on creation; warn but continue on non-fatal global creation failure.
  - acceptance: test with isolated HOME/XDG creates global config once; second init leaves file untouched; project init succeeds if global write denied but project write succeeds.
  - verify: go test ./internal/cmd -run Init
  - depends: T3
  - requirements: 2

## Wave 3 — Legacy notices and doctor
- [x] T5 — Add legacy config deprecation notices
  - why: users need clear migration guidance without forced rewrites (Req 3)
  - role: builder
  - files: internal/cmd/init.go, internal/core/config_loader.go, internal/cmd/init_test.go
  - contract: when project JSON exists without YAML, print `config.json is deprecated; run specd migrate config to convert to config.yml.`; when both exist, report JSON ignored.
  - acceptance: notices are deterministic and absent for clean YAML projects.
  - verify: go test ./internal/cmd -run Init
  - depends: yaml-loader-cascade/T3,T5
  - requirements: 3

- [x] T6 — Teach doctor about YAML scaffold and global config health
  - why: doctor should be the central health check (Req 4)
  - role: builder
  - files: internal/cmd/doctor.go, internal/cmd/doctor_test.go
  - contract: doctor accepts config.yml as canonical; `--fix` creates safe missing YAML scaffold; JSON mode includes global/project config diagnostics; invalid global config makes config-policy check fail/warn per severity.
  - acceptance: doctor passes fresh YAML init; doctor flags invalid global YAML; `--fix` does not rewrite invalid custom user values.
  - verify: go test ./internal/cmd -run Doctor
  - depends: T1,T3,yaml-loader-cascade/T6
  - requirements: 4
