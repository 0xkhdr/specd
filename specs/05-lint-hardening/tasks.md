# Spec 05 — Tasks (Lint Hardening)

> Prereq: **Spec 01 done** (subprocess code isolated in `internal/worker` →
> `gosec` triage is contained).

---

## Wave A — Enable

### [ ] W3.1a — Add linters to config
- **Files:** `.golangci.yml`
- **Do:** Add `errorlint, gosec, bodyclose, gocritic, unconvert, misspell` to
  the `linters.enable` list (keep existing five). Add minimal per-linter config
  only if needed (e.g. `gosec` excludes for the documented subprocess path can
  be done via `//nolint`, not global excludes).
- **Done when:** `golangci-lint run` executes with all eleven linters (may report
  findings — that's Wave B).

---

## Wave B — Triage & fix

### [ ] W3.2a — Fix `bodyclose` findings
- **Files:** `internal/cmd/update.go`, `internal/cmd/watch_sse.go` (+ any others)
- **Do:** Ensure every `http.Response.Body` is closed (`defer resp.Body.Close()`
  after error check). Fix leaks.
- **Done when:** `bodyclose` clean.

### [ ] W3.2b — Fix `errorlint` findings
- **Files:** wherever flagged
- **Do:** Convert direct error comparisons to `errors.Is`, type assertions to
  `errors.As`, and ensure wrapping uses `%w` where the wrapped error is meant to
  be inspectable. Leave intentional non-wrapping (`%v`) where unwrapping is not
  desired — confirm each is deliberate.
- **Done when:** `errorlint` clean.

### [ ] W3.2c — Triage `gosec` + apply narrow suppressions
- **Files:** `internal/worker/shell_runner.go` (subprocess), temp-file sites,
  any `os` perms findings
- **Do:** Fix genuine issues (e.g. file perms, unhandled errors). For the
  intentional operator-supplied `sh -c` worker command, add a per-line
  `//nolint:gosec // worker command is operator-supplied by design; see SECURITY.md`
  with the rationale. No blanket disables.
- **Done when:** `gosec` clean (real fixes + commented narrow suppressions).

### [ ] W3.2d — Apply `gocritic`/`unconvert`/`misspell` mechanical fixes
- **Files:** wherever flagged
- **Do:** Apply only mechanical fixes (redundant conversions, typos, simple
  `gocritic` suggestions). Defer anything structural to a follow-up; suppress
  with a comment if a suggestion conflicts with existing idiom.
- **Done when:** These three linters clean.

---

## Wave C — Verify

### [ ] W3.2e — CI lint green
- **Files:** n/a (CI)
- **Do:** Confirm the lint job passes across the OS matrix. Run
  `./scripts/test-lint.sh` if it wraps the linter.
- **Done when:** Lint job green on `level-up`; update `specs/progress.md` W3.

---

## Definition of done (Spec 05)
- [ ] Six new linters enabled, all clean (fixes + narrow commented suppressions).
- [ ] HTTP bodies closed; error wrapping/compare correct.
- [ ] CI lint green; stdlib-only runtime invariant intact.
