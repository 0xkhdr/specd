# specd — Go Migration Plan

> **Context for fresh sessions:** This document contains the complete analysis of the TypeScript/Node.js
> `specd` codebase and a step-by-step plan to rewrite it in Go. Read this before touching any code.

---

## 1. What specd Is

`specd` is a spec-driven coding harness CLI — agent-agnostic, zero-runtime-dependency, single binary.
It enforces a structured workflow: requirements → design → tasks → executing → verifying → complete.
All state lives in plain files (JSON + Markdown) under `.specd/` in the project root.

**Philosophy preserved in Go:**
- Zero runtime dependencies (stdlib only)
- Atomic file writes (write-to-tmp + fsync + rename)
- Optimistic concurrency (CAS on `revision` field in `state.json`)
- Per-spec advisory file locks (O_EXCL lockfile, reentrant within process)
- 100% round-trip parser stability for `tasks.md`
- Strict exit code contract: 0/1/2/3

---

## 2. Full Inventory of TypeScript Source

### Entry Point
| File | Role |
|------|------|
| `src/cli.ts` | Arg parsing, command dispatch, top-level error → exit-code conversion |

### Commands (`src/commands/`) — 17 total
| Command | File | What it does |
|---------|------|-------------|
| `init` | `init.ts` | Scaffold `.specd/` + `AGENTS.md`, idempotent, `--force` to overwrite |
| `new` | `new.ts` | Create spec dir with 6 artifact stubs + `state.json` |
| `status` | `status.ts` | Show board: all specs or single-spec detail, `--json` |
| `context` | `context.ts` | Phase-scoped minimal briefing (what to load + next action) |
| `check` | `check.ts` | Run all 7 validation gates, emit violations, `--json` |
| `next` | `next.ts` | Next runnable task (or `--all` for frontier), `--json` |
| `dispatch` | `dispatch.ts` | Ready-to-run packets for entire runnable frontier, `--json` |
| `task` | `task.ts` | Evidence-gated status flip for a task (complete/blocked/running/pending) |
| `verify` | `verify.ts` | Run task's `verify:` shell command and record `VerificationRecord`; OR record per-criterion acceptance proof |
| `approve` | `approve.ts` | Clear awaiting-approval gate OR advance planning ratchet |
| `decision` | `decision.ts` | Append ADR to `decisions.md` |
| `midreq` | `midreq.ts` | Log mid-flight requirement change; sets gate on high/critical impact |
| `memory` | `memory.ts` | Add/promote learning items in `memory.md` |
| `report` | `report.ts` | Generate Markdown or HTML progress report |
| `waves` | `waves.ts` | Render task wave DAG (ASCII or `--json`) |
| `program` | `program.ts` | Cross-spec DAG: status, link, unlink |
| `update` | `update.ts` | Self-update via GitHub releases/git pull + rebuild |

