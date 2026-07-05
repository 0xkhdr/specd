## Summary

What does this PR change and why?

## Related

Closes #… / relates to requirement R…

## Checklist

- [ ] `go test ./... -race -count=1`, `go test ./... -count=2`, `scripts/test-lint.sh`, and `scripts/docs-lint.sh` pass locally
- [ ] Tests added/updated for behavior changes
- [ ] No coverage floor lowered without written justification
- [ ] No new runtime dependency; invariants preserved (stdlib-only, zero LLM
      calls, deterministic output, Foundational Split, evidence gate)
- [ ] Docs updated if behavior/CLI/JSON output changed
