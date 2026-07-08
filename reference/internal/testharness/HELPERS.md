# Test-helper inventory

Audit of locally-defined `func`s living in `_test.go` files, with their current
home and the target home under the consolidation rule (spec §3.2):

- **Generic, no specd/package types** → exported `testharness` helper.
- **Needs an unexported package type** → one `helpers_test.go` in that package,
  shared by every `_test.go` in it.

| Helper | Signature | Current file | Target home | Rationale |
|--------|-----------|--------------|-------------|-----------|
| `captureStderr` | `(t, fn func()) string` | `mcp/wave4_test.go` | `testharness.CaptureStderr` | pure os.Stderr redirect, no specd types |
| `ids` | `([]DagTask) []string` | `core/dag_more_test.go`, `core/frontier_test.go` | `core/helpers_test.go` | typed to unexported-package `DagTask` |
| `names` | `([]toolDef) []string` | `mcp/wave4_test.go` | `mcp/helpers_test.go` | typed to package `toolDef` |
| `td` | `(name string) toolDef` | `mcp/wave4_test.go` | `mcp/helpers_test.go` | constructs package `toolDef` |
| `mkState` | `(spec string, rev int, …TaskState) *State` | `core/frontier_test.go` | `core/helpers_test.go` | constructs package `State` |
| `newTestACPStore` | `(t) *acp store` | `core/acp_store_test.go` | `core/helpers_test.go` | constructs ACP store internals |
| `newPinkyHarness` | `(t) …` | `integration/pinky_test.go` | stays in `integration` (single consumer) | package-local, one user |
| `newOrchestrationMCPClient` | `(t) …` | `mcp/orchestration_integration_test.go` | `mcp/helpers_test.go` | shared MCP client builder |

## Notes

- No helper is *literally* copy-pasted across files today; each has a single
  definition. The drift the spec targets is **placement** — generic helpers
  buried in a unit file, package-shared helpers wired to whichever file happened
  to define them first.
- `ids`/`names` are *not* promoted to `testharness` despite the spec's example
  naming (`testharness.IDs`): both are typed to unexported package types
  (`DagTask`, `toolDef`) and cannot live outside their package without leaking
  those types. They land in the package `helpers_test.go` per §3.2.
- Only `captureStderr` is genuinely type-free and moves to
  `testharness.CaptureStderr`.
