# Domain: Spec Lifecycle & State Model

## 1. Purpose & value mapping
- **Principles served:** P2 (Specs as the Source of Truth), P6 (Human Gates at Phase
  Boundaries). Underpins P3 and P7.
- **Paper concept realized:** *instructions/context* + the "Configuring the Harness"
  phase (pp.28–29). The plan lives as versioned Markdown on disk and machine truth
  lives in `state.json`; the harness never holds the plan in a context window.
- **Core use case:** a spec moves forward through Requirements → Design → Tasks →
  Executing → Verifying → Complete, and every semantic transition is an explicit,
  human-approvable event with an on-disk record. This is the paper's insistence that
  the last 20% (correctness, architecture) stays under human control.
- **If none → CUT:** N/A — this is the spine.

## 2. Current-state analysis (from specd)
- **Reference files read:** `internal/core/state.go`, `internal/core/phases.go`,
  `internal/core/task_complete.go`, `internal/cmd/new.go`, `internal/cmd/approve.go`,
  `internal/cmd/status.go`, `docs/concepts.md`, `docs/validation-gates.md`.
- **What exists today; key contracts/invariants:**
  - `state.json` is the single machine truth (`LoadState`/`SaveState`), with
    `SchemaVersion = 6`. **CAS on `Revision`**: `SaveState` reads the on-disk revision,
    refuses to write if it differs, bumps, then `AtomicWrite`s. It **must** run inside
    `WithSpecLock` — test builds panic via `assertLocked`/`lockHeldBy` if not.
  - `LoadState` fails loud on corrupt / newer-schema / invalid-status rather than
    coercing (`GateError`). `Clock` is injectable for determinism.
  - The **phase ratchet** is `phases.go`: `PlanningAdvance` is a single forward-only
    status/phase map; `PhaseForStatus` derives phase; `PhaseReadiness(status, …)`
    dispatches to `LintEars` / `DesignGate` / DAG checks by status (pure functions).
  - `approve.go` (237 LOC) performs the three approval transitions under lock;
    `task_complete.go` holds `CompleteTask` / `ValidateTaskCompletion` /
    `DeriveSpecStatus` — the evidence-integrity path.
  - Execution mode is already a concept: `ModeSimple / ModeOrchestrated / ModeConductor`
    with `State.EffectiveMode`.
