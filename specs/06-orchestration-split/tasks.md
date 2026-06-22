# Spec 06 — Tasks (Orchestration Split)

> Prereq: land **after** any W3 specs that touch `internal/core` (Spec 02/03
> event hooks settled) to avoid re-conflict. Pure mechanical move.

---

## Wave A — Plan the cut

### [ ] W3.3a — Map functions to bands
- **Files:** n/a (working note)
- **Do:** List every func/type in `program_orchestration.go` and assign each to
  one band (snapshot / decide / step / session / lease) per `spec.md §1`.
  Identify unexported helpers and pin them to the band of their primary caller.
- **Done when:** Every symbol has exactly one destination file; no symbol
  straddles two bands ambiguously.

---

## Wave B — Move

### [ ] W3.3b — Create band files by relocation
- **Files:** `internal/core/program_snapshot.go`, `program_decide.go`,
  `program_step.go`, `program_session.go`, `program_lease.go` (new);
  `internal/core/program_orchestration.go` (shrink)
- **Do:** Move each band's funcs/types into its file **verbatim** — same
  `package core`, same signatures, same bodies. Keep the `program_` naming.
  Leave only shared top-level types + the subsystem doc comment in (or delete)
  `program_orchestration.go`. Do **not** edit any logic.
- **Done when:** `go build ./...` passes; each new file ≤~400 LOC; no file in
  `internal/core` >~700 LOC.

### [ ] W3.4a — Re-home co-located tests (optional)
- **Files:** `internal/core/program_orchestration_test.go` (if 1:1 mappable)
- **Do:** If a test file maps cleanly to a band, move its tests alongside. Do
  **not** split a test file through the middle of a logical group. Skip if it
  risks churn.
- **Done when:** Tests compile; structure mirrors the new files where clean.

---

## Wave C — Verify no-op

### [ ] W3.4b — Prove pure relocation
- **Files:** n/a (review + CI)
- **Do:** Review `git diff` (use `--find-renames`) and confirm it reads as
  moves, not logic edits — zero changed assertions, zero signature changes. Run
  `go test ./... -race -count=2`.
- **Done when:** Suite green under `-race -count=2`; diff confirmed no-op;
  update `specs/progress.md` W3 + exit gate (no file >700 LOC).

---

## Definition of done (Spec 06)
- [ ] `program_orchestration.go` god-file eliminated; bands ≤~400 LOC each.
- [ ] No `internal/core` non-test file >~700 LOC.
- [ ] Exported API unchanged; diff is pure relocation.
- [ ] `-race -count=2 ./...` green.
