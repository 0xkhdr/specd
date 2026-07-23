# Requirements — workflow-12-reset-hygiene

Release L closes the reset and harness-hygiene findings: a reopened spec that cannot be re-executed,
a program view that disagrees with per-spec lifecycle, a feedback inventory that no task can legally
sync, and defense-in-depth gaps around refusal codes, slug paths, running-task scope amendment, and
binary pinning. Source: `WORKFLOW-FEEDBACK.md` open entries on reopen completeness, approval-request
cycle reset, program projection, feedback inventory generation, and harness hardening.

## R1 — Reopen restores executability

owner: project maintainers
priority: must
risk: high

- R1.1: When a task is reopened, the system shall set that task status back to pending and rewrite its tasks marker in the same transaction that invalidates its evidence.
- R1.2: When a spec is reopened, the system shall apply the same pending reset to every task in the reopened cycle so the spec can be walked back to executing through approve.
- R1.3: When a spec cannot be approved into executing because tasks are not in an accepted terminal disposition, the system shall wire the existing pending-completion readiness check into the executing approval gate.

## R2 — Reopened cycles re-approve cleanly

owner: project maintainers
priority: must
risk: high

- R2.1: When a spec is reopened into a new cycle, the system shall clear the approval and approval-request records for gates at or after the reopened phase, keyed by cycle so history is retained rather than deleted.
- R2.2: When a reopened cycle re-requests a gate, the system shall allow the transition rather than reporting the prior cycle approval as already approved.

## R3 — Program view derives from lifecycle state

owner: project maintainers
priority: must
risk: high

- R3.1: When the program view projects completion and dependency satisfaction, the system shall derive both from loaded per-spec lifecycle state rather than a separate projection that can disagree.
- R3.2: When every spec in a chain sits at requirements, the system shall report only the root spec as actionable and no dependency as complete.

## R4 — Feedback inventory is generated, not hand-synced

owner: project maintainers
priority: must
risk: medium

- R4.1: When structural lint checks the feedback inventory, the system shall generate the inventory from the feedback log so an appended observer entry does not create a cross-scope lint failure no task can fix, or exempt open-status entries until a dedicated maintenance task dispositions them.

## R5 — Refusal codes and slug paths are guarded at the sink

owner: project maintainers
priority: must
risk: high

- R5.1: When a refusal is constructed with a code absent from the refusal table, the system shall fail a test rather than silently emitting the generic template.
- R5.2: When any per-spec path under the specs directory is built, the system shall route through one join helper that validates the slug at the sink so traversal is neutralized regardless of the caller.

## R6 — Running tasks have a governed scope amendment

owner: project maintainers
priority: must
risk: medium

- R6.1: When a running task must touch a file its row did not declare, the system shall provide a governed scope-amendment transaction that appends the path and records an auditable workflow event, so the only route is not a hand edit of tasks.md.
- R6.2: When mid-requirement repair intent is recorded, the system shall either create or authorize the amendment transaction or name the next legal authoring action rather than advancing a revision with no executable recovery.

## R7 — Hosted binary matches the bootstrap pin

owner: project maintainers
priority: should
risk: medium

- R7.1: When agent MCP hosting is generated, the system shall register the same resolved binary the bootstrap pinned, or default the repository scaffold to the local executable when it exists.
- R7.2: When the doctor runs, the system shall compare the MCP command version and commit against the active handshake pin and report an actionable mismatch.

## Edge and failure behavior

- A reopened and re-approved spec reaches executing with every task pending and no hand edit of state.json.
- The program frontier with all specs at requirements lists exactly one actionable spec.
- A traversal slug is rejected at the path sink even for a verb that skips phase enforcement.

## Non-goals

- Deleting historical approval, evidence, or migration records; cycle rollover keys them, it does not erase them.
- Adding an evidence or approval bypass; the scope-amendment transaction is human or operator authored and audited.
- Replacing git as the source of the diff-scope baseline.
