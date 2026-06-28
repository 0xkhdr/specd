# Progress — Interactive `/init` Command Wrapper

| Wave | Scope | Tasks | Status | Exit criteria |
|------|-------|-------|--------|---------------|
| 1 | Command model and probing | T1-T2 | pending | Option model exists; host probe works with JSON and fallback. |
| 2 | Interactive/non-interactive execution | T3-T4 | pending | Menus and flag mode construct safe `specd init` argv and propagate exit codes. |
| 3 | Hardening and docs | T5-T6 | pending | Tests and docs cover dry-run, orchestration, failures, examples. |

## Completion checklist

- [ ] Shell wrapper implemented.
- [ ] Python wrapper implemented.
- [ ] No direct state/scaffold writes outside native `specd init`.
- [ ] Non-TTY mode cannot hang.
- [ ] Tests pass.
- [ ] Docs updated.
