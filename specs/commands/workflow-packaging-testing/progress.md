# Progress — Slash Workflow Packaging, Tests, and Documentation

| Wave | Scope | Tasks | Status | Exit criteria |
|------|-------|-------|--------|---------------|
| 1 | Shared command pack | T1-T3 | complete | One shell pack and one Python CLI expose all workflows with shared helpers. |
| 2 | Test harness | T4-T6 | complete | Fake specd harness, safety invariants, parity and exit tests pass. |
| 3 | Docs and skills | T7-T9 | complete | README/AGENTS/skills updated as selected by implementation. |
| 4 | CI readiness | T10-T11 | complete | Wrapper tests in local gate; `make test`/`make ci` pass as required. |

## Completion checklist

- [x] Single shell source file.
- [x] Python CLI fallback.
- [x] Fake `specd` tests.
- [x] Safety invariants enforced.
- [x] Docs and optional skills updated.
- [x] Final verification complete.