### Core Modules (`src/core/`)
| File | Role |
|------|------|
| `exit.ts` | Exit codes (0/1/2/3) + `SpecdError` carrying a code |
| `io.ts` | `atomicWrite`, `readOrNull`, `readOrDefault`, `appendFile` |
| `lock.ts` | `withSpecLock` — O_EXCL advisory lock, reentrant, stale-reclaim, timeout |
| `paths.ts` | `findSpecdRoot`, `requireSpecdRoot`, all canonical path helpers |
| `state.ts` | `State` shape, schema migration (v0→v4), `loadState`, `saveState` (CAS) |
| `specFiles.ts` | `loadSpec`, `reconcile`, `loadConfig`, `readArtifact`, `listSpecs`, `readRole`, `ARTIFACTS` |
| `tasksParser.ts` | Line parser + canonical serializer for `tasks.md`; `applyTaskAnnotation` (surgical rewrite) |
| `dag.ts` | `nextRunnable`, `runnableFrontier`, `detectCycle`, `orphanDeps`, `waveViolations`, `groupWaves`, `criticalPath` |
| `ears.ts` | EARS grammar linter for `requirements.md` |
| `phases.ts` | `phaseForStatus`, `PLANNING_ADVANCE`, `phaseReadiness`, `designGate` |
| `render.ts` | `waveGraph`, `nextSummary`, `counts`, `blockerLines`, `latestMidreq`, `requirementNumbers`, `acceptanceGaps`, `uncoveredRequirements` |
| `program.ts` | Cross-spec DAG: `buildProgram`, `loadProgram`, `saveProgram` |
| `report.ts` | HTML/Markdown report generation |
| `templates.ts` | `readTemplate`, `applyVars` (`{{VAR}}` substitution) |
| `md.ts` | `stripHtmlComments` |
| `help.ts` | `renderHelp`, `renderCommandHelp`, `renderHelpJson` |
| `commands.ts` | Command registry metadata (name, flags, examples, exit codes) |
| `ui.ts` | Colored output (`info`, `success`, `warn`, `error`, `step`, `header`, `divider`), JSON mode |
| `output.ts` | Redirectable stdout sink used by tests |

### Templates (`src/templates/`)
- `AGENTS.md` — default agents steering file
- `config.json` — default config
- `steering/` — 6 steering markdown files
- `roles/` — 4 role prompts (investigator, builder, reviewer, verifier)
- `specStubs/` — 6 artifact stub templates with `{{VAR}}` placeholders

### Tests (`test/`) — 92 tests
- Unit: `dag.test.ts`, `ears.test.ts`, `tasksParser.test.ts`, `core.test.ts`, `ui.test.ts`
- Command: `check.test.ts`, `task.test.ts`, `verify.test.ts`, `approve.test.ts`, `context.test.ts`, `dispatch.test.ts`, `report.test.ts`, `g5.test.ts`, `program.test.ts`
- Concurrency: `concurrency.test.ts`, `hardening.test.ts`
- E2E: `e2e/full-spec.test.ts`

---

## 3. Data Structures (Go types)

### `state.json` — `State` struct (schema v4)
```go
type SpecStatus string
const (
    StatusRequirements SpecStatus = "requirements"
    StatusDesign       SpecStatus = "design"
    StatusTasks        SpecStatus = "tasks"
    StatusExecuting    SpecStatus = "executing"
    StatusVerifying    SpecStatus = "verifying"
    StatusComplete     SpecStatus = "complete"
    StatusBlocked      SpecStatus = "blocked"
)

type Phase string
const (
    PhasePerceive Phase = "perceive"
    PhaseAnalyze  Phase = "analyze"
    PhasePlan     Phase = "plan"
    PhaseExecute  Phase = "execute"
    PhaseVerify   Phase = "verify"
    PhaseReflect  Phase = "reflect"
)

type Gate string
const (
    GateNone             Gate = "none"
    GateAwaitingApproval Gate = "awaiting-approval"
)

type TaskStatus string
const (
    TaskPending  TaskStatus = "pending"
    TaskRunning  TaskStatus = "running"
    TaskComplete TaskStatus = "complete"
    TaskBlocked  TaskStatus = "blocked"
)

type VerificationRecord struct {
    Command     string `json:"command"`
    ExitCode    int    `json:"exitCode"`
    Verified    bool   `json:"verified"`
    TimedOut    bool   `json:"timedOut"`
    StdoutTail  string `json:"stdoutTail"`
    StderrTail  string `json:"stderrTail"`
    DurationMs  int64  `json:"durationMs"`
    RanAt       string `json:"ranAt"`
    GitHead     string `json:"gitHead,omitempty"`
}

type CriterionRecord struct {
    Requirement int    `json:"requirement"`
    Criterion   int    `json:"criterion"`
    Status      string `json:"status"` // "pass" | "fail"
    Evidence    string `json:"evidence"`
    RanAt       string `json:"ranAt"`
}

type TaskState struct {
    ID          string              `json:"id"`
    Title       string              `json:"title"`
    Role        string              `json:"role"`
    Wave        int                 `json:"wave"`
    Depends     []string            `json:"depends"`
    Requirements []int             `json:"requirements"`
    Status      TaskStatus          `json:"status"`
    StartedAt   *string             `json:"startedAt,omitempty"`
    FinishedAt  *string             `json:"finishedAt,omitempty"`
    Evidence    *string             `json:"evidence,omitempty"`
    Verification *VerificationRecord `json:"verification,omitempty"`
    Blocker     *string             `json:"blocker,omitempty"`
}

type Blocker struct {
    Task   string `json:"task"`
    Reason string `json:"reason"`
    Since  string `json:"since"`
}

type State struct {
    SchemaVersion int                        `json:"schemaVersion"`
    Revision      int                        `json:"revision"`
    Spec          string                     `json:"spec"`
    Title         string                     `json:"title"`
    Status        SpecStatus                 `json:"status"`
    Phase         Phase                      `json:"phase"`
    Gate          Gate                       `json:"gate"`
    Turn          int                        `json:"turn"`
    CreatedAt     string                     `json:"createdAt"`
    UpdatedAt     string                     `json:"updatedAt"`
    Tasks         map[string]TaskState       `json:"tasks"`
    Blockers      []Blocker                  `json:"blockers"`
    Acceptance    map[string]CriterionRecord `json:"acceptance,omitempty"`
}
```

