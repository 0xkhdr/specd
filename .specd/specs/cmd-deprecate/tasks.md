# Tasks — cmd-deprecate

## Wave 1
- [ ] T1 — Add Hidden/DeprecatedIn/RemovedIn to CommandMeta
  - why: Visibility + removal scheduling need first-class registry fields
  - role: builder
  - files: internal/core/commands.go
  - contract: CommandMeta gains Hidden bool, DeprecatedIn string, RemovedIn string; defaults preserve current behavior
  - acceptance: Existing tests compile and pass with new zero-value fields
  - verify: go build ./... && go test ./internal/core/
  - depends: —

## Wave 2
- [ ] T2 — Mark meta-hidden commands (mcp, fusion, version, help)
  - why: Daily palette must exclude non-workflow meta to stay agent-containable
  - role: builder
  - files: internal/core/commands.go, internal/cmd/help.go
  - contract: Four commands set Hidden=true; help omits them unless --all
  - acceptance: `help` count == daily palette; `help --all` includes the four
  - verify: test "$(specd help --json | jq '.commands|length')" -lt "$(specd help --all --json | jq '.commands|length')"
  - depends: T1
- [ ] T3 — Retire update/uninstall/migrate from runtime
  - why: Install/maintenance concerns do not belong in the runtime palette
  - role: builder
  - files: internal/core/commands.go, internal/cmd/update.go, internal/cmd/uninstall.go, internal/cmd/migrate.go, scripts/install.sh
  - contract: Each retired command becomes a stub exiting non-zero with migration hint; install.sh gains update/uninstall flows; migrate reachable via init --migrate
  - acceptance: `specd update` exits non-zero with hint; install.sh covers the relocated flows
  - verify: ! specd update 2>/dev/null
  - depends: T1

## Wave 3
- [ ] T4 — Enforce palette ceiling
  - why: A machine-checked ceiling is the only durable guard against re-bloat
  - role: reviewer
  - files: internal/core/commands_test.go
  - contract: TestPaletteCeiling asserts non-hidden survivors ≤16 and total ≤20
  - acceptance: Test passes with current surface; fails if a command is added
  - verify: go test ./internal/core/ -run TestPaletteCeiling
  - depends: T2,T3
- [ ] T5 — Gate cmd-deprecate spec
  - why: Must pass validation before mcp-sync consumes the reduced surface
  - role: verifier
  - files: .specd/specs/cmd-deprecate/
  - contract: `specd check cmd-deprecate` exits 0
  - acceptance: All core gates pass
  - verify: specd check cmd-deprecate
  - depends: T4
