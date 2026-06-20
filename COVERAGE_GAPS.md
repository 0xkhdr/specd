# Dark-path inventory (`internal/core`)

Generated per `tasks.md` D1 from:

```
go test ./internal/core/... -coverprofile=coverage-core.out
go tool cover -func=coverage-core.out
```

Package totals after the rebuild: **internal/core 74.1%** (floor 70%, long-term
target 95%). Every function below the §5 target is listed with its disposition:
either an owning test task or a "won't test" with a reason.

## Integrity-critical — confirmed 100% (no regression permitted)

| Function | File | Coverage |
|----------|------|----------|
| `ValidateSlug` | `slug.go:14` | 100% |
| `WithSpecLock` | `lock.go:122` | 100% |
| `LoadState` | `state.go:209` | 100% |
| `UsageError` / `GateError` / `NotFoundError` / `Error` / `IsSpecdError` | `exit.go` | 100% |

These are guarded against regression by the §5 contract; the CI lint (E1) and
the coverage floor (D4) fail if they drop.

## Genuine logic gaps — owned by a follow-up test task (toward 95%)

| Function | File | % | Owner |
|----------|------|---|-------|
| `NextRunnable` | `dag.go:162` | 39% | D2-follow: frontier edge cases (cyclic / partially-blocked graphs) |
| `SenseProgramOrchestration` | `program_orchestration.go:263` | 0% | D2-follow: program-orchestration sense path |
| `ResumeProgramOrchestration` | `program_orchestration.go:492` | 0% | D2-follow: program resume/recovery |
| `ReleaseProgramChildLease` | `program_orchestration.go:721` | 0% | D2-follow: program child lease lifecycle |
| `ensureProgramChildSession` | `program_orchestration.go:926` | 44% | D2-follow: program child session creation |
| `ActiveOrchestrationSessionForSpec` | `orchestration_engine.go:473` | 0% | D2-follow: active-session lookup |
| `OrchestrationPreflight` | `orchestration_preflight.go:29` | 0% | D2-follow: preflight validation |
| `ValidateAgentsMD` | `agents.go:88` | 0% | D2-follow: AGENTS.md structural validation |
| `InjectPrompt` | `authoring.go:89` | 0% | D2-follow: prompt injection authoring path |
| `LoadContextManifest` / `Present` | `manifest_tools.go` | 0% | covered indirectly by mcp filter tests; add a direct core unit test |
| `ResolveTaskView` | `taskview.go:23` | 0% | D2-follow: task-view resolution |
| `BuildPRSummary` / `Markdown` | `prsummary.go` | 0% | covered by `cmd` `TestPRSummary`; add a core-level unit for the markdown builder |

## Won't test — with reason

| Function(s) | File | Reason |
|-------------|------|--------|
| `RenderHelp`, `RenderCommandHelp`, `RenderHelpJSON` | `help.go` | print-formatting glue; asserting exact help text is golden-churn the spec forbids (§6.3, §8). Behavior is exercised end-to-end via `cmd` help tests. |
| `Info`, `Success`, `Error`, `Header`, `Divider`, `toUpper`, `IsJSONMode` | `ui.go` | terminal-presentation glue (§8 "not chasing 100% on print-formatting glue"). |
| `PrintJSON` | `output.go` | thin `json.Encoder` wrapper over stdout; covered transitively wherever a `--json` command is asserted. |
| `SteeringDir`, `RolesDir`, `SkillsDir`, `SpecsDir`, `IntegrationsPath`, `AgentsPath`, `RuntimeDir`, `ArtifactsDir`, `ProgramSessionsDir` | `paths.go`, `runtime_paths.go` | one-line `filepath.Join` path builders; no branching logic to guard. Exercised implicitly by every test that reads these locations. |
| `FindSpecdRoot`, `RequireSpecdRoot` | `paths.go` | the happy path is covered via the harness; the not-found branch needs a cwd outside any project, which the hermetic harness (always chdir'd into a root) cannot express without a fault-injection seam. Owner: add when a root-discovery seam exists. |
| `runIsolated`, sandbox `Name`/`Run` | `runner_sandbox.go` | platform-specific process isolation (namespaces/seccomp); needs privileged, non-hermetic execution — out of scope for the default suite (§4 stress/external). |
| `registerOptionalBackend` | `backend.go` | only reachable under the `specd_postgres`/`specd_redis` build tags (§4 external-backend); tested in those tagged jobs, not the default suite. |
| `StripHTMLComments` (14%), `ExtractSection` (10%), `sectionRE`, `GetBadge` | `md.go`, `report.go` | markdown-rendering helpers; remaining branches are formatting variants. Low integrity risk; deprioritized below the logic gaps above. |

## Notes

- `internal/mcp` (88.8%) and `internal/testharness` (80.8%) are above their §5
  floors; their fakes (`testharness/orchestration.go`, `pinky.go`) are now
  driven directly by `testharness`'s own happy-path tests in addition to the
  integration suites.
- The "D2-follow" items raise core from the 70% floor toward the 95% long-term
  target; they are intentionally out of the floor-meeting scope but enumerated
  here so the gap is mapped, not silent.