### `config.json` — `Config` struct
```go
type Config struct {
    Version           int         `json:"version"`
    DefaultVerify     string      `json:"defaultVerify"`
    Report            ReportCfg   `json:"report"`
    Roles             RolesCfg    `json:"roles"`
    PromotionThreshold int        `json:"promotionThreshold"`
    Gates             GatesCfg    `json:"gates"`
}

type ReportCfg struct {
    Format             string `json:"format"` // "md" | "html"
    AutoRefreshSeconds int    `json:"autoRefreshSeconds"`
}

type RolesCfg struct {
    SubagentMode string `json:"subagentMode"` // "inline" | "delegate"
}

type GatesCfg struct {
    Traceability string `json:"traceability"` // "warn" | "error"
    Acceptance   string `json:"acceptance"`   // "off" | "required"
}
```

---

## 4. Go Project Structure

```
specd/                          ← repo root
├── main.go                     ← os.Exit(run(os.Args[1:]))
├── go.mod                      ← module github.com/0xkhdr/specd; go 1.22; no external deps
├── go.sum                      ← empty (no deps)
│
├── internal/
│   ├── cli/
│   │   ├── args.go             ← parseArgs, Args struct, BOOLEAN_FLAGS
│   │   └── dispatch.go         ← command dispatch switch
│   │
│   ├── core/
│   │   ├── exit.go             ← ExitCode consts (0/1/2/3), SpecdError
│   │   ├── io.go               ← AtomicWrite, ReadOrNull, ReadOrDefault, AppendFile
│   │   ├── lock.go             ← WithSpecLock, reentrant, O_EXCL, stale-reclaim
│   │   ├── paths.go            ← FindSpecdRoot, RequireSpecdRoot, path helpers
│   │   ├── state.go            ← State types, LoadState, SaveState, migrate, NowISO
│   │   ├── specfiles.go        ← LoadSpec, Reconcile, LoadConfig, ReadArtifact, ListSpecs
│   │   ├── tasksparser.go      ← ParseTasks, SerializeTasks, ApplyTaskAnnotation, etc.
│   │   ├── dag.go              ← NextRunnable, RunnableFrontier, DetectCycle, etc.
│   │   ├── ears.go             ← LintEars, MatchEars
│   │   ├── phases.go           ← PhaseForStatus, PlanningAdvance, PhaseReadiness, DesignGate
│   │   ├── render.go           ← WaveGraph, NextSummary, Counts, BlockerLines, etc.
│   │   ├── program.go          ← BuildProgram, LoadProgram, SaveProgram
│   │   ├── report.go           ← GenerateReport (md/html)
│   │   ├── templates.go        ← ReadTemplate, ApplyVars (embed via go:embed)
│   │   ├── md.go               ← StripHTMLComments
│   │   ├── help.go             ← RenderHelp, RenderCommandHelp, RenderHelpJSON
│   │   ├── commands.go         ← CommandMetadata slice (registry)
│   │   └── ui.go               ← Info, Success, Warn, Error, Step, Header, Divider, IsJSONMode
│   │
│   └── cmd/
│       ├── init.go
│       ├── new.go
│       ├── status.go
│       ├── context.go
│       ├── check.go
│       ├── next.go
│       ├── dispatch.go
│       ├── task.go
│       ├── verify.go
│       ├── approve.go
│       ├── decision.go
│       ├── midreq.go
│       ├── memory.go
│       ├── report.go
│       ├── waves.go
│       ├── program.go
│       └── update.go
│
├── templates/                  ← identical to current src/templates/
│   ├── AGENTS.md
│   ├── config.json
│   ├── steering/
│   ├── roles/
│   └── specStubs/
│
└── internal/core/embed.go      ← //go:embed ../../../templates/**/* var TemplatesFS embed.FS
```

