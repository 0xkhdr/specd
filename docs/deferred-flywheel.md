# Deferred Flywheel Modules

Specd v2 keeps the flywheel surface minimal. Security scanning ships as an
opt-in deterministic gate; deploy, observe, eval, review, submit, ingest, and
migration workflows remain deferred.

Future modules must return through the gate and evidence architecture already in
the harness. They must not add LLM or network calls to deterministic decisions.
They must preserve the documented evidence shapes below when they return:

- DeployApproval
- SecurityScan
- ReviewRecord
- EvalRecord
- IngestRecord
- MigrationRecord
