# Tasks — cmd-docs

## Wave 1
- [x] T1 — Regenerate command reference from live registry ✓ complete · evidence: docs/command-reference.md regenerated from help registry; grep count recorded · 2026-06-30T16:18:34.672746023Z
  - why: The reference must equal the registry or drift re-introduces dead commands
  - role: builder
  - files: docs/command-reference.md
  - contract: Table lists exactly the surviving palette with flags + exit codes; merged behaviors under survivor flags
  - acceptance: Every reference command exists in the registry; no merged command has a standalone entry
  - requirements: 1
  - verify: grep -c 'specd ' docs/command-reference.md
  - depends: —
- [x] T2 — Update agent-integration for MCP parity ✓ complete · evidence: docs/agent-integration.md updated for survivor MCP parity and intent mappings · 2026-06-30T16:18:34.824143704Z
  - why: Documented tools must match the parity-tested MCP surface
  - role: builder
  - files: docs/agent-integration.md
  - contract: Document only parity-passing tools; include intent→flag mappings; note meta-hidden exclusion
  - acceptance: No tool documented that fails TestCLIMCPParity
  - requirements: 2
  - verify: ! grep -E 'specd (serve|watch|replay|diff|dispatch)' docs/agent-integration.md
  - depends: —

## Wave 2
- [x] T3 — Write migration appendix + cheat-sheet sentences ✓ complete · evidence: Migration appendix covers 13 killed commands; CHEATSHEET exists with 20 entries · 2026-06-30T16:18:48.02472413Z
  - why: A single old→new table makes the breaking change adoptable in one pass
  - role: builder
  - files: docs/command-reference.md, docs/user-guide.md, .specd/specs/CHEATSHEET.md
  - contract: old→new table covers every merged+deprecated command with removed_in; one sentence per survivor
  - acceptance: Appendix covers all 13 killed commands; CHEATSHEET count == survivor count
  - requirements: 3
  - verify: test -f .specd/specs/CHEATSHEET.md
  - depends: T1
- [x] T4 — Sweep README + AGENTS for dead command references ✓ complete · evidence: README.md and AGENTS.md swept for retired command invocations after update wording cleanup · 2026-06-30T16:21:35.123371242Z
  - why: Prose examples citing merged commands re-teach the old surface
  - role: builder
  - files: README.md, AGENTS.md
  - contract: Replace dead-command examples with survivor-flag equivalents
  - acceptance: No dead command name appears outside a migration link
  - requirements: 1
  - verify: ! grep -E '\bspecd (doctor|mode|dispatch|validate|schema|serve|watch|replay|diff|program|update|uninstall|migrate)\b' README.md AGENTS.md
  - depends: T1

## Wave 3
- [x] T5 — Docs-lint: no dead commands outside appendix ✓ complete · evidence: scripts/docs-lint.sh passed after final docs sweep · 2026-06-30T16:21:37.51785469Z
  - why: Automated lint is the durable guard against documentation drift
  - role: reviewer
  - files: docs/, scripts/docs-lint.sh
  - contract: Lint scans docs+README+AGENTS for dead-command names from audit.csv; whitelists appendix anchor
  - acceptance: Lint exits 0
  - requirements: 1, 2, 3
  - verify: bash scripts/docs-lint.sh
  - depends: T2,T3,T4
- [x] T6 — Gate cmd-docs spec ✓ complete · evidence: specd check cmd-docs passed after final docs sweep · 2026-06-30T16:21:37.675932683Z
  - why: Final documentation spec must pass validation to close the suite
  - role: verifier
  - files: .specd/specs/cmd-docs/
  - contract: `specd check cmd-docs` exits 0
  - acceptance: All core gates pass
  - requirements: 1, 2, 3
  - verify: specd check cmd-docs
  - depends: T5
