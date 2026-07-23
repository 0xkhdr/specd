# Design — workflow-10-observability

- references: R1,R1.1,R1.2,R1.3,R2,R2.1,R2.2,R3,R3.1,R3.2,R4,R4.1,R4.2,R5,R5.1,R5.2,R6,R6.1,R6.2,R6.3,R6.4
- disposition: accepted
- owner: project maintainers

## Boundaries

- `internal/cmd/status.go`, `mode.go`, and `verify.go` own truthful read surfaces and palette-derived usage.
- `internal/core/commands.go` remains the canonical command and flag metadata source; help and dispatch consume it rather than duplicating enumerations or usage.
- `internal/core/gates/contextbudget.go` owns deterministic contribution data; command guidance owns the actor-legal recovery sequence.
- `internal/cmd/lifecycle.go` preserves finding severity through text and JSON exit decisions.
- `internal/cmd/brain_run.go` owns the controller halt exit classification.
- `internal/core/review.go` owns review parsing; status projects parsed review data read-only, while the review command owns guarded scaffolding.
- Excluded: new mutation authority, an LLM in guidance, and a new review schema.

## Interfaces

- Status text/JSON expose the current mode; `mode <slug>` becomes a read when no value is supplied.
- Status guide emits an exact criterion verification command only for uncovered criteria referenced by completed tasks.
- Handler arity errors and `help <verb>` render the same palette usage. Enumerated flags render their allowed values.
- A leading `--help` or `-h` after a known verb routes to command help before handler validation; other unknown flags remain exit 2.
- Context-budget findings carry ordered per-source token contributions and one recovery sequence naming the editable artifact, authorized role, re-check, and human approval.
- `check` exits non-zero when any rendered finding is error severity in either output mode.
- A controller halt before dispatch uses a distinct stable exit code; successful dispatch behavior is unchanged.
- Status JSON includes an optional parsed review object with verdict token, note, reviewer, and HEAD.
- Review scaffolding refuses an existing report unless `--force`; forced replacement requires explicit acknowledgement and preserves the prior bytes in a deterministic backup.

## Invariants

- Read operations do not mutate spec state.
- Palette metadata is the single source for usage, enum help, and accepted flags.
- Text and JSON output make the same success/failure decision from the same findings.
- Criterion guidance appears only when evidence is absent; covered criteria produce no blocker.
- Review parsing preserves the original qualifier as a note, and existing reports are never silently overwritten.
- All guidance and exit decisions remain deterministic functions of repository state and command input.

## Failure

- Missing or malformed mode/review data fails closed with the existing typed state or parse refusal and names the source path.
- Unknown flags exit 2 before mutation; help flags exit 0 without invoking the command handler.
- Budget overflow reports every counted source in stable order and names the next legal authoring action.
- Error-severity checks return non-zero even when JSON rendering succeeds.
- A zero-dispatch controller halt returns its distinct code and retains the existing reason text.
- Existing review report without `--force` refuses and names the path; failure while preserving the prior report leaves the original intact.

## Integration

- Additive status JSON fields preserve existing consumers.
- Existing palette generation and command-reference generation remain unchanged except for richer enum text.
- Criterion guidance reads the existing task acceptance and criterion evidence stores; it creates no second ledger.
- Review projection reuses the parser used by review gates instead of reparsing in the status handler.
- No runtime dependency, configuration key, or state migration is added.

## Alternatives

- Add separate read-only mode/review verbs: rejected; status and the existing mode command already own those read surfaces.
- Hand-write improved usage strings in handlers: rejected; that preserves the drift that caused R2.2 and R3.
- Return non-zero for every controller wait: rejected; only a halt before any dispatch needs the distinct unattended signal.
- Silently overwrite review reports with empty HEAD: rejected; existence, not field content, determines overwrite safety.

## Verification

- Status/mode tests prove text and JSON mode projection without state revision changes.
- Coverage tests prove uncovered completed criteria emit exact verify guidance and covered criteria do not.
- Palette/CLI tests prove enum rendering, verb help routing, shared verify usage, and unknown-flag exit 2.
- Context-budget tests prove stable per-source contributions and the actor-ordered recovery sequence.
- Lifecycle and Brain tests prove severity/exit parity and the distinct zero-dispatch halt code.
- Review tests prove status projection, qualified-verdict parsing, existing-file refusal, forced backup, and failure preservation.
- Full race tests, docs lint, and domain regressions preserve command parity and evidence invariants.

## Deployment

- Changes activate immediately with no migration or feature flag.
- Observe typed refusals, exit codes, and JSON compatibility through the workflow regression suite.

## Rollback

- Revert the task commits if exit-code consumers regress or status JSON compatibility breaks.
- Additive JSON fields and backup files require no state rollback.
