# Tasks — cmd-audit

## Wave 1
- [ ] T1 — Extract live command registry
  - why: Cannot optimize a surface we have not exhaustively measured
  - role: investigator
  - files: internal/core/commands.go, internal/cmd/
  - contract: Produce list of every top-level command + subcommand with category and flags
  - acceptance: Row count matches `grep`-verified registry entries (33 top-level)
  - verify: test -s .specd/specs/cmd-audit/registry.txt && wc -l .specd/specs/cmd-audit/registry.txt
  - depends: —
- [ ] T2 — Cross-reference documentation
  - why: Surface undocumented commands that would otherwise escape the audit
  - role: investigator
  - files: docs/command-reference.md, .specd/specs/cmd-audit/registry.txt
  - contract: For each registry command, mark documented|undocumented; mark doc-only commands as doc-orphan
  - acceptance: Every registry row has a documented flag; orphans listed
  - verify: grep -c undocumented .specd/specs/cmd-audit/registry.txt
  - depends: T1

## Wave 2
- [ ] T3 — Classify dispositions and overlap
  - why: Disposition ledger is the single source of truth for all downstream specs
  - role: investigator
  - files: .specd/specs/cmd-audit/registry.txt, PROMPT.md, specd_analysis_and_action_plan.md
  - contract: Apply §5 decision matrix; assign disposition∈{keep,merge,deprecate,meta-hidden} and overlap_with per row; seed §5.1 non-negotiables as keep
  - acceptance: Every row has disposition + overlap_with; 12 backbone commands == keep
  - verify: test -f .specd/specs/cmd-audit/audit.csv && awk -F, 'NR>1 && $10==""' .specd/specs/cmd-audit/audit.csv | wc -l | grep -qx 0
  - depends: T2
- [ ] T4 — Emit ledger + summary and assert ≤20
  - why: The ≤20 survivor target must be machine-checked, not asserted by prose
  - role: reviewer
  - files: .specd/specs/cmd-audit/audit.csv, .specd/specs/cmd-audit/audit-summary.md
  - contract: Write summary counts table; emit OVERFLOW marker iff survivors>20
  - acceptance: Summary present; survivor count ≤20; no OVERFLOW marker
  - verify: test -f .specd/specs/cmd-audit/audit-summary.md && ! grep -q OVERFLOW .specd/specs/cmd-audit/audit-summary.md
  - depends: T3

## Wave 3
- [ ] T5 — Gate the audit spec
  - why: The audit must itself pass specd validation before downstream specs consume it
  - role: verifier
  - files: .specd/specs/cmd-audit/
  - contract: `specd check cmd-audit` exits 0
  - acceptance: All core gates pass for cmd-audit
  - verify: specd check cmd-audit
  - depends: T4