**Key Go packaging rules:**
- `internal/` — not importable outside the module (enforces encapsulation)
- All packages under `internal/core/` are `package core`; `internal/cmd/` is `package cmd`
- `main.go` imports `internal/cli` only

---

## 5. Critical Invariants and Go Translations

### 5.1 Exit Code Contract
```go
// internal/core/exit.go
const (
    ExitOK       = 0
    ExitGate     = 1
    ExitUsage    = 2
    ExitNotFound = 3
)

type SpecdError struct {
    Code    int
    Message string
}
func (e *SpecdError) Error() string { return e.Message }
func GateError(msg string) *SpecdError     { return &SpecdError{Code: ExitGate, Message: msg} }
func UsageError(msg string) *SpecdError    { return &SpecdError{Code: ExitUsage, Message: msg} }
func NotFoundError(msg string) *SpecdError { return &SpecdError{Code: ExitNotFound, Message: msg} }
```

### 5.2 Atomic Write
```go
// internal/core/io.go
func AtomicWrite(path, data string) error {
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0o755); err != nil { return err }
    f, err := os.CreateTemp(dir, fmt.Sprintf(".%d.*.tmp", os.Getpid()))
    if err != nil { return err }
    name := f.Name()
    defer func() {
        f.Close()
        os.Remove(name) // no-op if rename succeeded
    }()
    if _, err := f.WriteString(data); err != nil { return err }
    if err := f.Sync(); err != nil { return err }   // fsync — durability invariant
    if err := f.Close(); err != nil { return err }
    return os.Rename(name, path)  // atomic on POSIX
}
```

### 5.3 Advisory Lock
```go
// internal/core/lock.go
// Use sync.Map for per-process reentrancy depth keyed by lockfile path.
// O_EXCL create via os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
// Write "PID TIMESTAMP\n" to lockfile.
// Stale detection: parse timestamp, compare to time.Now().
// Retry loop: time.Sleep(25ms) up to timeout (default 5s).
// Always defer os.Remove(lockPath).

var lockDepths sync.Map // map[string]int

func WithSpecLock[T any](root, slug string, fn func() (T, error)) (T, error) {
    path := lockPath(root, slug)
    if depth, ok := lockDepths.Load(path); ok {
        lockDepths.Store(path, depth.(int)+1)
        defer func() {
            d := lockDepths.Load // ...decrement
        }()
        return fn()
    }
    // acquire, defer release, call fn
}
```

> **Note:** Go generics (1.18+) make `WithSpecLock[T]` clean. Use `go 1.22` in `go.mod`.

