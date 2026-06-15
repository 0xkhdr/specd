## Summary

What does this PR change and why?

## Related

Closes #… / relates to requirement R…

## Checklist

- [ ] `make ci` passes locally (lint + race + `-count=2` + coverage floor + stress)
- [ ] Tests added/updated for behavior changes
- [ ] No coverage floor lowered without written justification
- [ ] No new runtime dependency; invariants preserved (stdlib-only, zero LLM
      calls, deterministic output, Foundational Split, evidence gate)
- [ ] Docs updated if behavior/CLI/JSON output changed
