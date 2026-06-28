# Progress — `/steer` Steering Console

| Wave | Scope | Tasks | Status | Exit criteria |
|------|-------|-------|--------|---------------|
| 1 | Root discovery and read views | T1-T3 | complete | `/steer show/status` work from nested dirs and filter canonical files. |
| 2 | Editing and bootstrap | T4-T5 | pending | Safe editor/stdin bootstrap supports dry-run and only edits steering Markdown. |
| 3 | Memory and docs | T6-T7 | pending | Memory action plus docs complete. |

## Completion checklist

- [x] Missing `.specd/steering` returns 3.
- [x] Arbitrary path args rejected.
- [x] Stub detection deterministic.
- [ ] Shell/Python parity tested.
- [ ] Docs updated.