### 5.4 Optimistic Concurrency (CAS)
```go
// in SaveState: before write, re-read state.json and check onDisk.Revision == state.Revision
// If mismatch → GateError("state.json for '%s' changed underfoot...")
// On write: state.Revision++, state.UpdatedAt = NowISO()
```

### 5.5 Template Embedding
```go
// internal/core/embed.go
//go:embed ../../../templates
var TemplatesFS embed.FS

func ReadTemplate(rel string) (string, error) {
    b, err := TemplatesFS.ReadFile("templates/" + rel)
    return string(b), err
}
```
This replaces `templatesDir()` + `readFileSync`. Templates ship inside the binary — **no external files needed at runtime**.

### 5.6 Tasks Parser Round-Trip Stability
Same line-by-line logic as the TypeScript version.
- Regex constants declared as `var` at package level (compiled once)
- `SerializeTasks` must produce identical bytes for a canonical document
- Test: `parse → serialize → parse → serialize`, assert bytes equal

### 5.7 Sleep in Lock (no Atomics.wait)
```go
// TypeScript used Atomics.wait for blocking sleep. Go: time.Sleep(25 * time.Millisecond)
```

### 5.8 Shell Command Execution (verify command)
```go
// internal/cmd/verify.go
cmd := exec.CommandContext(ctx, "sh", "-c", command)
cmd.Dir = root
// Capture stdout/stderr with bytes.Buffer
// ctx with timeout: context.WithTimeout(context.Background(), timeoutMs)
// On timeout: exitCode = 124 (like unix `timeout` utility)
```

### 5.9 Version Embedding
```go
// main.go or internal/core/version.go
var version = "dev" // overridden at build time:
// go build -ldflags "-X main.version=v0.2.0" ...
```

---

## 6. EARS Linter (Go)

Direct port of the TypeScript regex patterns:
```go
var earsPatterns = []struct {
    name string
    re   *regexp.Regexp
}{
    {"unwanted",        regexp.MustCompile(`(?i)^IF .+ THEN THE SYSTEM SHALL .+`)},
    {"event-driven",    regexp.MustCompile(`(?i)^WHEN .+ THE SYSTEM SHALL .+`)},
    {"state-driven",    regexp.MustCompile(`(?i)^WHILE .+ THE SYSTEM SHALL .+`)},
    {"optional-feature",regexp.MustCompile(`(?i)^WHERE .+ THE SYSTEM SHALL .+`)},
    {"ubiquitous",      regexp.MustCompile(`(?i)^THE SYSTEM SHALL .+`)},
}
```

---

## 7. DAG Engine (Go)

All pure functions, no goroutines needed (single-threaded CLI).
```go
type DagTask struct {
    ID     string
    Wave   int
    Depends []string
    Status TaskStatus
}

type NextResult struct {
    Kind     string   // "task" | "all-complete" | "all-blocked" | "waiting"
    ID       string   // for "task"
    Blocked  []string // for "all-blocked"
    Blocking []string // for "waiting"
}
```

Cycle detection: DFS with WHITE/GREY/BLACK coloring — identical algorithm.

---

## 8. Color/UI (Go)

```go
// internal/core/ui.go
// Detect NO_COLOR env var (https://no-color.org)
// Check os.Stdout.Fd() with term.IsTerminal if needed for CI detection
// ANSI escape codes identical to TS version

func IsJSONMode() bool {
    return os.Getenv("SPECd_JSON") == "1" || os.Getenv("SPECd_JSON") == "true"
}
```

---

## 9. Go Best Practices to Apply

### Module
```
module github.com/0xkhdr/specd
go 1.22
// No require block — zero external dependencies
```

### Error Handling
- All functions return `(value, error)` pairs
- Wrap SpecdError as sentinel: `errors.As(err, &specdErr)`
- In `main.go`: `if err != nil { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(exitCode) }`

### No `panic` in Library Code
- Library functions return errors, never panic
- Only `main.go` calls `os.Exit`

