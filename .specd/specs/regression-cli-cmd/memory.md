# Memory — Regression: CLI + Command Surface (args, lifecycle, JSON contracts)

<!--
Source-attributed, generalizable learnings (append-only). Use
`specd memory <spec> add --key <slug> --pattern "<one-line>" --body "<detail>"
  --source "<Turn N, Task T?, role>" --criticality <minor|important|critical> [--related k,k]`.
Only generalizable patterns, never raw observations. Promote to project steering at 3+ specs via
`specd memory <spec> promote --key <slug>`. Format:

## <key-slug>
**Pattern:** <one-line generalizable claim>
**Detail:** <why it's true; the mechanism>
**Source:** Task T3, Turn 2, discovered by investigator
**Criticality:** important
**Related:** [[other-key]]
-->

## cli-surface-inventory
**Pattern:** 23 dispatchable subcommands; 9 support `--json`; `help`/`version`/`mcp` handled in main.run pre-dispatch.
**Detail:** Registry (internal/cmd/registry.go): init, new, approve, decision, midreq, memory, next,
dispatch, program, verify, task, status, context, check, validate, schema, report, replay, diff,
serve, watch, waves, update, uninstall. `--json` cmds: approve, check, context, dispatch, next,
program, status, uninstall, waves. Exit taxonomy (core/exit.go): ExitOK=0, ExitGate=1, ExitUsage=2,
ExitNotFound=3.
**Source:** Task T1, investigator
**Criticality:** important

## cli-coverage-gaps
**Pattern:** Base tests are strong; T2-T4 close 4 specific gaps, not whole-surface rewrites.
**Detail:**
- R1 parse/help: args_test.go (ParseArgs units) + main_test.go (TestRunTopLevelExitCodes,
  TestRunHelpJSONAndErrors) exist. GAP: no per-subcommand "unknown flag / missing required arg
  → non-zero usage" table. → T2.
- R2 json: json_contract_test.go locks status/context/next/dispatch/program. GAP: approve, check,
  uninstall, waves `--json` schema unasserted; no explicit ANSI-free assertion; no error-path-emits-
  JSON assertion. → T3.
- R3 lifecycle: lifecycle_test.go TestFullLifecycle is full E2E (new→approve×3→verify→task→verifying
  →approve→complete). Covered.
- R4 exit codes: scattered RunExpect(ExitGate/Usage/OK) + main_test top-level. GAP: no consolidated
  golden {scenario→code} taxonomy table. → T4.
**Source:** Task T1, investigator
**Criticality:** important
**Related:** [[cli-surface-inventory]]

## cli-regression-review
**Pattern:** CLI contract regression (T2-T4) reviewed clean: no byte-equality goldens, no ANSI leak, exit codes documented.
**Detail:** All --json asserts unmarshal into structs (presence/type, not bytes). ANSI freedom guarded
by TestJSONNoANSI + structurally guaranteed (PrintJSON colorless, NO_COLOR=1 in harness). Exit-code
documentation enforced via TestEveryRegisteredCommandHasHelp (EXIT CODES + code 0 per cmd) and
TestRegistryMatchesHelp (Registry<->metadata parity, no UNMAPPED). Stable under -count=2. R1.2 parser
permissiveness recorded as ADR-001 (not faked). Residual (non-blocking): approve/uninstall not in the
ANSI scan list; help test couples to "0  " spacing.
**Source:** Task T5, reviewer
**Criticality:** minor
**Related:** [[cli-coverage-gaps]]