- **Redundancy / complexity / drift found (evidence):**
  - `state.go` is ~20K and `State` carries record structs for *every* flywheel feature
    (`SecurityScan`, `ReviewRecord`, `DeployRecord`, `DeployApproval`, `IngestRecord`,
    `EvalSummary`, `RoutingStamp`, `ConductorSession`, `EscalationRecord`). Deferring
    the flywheel (domain 12) should remove most of these from the core schema.
  - Three modes exist but the naming collides with the paper's *conductor/orchestrator*
    axis confusingly (`ModeConductor` is a rejection-clustering analytics mode here, not
    the paper's real-time IDE mode).

## 3. Fresh-start decision
- **Verdict per capability:**
  - `state.json` as machine truth + CAS + atomic write + lock discipline — **KEEP**
    (hard invariants; `docs/contributor-guide.md` code-style invariants #2–#4).
  - Forward-only phase ratchet (`PlanningAdvance`) — **KEEP** (P6).
  - Pure readiness gates (`PhaseReadiness`) — **KEEP** (feeds domain 03).
  - Fat `State` with per-flywheel record structs — **SIMPLIFY**: strip to core fields;
    optional records move behind a `records` extension map so the schema does not bloat
    when the flywheel returns as plugins.
  - Execution mode — **REDESIGN**: make `mode` a clean first-class enum with exactly two
    real states aligned to the paper — `simple` (conductor: human-in-the-loop, no worker
    delegation) and `orchestrated` (orchestrator: async delegation). Drop the
    analytics-flavored `conductor` mode (that analytics is deferred with `conductor` the
    command, domain 12/09).
- **Minimal accurate surface:**
  - Commands: `new`, `approve`, `status`, `midreq`, `decision` (all Tier 1).
  - Modules: `state.go` (thin), `phases.go`, `task_complete.go`, `lock.go`, `io.go`.
  - On-disk: `.specd/specs/<slug>/{requirements.md,design.md,tasks.md,state.json,.lock}`.
- **Architecture & flexibility improvements:**
  - **Records extension map:** `State.Records map[string]json.RawMessage` so a plugin
    (eval/review/security) can attach evidence without a core schema change — the schema
    stops being a dumping ground.
  - **Explicit `mode` field** at spec creation (`new <slug> --mode simple|orchestrated`),
    default `simple`, so the conductor/orchestrator posture is chosen deliberately and
    the same binary serves both.
  - Keep `SchemaVersion` but reset to `1` for the fresh tree (no migration burden —
    `migrate` is CUT, `00-decisions.md`).

## 4. Requirements (EARS-shaped) — seed for requirements.md
1. When a spec is created, the system shall write `state.json` with `schemaVersion: 1`,
   `revision: 0`, `mode: "simple"`, and status `requirements`.
2. When any command mutates spec state, the system shall hold the reentrant per-spec
   lock for the whole read-modify-write and shall persist via compare-and-swap on
   `revision`, refusing the write if the on-disk revision changed.
3. When a persisted write is attempted, the system shall write atomically
   (temp file → fsync → rename) so a partial write can never replace the target.
4. When `approve` is invoked on a spec whose current phase readiness gates do not pass,
   the system shall refuse the transition and report the failing gates.
5. The system shall advance status only along the forward-only `PlanningAdvance` map and
   shall reject any backward or skipping transition.
6. When `state.json` is corrupt, newer-schema, or has an invalid status, the system shall
   fail loud with a gate error rather than coerce it.
7. When a plugin attaches evidence, the system shall store it under `state.records[<key>]`
   without altering core fields.

## 5. Design notes — seed for design.md
- **Module boundaries:** `state.go` (types + CAS/atomic persistence), `phases.go` (pure
  ratchet + readiness), `task_complete.go` (evidence-integrity completion),
  `lock.go`/`io.go` (primitives, domain 10).
- **Key types:** `State{ SchemaVersion, Revision, Mode, Status, Phase, Tasks[],
  Decisions[], MidReqGates[], Records map[string]RawMessage }`; `TaskState`;
  `VerificationRecord` (thin, domain 05).
- **Data/on-disk contracts:** `state.json` (0644), forward-only status, `revision`
  monotonic, injectable `Clock`.
- **Invariants to preserve:** CAS on revision; must-be-locked; atomic write; loud-load;
  forward-only ratchet.
- **External interfaces:** `LoadState`/`SaveState`; `PhaseReadiness`; `CompleteTask`.

## 6. Proposed task DAG — seed for tasks.md

### Wave 1 — state core
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T2.1 | craftsman | `internal/core/state.go` | — | `go test ./internal/core -run TestStateCAS` | CAS refuses on stale revision; monotonic bump |
| T2.2 | craftsman | `internal/core/io.go` | — | `go test ./internal/core -run TestAtomicWrite` | temp→fsync→rename; partial write never replaces |
| T2.3 | craftsman | `internal/core/phases.go` | T2.1 | `go test ./internal/core -run TestPhaseRatchet` | forward-only; backward rejected |
### Wave 2 — lifecycle commands
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T2.4 | craftsman | `internal/cmd/new.go` | T2.1 | `go run . new demo && test -f .specd/specs/demo/state.json` | new spec at rev 0, mode simple |
| T2.5 | craftsman | `internal/cmd/approve.go`, `internal/core/task_complete.go` | T2.3 | `go test ./internal/cmd -run TestApproveGates` | approve blocked when readiness fails |
| T2.6 | craftsman | `internal/cmd/status.go` | T2.1 | `go run . status demo --json | grep -q '"mode":"simple"'` | status is a pure state projection |
| T2.7 | validator | `internal/core/state_lock_test.go` | T2.1 | `go test ./internal/core -run TestSaveStateRequiresLock` | unlocked SaveState panics in test build |

## 7. Risks, open questions, cross-domain dependencies
- **Risk:** the `Records` extension map becomes an untyped junk drawer. Mitigation: each
  plugin owns a documented schema for its key; core validates only that it is valid JSON.
- **Open question:** should `mode` be mutable after creation (simple→orchestrated
  mid-flight)? Proposed: yes, but only via an explicit `approve --mode` transition so it
  is an auditable event.
- **Cross-domain deps:** domain 03 consumes `PhaseReadiness`; domain 05 writes
  `VerificationRecord`; domain 09 reads `mode` to decide whether the orchestration tier
  is even eligible; domain 10 provides `lock`/`io`/CAS primitives.
