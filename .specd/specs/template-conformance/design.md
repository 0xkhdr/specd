# Design — template-conformance

> One structural gap, four symptoms: no test asserts that a shipped template
> satisfies the consumer that parses it. The fix is a conformance suite plus the
> four template corrections it would have caught.

- references: R1, R1.1, R1.2, R1.3, R2, R2.1, R2.2, R2.3, R3, R3.1, R4, R4.1, R4.2, R5, R5.1
- disposition: accepted
- owner: maintainer

## Boundaries

- Owned: `internal/core/embed_templates/` bytes, `internal/core/scaffold.go`, the
  `task-trace` gate message, and a new template-conformance test file.
- Excluded: selector semantics in `internal/context/steering.go`, the context budget,
  gate strictness, and any orchestration or MCP surface.

## Interfaces

- Steering template files gain a leading `specd-context` block with keys drawn from
  `steering.go:parseMetadata` (`id`, `version`, `priority`; selectors omitted so they match everything).
- `context.SelectSteering(root)` — consumer contract for steering templates.
- `core.ParseRequirements([]byte) (Doc, error)` and `core.RequirementIDSet` — consumer
  contract for the requirements template.
- The `quality-declaration` gate — consumer contract for the tasks template `evidence=` example.
- `specd check` findings list — carrier for the new whole-set steering diagnostic (warning severity).

## Invariants

- Templates stay `go:embed`'d; no runtime dependency is added.
- No LLM enters any gate, parser, or diagnostic path; every added check is a pure
  function of on-disk bytes.
- The new steering diagnostic is a warning-severity finding, never a completion gate,
  and manufactures no evidence.
- Per-file omission stays silent (R1.3); only total omission is loud (R1.2).

## Failure

- A template correction that breaks an existing project: managed regions are matched by
  `:v\d+` (`managed.go:100`), so old-version regions are still located and replaced by
  `--repair`/`--refresh`; `--dry-run` previews before any write.
- The whole-set diagnostic misfiring on an intentionally empty steering directory:
  contained by requiring at least one steering file present before the check reports.
- A conformance test that pins the wrong shape: contained by asserting through the real
  consumer function, never against a hand-copied expected string.

## Integration

- `specd init --refresh` is the migration path for existing projects; behavior of the
  `:v\d+` marker match is confirmed before any `TemplateVersion` bump is considered.
- CI already runs `gofmt`, `go vet`, `go mod tidy`, `scripts/test-lint.sh`, and
  `scripts/docs-lint.sh`; the conformance suite runs as an ordinary `go test` target and
  needs no new script.

## Alternatives

- Make `SelectSteering` accept metadata-less files by default — rejected: it deletes the
  explicit-applicability property that keeps machine manifests bounded and auditable.
- Add a `scripts/template-lint.sh` mirroring `docs-lint.sh` — deferred: a Go test reaches
  the consumer functions directly, so a shell wrapper adds surface without adding coverage.
- Bump `TemplateVersion` to 2 now — deferred to a recorded decision; the `:v\d+` match makes
  it unnecessary for correctness (plan A10 is a question, not a change).

## Verification

- R1.1: test asserting `SelectSteering` over a freshly scaffolded root returns zero
  omissions with reason `missing explicit applicability metadata`.
- R1.2/R1.3: test that a metadata-less steering directory yields exactly one diagnostic and
  that a single metadata-less file among valid ones yields none.
- R2.1: test asserting the scaffolded `requirements.md` yields a non-empty `RequirementIDSet`.
- R2.2/R2.3: scaffold-to-gates round-trip test filling only marked placeholders and asserting
  no format-class findings.
- R3.1: test asserting the empty-ID-set message names `requirements.md`.
- R4.1/R4.2: test asserting the editing instruction appears inside each managed region;
  `docs/agent-integration.md` states the two manifest paths.
- R5.1: the conformance suite itself — one case per template naming its consumer.

## Deployment

- Templates are compiled in, so rollout is a rebuild and reinstall of the binary followed by
  `specd init --refresh --dry-run` then `specd init --refresh` in each managed project.
- Observation: `specd context <slug> <task> --json` shows steering in `items`, not `omissions`.
- Ownership: maintainer.

## Rollback

- Trigger: a corrected template breaks scaffolding or a managed region fails to replace.
- Path: revert the template commit and rebuild; managed regions are regenerated from the
  binary on the next `--repair`, and content appended below closing markers is preserved
  across both directions.