### Avoid `init()` Functions
- Use package-level `var` for compiled regexes

### Locking with Generics
```go
func WithSpecLock[T any](root, slug string, fn func() (T, error)) (T, error)
```

### JSON Marshaling
- Use `encoding/json` stdlib — no external libs
- `json.MarshalIndent(v, "", "  ")` for pretty output (matches TS `JSON.stringify(v, null, 2)`)

---

## 10. Installation Methods (Go Best Practices)

### A. `go install` (primary — works out of the box)
```sh
go install github.com/0xkhdr/specd@latest
```
Places binary in `$GOPATH/bin` (usually `~/go/bin`).
Requires Go installed. **Recommended for Go developers.**

### B. Pre-built Binary (GitHub Releases via GoReleaser)
Add `.goreleaser.yaml`:
```yaml
project_name: specd
builds:
  - main: ./main.go
    env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w -X main.version={{.Version}}
archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
checksum:
  name_template: "checksums.txt"
```
Users then:
```sh
curl -fsSL https://github.com/0xkhdr/specd/releases/latest/download/specd_linux_amd64.tar.gz | tar -xz
sudo mv specd /usr/local/bin/
```

### C. Homebrew (for macOS/Linux)
Create a Homebrew tap formula (`homebrew-tap/Formula/specd.rb`) that downloads the release tarball.
```sh
brew install 0xkhdr/tap/specd
```

### D. Install Script (curl | sh)
```sh
curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/install.sh | sh
```
Script detects OS/arch, downloads the right binary from GitHub Releases, moves to `/usr/local/bin`.

### E. `specd update` Command Rewrite for Go
The TypeScript `update` command rebuilds from source (git pull + npm build).
In Go, `update` should instead:
1. Query GitHub Releases API for latest tag
2. Download the pre-built binary for current OS/arch
3. Replace the running binary (write to temp, rename into place)
4. Verify the new binary is executable

```go
// Detect current binary path: os.Executable()
// Download from: https://github.com/0xkhdr/specd/releases/download/{tag}/specd_{OS}_{ARCH}.tar.gz
// Atomic replace: write to tmp next to binary, os.Rename
```

---

## 11. Testing Strategy (Go)

Use only `testing` stdlib package.

### Unit Tests
```
internal/core/dag_test.go          ← port dag.test.ts
internal/core/ears_test.go         ← port ears.test.ts
internal/core/tasksparser_test.go  ← port tasksParser.test.ts
internal/core/state_test.go        ← schema migration, CAS
internal/core/io_test.go           ← atomic write, crash safety
internal/core/lock_test.go         ← reentrancy, stale-reclaim, timeout
```

### Command Tests (integration against temp dirs)
```
internal/cmd/check_test.go
internal/cmd/task_test.go
internal/cmd/verify_test.go
internal/cmd/approve_test.go
internal/cmd/dispatch_test.go
```

### Concurrency Tests
```go
// Run N goroutines each calling specd task <slug> Tx --status complete
// Assert final state revision = N (no lost updates)
// Uses t.Parallel()
```

### E2E
```
e2e/full_spec_test.go   ← full lifecycle: init → new → check → approve (×3) → task → verify → approve
```

**Pattern for command tests:**
```go
func TestCheckGate(t *testing.T) {
    root := t.TempDir()
    // scaffold .specd/ and spec files programmatically
    // call run function directly (no subprocess) 
    // capture stdout/stderr via strings.Builder
    code := cmd.RunCheck(root, args, &stdout, &stderr)
    if code != 1 { t.Errorf(...) }
}
```

---

## 12. Implementation Order (Step-by-Step)

