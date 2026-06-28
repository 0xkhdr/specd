# Progress — Interactive `/init` Command Wrapper

| Wave | Scope | Tasks | Status | Exit criteria |
|------|-------|-------|--------|---------------|
| 1 | Command model and probing | T1-T2 | complete | Option model exists; host probe works with JSON and fallback. |
| 2 | Interactive/non-interactive execution | T3-T4 | pending | Menus and flag mode construct safe `specd init` argv and propagate exit codes. |
| 3 | Hardening and docs | T5-T6 | pending | Tests and docs cover dry-run, orchestration, failures, examples. |

## Completion checklist

- [x] Shell wrapper implemented.
- [x] Python wrapper implemented.
- [x] No direct state/scaffold writes outside native `specd init`.
- [x] Non-TTY mode cannot hang.
- [x] Tests pass.
- [ ] Docs updated.
