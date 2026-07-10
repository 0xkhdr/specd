# Design — Context Manifest v2

## Decision

Keep context selection local, deterministic, host-neutral. `internal/context` emits references
and compact metadata; it does not inline repository content or call external systems. Host loads
only validated selected representations and echoes receipt identity to later adapters.

`ManifestVersion = "2"` is new contract. V1 input/output compatibility decision lands before
default switch: either preserve V1 renderer behind explicit version, or reject it with documented
migration. No silent schema interpretation.

## Contract

```text
ManifestV2
  schema_version, kind=context_manifest, root, slug, action, phase, task_id
  config_digest, palette_digest, items[], required_tokens, optional_tokens,
  budget, omissions[], provenance, manifest_digest

Item
  kind, source, selector?, source_digest, representation_digest?, required,
  load_mode, priority, reason, trust, sensitivity, authority_limit,
  estimated_tokens, applicability?, route?, capability?, omission_reason?

Receipt
  manifest_digest, config_digest, palette_digest, skill_digests[],
  required_context_digests[], created_from, totals
```

`source` is canonical root-relative slash path, or explicit inline machine contract. `root`
identifies repository base. `selector` identifies stable section/symbol/ID; selected bytes get
`representation_digest`. All maps/orderings canonical before digest.

## Selection order

1. Validate root, slug, task, role, action, config/palette inputs.
2. Resolve required lanes: harness/role/guardrails; exact task; requirements; applicable design;
   normalized declared files.
3. Canonicalize and reject traversal, absolute escape, disallowed symlink escape, missing/read
   errors. Compute source and selected-representation digest/tokens.
4. If required total exceeds budget: fail. Never drop/truncate required item.
5. Select optional steering/memory/examples/skills by static metadata. Sort priority then stable
   identity. Shed optional items only in documented order, recording each omission.
6. Add tools and route metadata from command palette/handshake. Validate skill capabilities are
   subset of role/phase/policy capability.
7. Canonicalize, calculate manifest digest, render CLI JSON/HUD/reference list, emit receipt.

## Authority and trust

Instruction precedence is typed data: harness/guardrail > role > project instructions > knowledge
> examples/memory/external input. File contents cannot alter `kind`, route, authority, role,
files, approval, or gate policy. Trust/sensitivity labels travel with references; raw secrets and
private prompts never enter manifest, evidence, or reports.

## Migration and integration

- Extend task parser only through backward-compatible metadata. Preserve task Markdown bytes.
- Domain 01 owns known roles/declared-file task semantics. Domain 02 consumes them; Domain 06
  enforces actual post-work scope.
- Domain 03 owns legal next action. Domain 02 carries route/capability/digests in context.
- Domain 04 owns evidence gate. It validates receipt freshness using receipt identity; receipt
  alone never passes evidence.
- Existing `context` default plain-path output remains explicit compatibility renderer until
  documented migration; JSON V2 is authoritative machine contract.

## Verification layers

- Unit/golden: schema validation, canonical order/digest, resolver, selectors, token estimator.
- Negative: wrong root, traversal/symlink, missing required item, unknown type, stale selector,
  invalid skill/over-capability, expired memory, overflow.
- Black-box: fresh fixture `init → handshake → context/next --dispatch`; CLI/MCP route and drift.
- Conformance: host resolves only emitted root/base/digest bytes; old receipt rejected after input
  mutation; no content leak in JSON/report.