### Phase 1 — Scaffold + Core IO
1. `go mod init github.com/0xkhdr/specd`
2. `main.go` — minimal: parse args, call dispatch, exit
3. `internal/core/exit.go` — SpecdError, exit codes
4. `internal/core/io.go` — AtomicWrite, ReadOrNull, AppendFile
5. `internal/core/lock.go` — WithSpecLock
6. `internal/core/paths.go` — FindSpecdRoot, all path helpers
7. `internal/core/templates.go` + `embed.go` — embed FS, ReadTemplate, ApplyVars
8. Copy `src/templates/` → `templates/`
9. Tests: io, lock, paths

### Phase 2 — State + Parser
10. `internal/core/state.go` — all types, LoadState, SaveState, migrate, NowISO
11. `internal/core/tasksparser.go` — full parser + serializer + ApplyTaskAnnotation
12. Tests: state migration (v0→v4), tasks parser round-trip (50+ cases)

### Phase 3 — Domain Logic
13. `internal/core/dag.go` — NextRunnable, RunnableFrontier, DetectCycle, CriticalPath
14. `internal/core/ears.go` — LintEars
15. `internal/core/md.go` — StripHTMLComments
16. `internal/core/phases.go` — PhaseForStatus, PhaseReadiness, DesignGate
17. `internal/core/specfiles.go` — LoadSpec, Reconcile, LoadConfig, ListSpecs
18. `internal/core/render.go` — WaveGraph, Counts, NextSummary, etc.
19. `internal/core/program.go` — BuildProgram, LoadProgram, SaveProgram
20. Tests: dag (cycle, frontier, critical path), ears, phases

### Phase 4 — UI + Help
21. `internal/core/ui.go` — colored output, JSON mode, step/header/divider
22. `internal/core/commands.go` — CommandMetadata registry
23. `internal/core/help.go` — RenderHelp, RenderCommandHelp, RenderHelpJSON
24. Tests: ui (JSON mode, color suppression)

### Phase 5 — Commands (lifecycle group)
25. `internal/cmd/init.go` → `specd init`
26. `internal/cmd/new.go` → `specd new`
27. `internal/cmd/approve.go` → `specd approve`
28. `internal/cmd/decision.go` → `specd decision`
29. `internal/cmd/midreq.go` → `specd midreq`
30. `internal/cmd/memory.go` → `specd memory`
31. Tests per command

### Phase 6 — Commands (execution group)
32. `internal/cmd/task.go` → `specd task`
33. `internal/cmd/verify.go` → `specd verify` (run shell cmd + record criterion)
34. `internal/cmd/next.go` → `specd next`
35. `internal/cmd/dispatch.go` → `specd dispatch`
36. Tests: task (evidence gate, CAS, dual write), verify (timeout, git head)

### Phase 7 — Commands (inspection group)
37. `internal/cmd/status.go` → `specd status`
38. `internal/cmd/context.go` → `specd context`
39. `internal/cmd/check.go` → `specd check` (all 7 gates)
40. `internal/cmd/waves.go` → `specd waves`
41. `internal/cmd/program.go` → `specd program`
42. `internal/core/report.go` + `internal/cmd/report.go` → `specd report`
43. Tests: check (all gate failures), context (per-phase brief)

### Phase 8 — Meta + Release
44. `internal/cmd/update.go` → `specd update` (binary self-update)
45. `internal/cli/dispatch.go` — wire all commands into dispatch switch
46. `.goreleaser.yaml` — multi-platform builds
47. `install.sh` — curl-pipe installer
48. E2E test: full lifecycle
49. Concurrency test: parallel task updates

---

## 13. Arg Parsing (Go)

No external flag library. Direct port of the TypeScript hand-rolled parser:
```go
type Args struct {
    Pos   []string
    Flags map[string]string // boolean flags stored as "true"
}

var booleanFlags = map[string]bool{
    "force": true, "json": true, "all": true, "unverified": true,
}

func ParseArgs(argv []string) Args {
    args := Args{Flags: make(map[string]string)}
    for i := 0; i < len(argv); i++ {
        tok := argv[i]
        if strings.HasPrefix(tok, "--") {
            key := tok[2:]
            if booleanFlags[key] {
                args.Flags[key] = "true"
            } else if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "--") {
                i++
                args.Flags[key] = argv[i]
            } else {
                args.Flags[key] = "true"
            }
        } else {
            args.Pos = append(args.Pos, tok)
        }
    }
    return args
}
```

