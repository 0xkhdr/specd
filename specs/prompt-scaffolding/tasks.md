# tasks.md — One-shot Scaffolding execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Derive constraints from gates

- [x] **T1 — Map gate constraints to a single source** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R2,R5
  - Locate where the EARS forms, the 7 design headers, and the 7 task keys are
    defined (`ears.go`, `gates.go`, `tasksparser.go`). Report the exported
    symbols a brief generator can read instead of re-listing strings.
  - verify: N/A — complete with `--unverified --evidence "<symbol map>"`
  - **Evidence:** EARS forms — exported `EarsPattern` consts
    `internal/core/ears.go:7-13` (Unwanted/EventDriven/StateDriven/
    OptionalFeature/Ubiquitous) + `MatchEars` `ears.go:26`; the `earsPatterns`
    regex table `ears.go:15-24` is unexported, so a brief generator must call
    `MatchEars` or read the const names, not the regexes. 7 design headers —
    `DesignSections []string` (exported) `internal/core/phases.go:9-12`. 7 task
    keys — `MandatoryKeys` `internal/core/tasksparser.go:12` + `KeyOrder`
    `tasksparser.go:13`; roles `ValidRoles` `tasksparser.go:14`,
    `ReadonlyRoles` `tasksparser.go:15` (+ `IsValidRole`/`IsReadonlyRole`
    `tasksparser.go:22-26`). All three constraint sets are already exported
    package vars — the brief generator reads these symbols, never re-lists
    literal strings.

## Wave 2 — Persist prompt + brief generator

- [x] **T2 — Persist `--from` prompt into the spec** ✓ complete · 2026-06-16
  - role: builder · depends: — · requirements: R1,R6
  - Add optional `Prompt` to `state.json` and inject it into the
    `requirements.md` stub. `--from` omitted ⇒ unchanged behavior.
  - verify: `go test ./internal/cmd/ -run TestNewFrom -race -count=1`
  - **Evidence:** `State.Prompt` (omitempty) `state.go` + schema mirror; `new`
    reads `--from`, sets `state.Prompt`, and `core.InjectPrompt` inserts an
    `## Originating prompt` block before `## Requirement 1`. Empty `--from` is
    byte-identical to plain `new` (asserted). `TestNewFrom` passes.

- [x] **T3 — `authoring.go` gate-shaped brief generator** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R2,R3,R4
  - Pure function returning per-artifact gate constraints, sourced from the gate
    definitions (no duplicated strings). Text + `SPECD_JSON=1` JSON output. No
    network/LLM.
  - verify: `go test ./internal/core/ -run TestAuthoringBrief -race -count=2`
  - **Evidence:** `NewAuthoringBrief(prompt)` in `internal/core/authoring.go`
    returns `AuthoringBrief` sourced from `EarsForms()`/`DesignSections`/
    `MandatoryKeys`/`ValidRoles` (no literal re-lists); `.Text()` + json tags for
    `SPECD_JSON`. `EarsForms()` added to `ears.go` as single source.
    `TestAuthoringBrief` + `TestAuthoringSync` pass `-race -count=2`.

## Wave 3 — Wire & validate

- [x] **T4 — Wire `--from` into `new` to emit the brief** ✓ complete · 2026-06-16
  - role: builder · depends: T2,T3 · requirements: R1,R3
  - verify: `go test ./internal/cmd/ -run TestNewFrom -race -count=1`
  - **Evidence:** `new --from` persists the prompt into state and injects an
    "## Originating prompt" section before "## Requirement 1"; empty `--from` is
    byte-identical to plain `new`. `TestNewFrom` passes.

- [x] **T5 — Test: brief stays in sync with real gates** ✓ complete · 2026-06-16
  - role: verifier · depends: T3 · requirements: R2,R5
  - Assert the brief's EARS forms / design headers / task keys equal the values
    the gates enforce (fails if a gate changes but the brief does not).
  - verify: `go test ./internal/core/ -run TestAuthoringSync -race -count=2`
  - **Evidence:** `TestAuthoringSync` asserts the generated brief's EARS forms,
    design headers, and task keys equal the values the gates enforce — drift in a
    gate without a brief update fails the test. Passes `-race`.

- [x] **T6 — Test: faithful draft passes `specd check`** ✓ complete · 2026-06-16
  - role: verifier · depends: T4 · requirements: R5
  - Build a draft per the brief, run the full gate pipeline, assert pass.
  - verify: `go test ./internal/cmd/ -run TestFaithfulDraftPassesCheck -race -count=1`
  - **Evidence:** `cmd/faithful_test.go` — `TestFaithfulDraftPassesCheck` authors
    a brief-shaped draft (EARS criteria, full design headers, 7-key tasks) and
    asserts `specd check` passes (ExitOK).

- [x] **T7 — Review: no LLM/network leaked into the binary** ✓ complete · 2026-06-16
  - role: reviewer · depends: T6 · requirements: R4
  - verify: N/A — complete with `--unverified --evidence "<grep: no net/exec to LLM>"`
  - **Evidence:** Reviewed: the brief generator (`authoring.go`) derives entirely
    from in-binary gate constants; no `net/http`, no exec to an LLM. The binary
    remains stdlib-only with zero go.mod deps.

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R2, R5 |
| 2 | T2–T3 | R1–R4, R6 |
| 3 | T4–T7 | R1, R3, R4, R5 |
