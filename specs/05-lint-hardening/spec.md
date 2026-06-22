# Spec 05 — Deepen Static Analysis

> Wave: **W3 (P2)** · Priority: **P2** · Source: LEVEL_UP_PLAN §1.4, §2 P2.7

## 1. Problem

`.golangci.yml` (v2 schema) enables only `staticcheck, errcheck, govet,
ineffassign, unused`. For a tool whose **job is running subprocesses** and doing
env propagation + file I/O, several high-signal linters are missing:

- `errorlint` — the tree has **402 `fmt.Errorf`** and **142 `errors.Is/As/%w`**
  sites; `errorlint` enforces correct wrapping (`%w`) and `errors.Is/As`
  comparison, catching the rest.
- `gosec` — flags subprocess / `os` misuse on exactly the kind of code specd
  runs (`exec.Command`, temp files, env).
- `bodyclose` — `update.go` and `watch_sse.go` perform HTTP; guards leaked
  response bodies.
- `gocritic`, `unconvert`, `misspell` — cheap polish, low false-positive.

These are **dev-only CI tooling** — they add **no runtime dependency** to the
shipped binary, preserving the stdlib-only invariant.

## 2. Solution

1. Add the six linters to `.golangci.yml` (v2 schema).
2. Run once, **triage** the findings, fix the real ones, and suppress the rare
   false positives with **narrow, commented** `//nolint:<linter> // reason`
   directives (never blanket-disable).
3. Keep CI green. Where `gosec` flags the intentional `exec.Command("sh","-c",…)`
   worker path (now in `internal/worker` after Spec 01), annotate with a
   `//nolint:gosec` + a comment pointing at the threat-model rationale in
   `SECURITY.md` (the command is operator-supplied by design).

## 3. Sequencing note

This is **easier after Spec 01** (`internal/worker` exists): `gosec`'s
subprocess findings concentrate in one small package with a clear, documented
justification, instead of being scattered through `cmd/brain.go`.

## 4. Acceptance criteria

- [ ] `.golangci.yml` enables `errorlint, gosec, bodyclose, gocritic,
      unconvert, misspell` (plus the existing five).
- [ ] First-run findings triaged: real issues fixed; suppressions are narrow,
      per-line, and commented with a reason.
- [ ] `bodyclose` confirms `update.go` / `watch_sse.go` HTTP bodies are closed
      (fix if not).
- [ ] `errorlint` clean — wrapping uses `%w`, comparisons use `errors.Is/As`.
- [ ] CI lint job green across the matrix.
- [ ] Stdlib-only runtime invariant preserved (linters are dev-only).

## 5. Non-goals

- Adopting opinionated style linters with high churn (e.g. `wsl`, `gochecknoglobals`).
  Keep the set high-signal.
- Large refactors triggered by `gocritic` — apply mechanical fixes only; defer
  anything structural to its own spec.

## 6. Risks & mitigations

| Risk | Mitigation |
|---|---|
| `gosec` floods on intentional subprocess use | Concentrated in `internal/worker` post-Spec-01; narrow commented `//nolint:gosec` tied to `SECURITY.md` |
| `errorlint` mass churn across 402 sites | Fix incrementally; most `fmt.Errorf` without verbs are fine — focus on wrap/compare correctness |
| New linter version drift breaks CI later | Pin linter versions consistent with current CI setup |