---

## 14. `go.mod` Target
```
module github.com/0xkhdr/specd

go 1.22

// No external dependencies — stdlib only.
// embed, os, io, path/filepath, encoding/json, regexp, fmt, strings,
// sync, time, context, os/exec, syscall (for O_EXCL flock)
```

---

## 15. Key Differences from TypeScript Version

| Aspect | TypeScript | Go |
|--------|-----------|-----|
| Runtime | Node.js ≥18 required | Single static binary, no runtime |
| Templates | Read from `dist/templates/` on disk | Embedded in binary via `embed.FS` |
| Lock sleep | `Atomics.wait` (busy-wait) | `time.Sleep` |
| Generics | No `WithSpecLock<T>` (returns `any`) | `WithSpecLock[T any]` — type-safe |
| Self-update | `git pull` + `npm install` + `npm run build` | Download pre-built binary, atomic replace |
| Version | Read `package.json` at runtime | Embedded via `-ldflags "-X main.version=..."` |
| Async | `async/await` in `update`, `dispatch` | All synchronous (CLI is single-threaded) |
| Concurrency | Single-process, `Atomics.wait` for lock sleep | `sync.Map` for lock depth, `time.Sleep` |
| Testing | `node:test` with `tsx` | `testing` stdlib, `t.TempDir()` |
| Build | `tsc` + copy-templates script | `go build` (templates embedded) |
| Install | `npm install -g specd` | `go install` / pre-built binary |

---

## 16. AGENTS.md / Steering Files

Keep identical to current `src/templates/` — no changes to content.
The Go binary embeds them; `specd init` writes them to disk exactly as before.

---

## 17. Things to NOT Change

- `.specd/` directory structure — identical
- `state.json` schema and field names — backward compatible with existing repos
- `tasks.md` format — byte-for-byte compatible (round-trip invariant)
- All 7 gate names and their semantics
- Exit code contract (0/1/2/3)
- All command names and flag names
- JSON output shapes (other tools may parse them)
- Template content

---

## 18. Quick Reference: File Mapping

| TypeScript | Go |
|-----------|-----|
| `src/cli.ts` | `main.go` + `internal/cli/args.go` + `internal/cli/dispatch.go` |
| `src/core/exit.ts` | `internal/core/exit.go` |
| `src/core/io.ts` | `internal/core/io.go` |
| `src/core/lock.ts` | `internal/core/lock.go` |
| `src/core/paths.ts` | `internal/core/paths.go` |
| `src/core/state.ts` | `internal/core/state.go` |
| `src/core/specFiles.ts` | `internal/core/specfiles.go` |
| `src/core/tasksParser.ts` | `internal/core/tasksparser.go` |
| `src/core/dag.ts` | `internal/core/dag.go` |
| `src/core/ears.ts` | `internal/core/ears.go` |
| `src/core/phases.ts` | `internal/core/phases.go` |
| `src/core/render.ts` | `internal/core/render.go` |
| `src/core/program.ts` | `internal/core/program.go` |
| `src/core/report.ts` | `internal/core/report.go` |
| `src/core/templates.ts` | `internal/core/templates.go` + `internal/core/embed.go` |
| `src/core/md.ts` | `internal/core/md.go` |
| `src/core/help.ts` | `internal/core/help.go` |
| `src/core/commands.ts` | `internal/core/commands.go` |
| `src/core/ui.ts` | `internal/core/ui.go` |
| `src/core/output.ts` | (absorbed into ui.go — Go io.Writer injection for tests) |
| `src/commands/*.ts` | `internal/cmd/*.go` |
| `src/templates/` | `templates/` (embedded via `//go:embed`) |
