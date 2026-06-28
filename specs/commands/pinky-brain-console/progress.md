# Progress — `/pinky-brain` Orchestration Console

| Wave | Scope | Tasks | Status | Exit criteria |
|------|-------|-------|--------|---------------|
| 1 | Capability and status | T1-T3 | pending | Config and session status render safely. |
| 2 | Enable/disable | T4-T5 | pending | Orchestration can be explicitly toggled without corrupting config/session files. |
| 3 | Session ops and workers | T6-T8 | pending | Brain session actions delegate natively; worker view read-only. |
| 4 | Guards, tests, docs | T9-T10 | pending | POSIX guard, safety tests, docs complete. |

## Completion checklist

- [ ] Native Brain/Pinky availability detected.
- [ ] Enable/disable explicit only.
- [ ] Config writes atomic if direct writer exists.
- [ ] No Pinky report/verification forged.
- [ ] Windows/WSL guard present.
- [ ] Docs updated.
