# Design — Driver contract and native guidance

## Decision

Create pure `core` projections. Thin CLI/MCP renderers call same projection. Command palette
remains source for parser metadata; guide derives legal actions from palette plus state/gates,
never a second hand-maintained command list. Domain 02 manifest V2 remains context authority.

## Envelopes

```text
BootstrapV1
  protocol_version, root, specs[], active_spec?, resolution?,
  palette_digest, config_digest, guidance_digest, context_schema_digest, findings[]

DriverGuideV1
  protocol_version, root, spec_slug, phase, status, approvals, frontier,
  blockers[], next_actions[], evidence_refs[], compatibility

NextAction
  id, command, args, actor, side_effect, authority_required,
  allowed_phases, source_ref, reason

Finding
  code, severity, ref, message, recovery_action

DispatchV1
  protocol_version, mission/task identity, role, declared_files, acceptance, verify,
  context_receipt, authority_ref, subject_head, palette/config/guidance digests, limits
```

All arrays stable-sort by explicit priority then identity. Canonical JSON digest covers serialized
semantic fields. `root` is canonical project root; all returned paths root-relative slash paths.
Envelope contains references/digests/compact decisions, never repository bytes, secrets, prompts,
or raw untrusted tool output.

## Active-spec resolver

One resolver at command/MCP boundary returns `(slug, source, finding)`. Order:

1. Valid explicit operand.
2. Valid `SPECD_SPEC` host pin.
3. Exactly one eligible discovered spec.
4. Otherwise `SPEC_AMBIGUOUS` or `SPEC_REQUIRED`.

Resolver has no write path. Commands needing a task/spec consume it consistently. If compatibility
cannot support optional operands, generated MCP config stops emitting `SPECD_SPEC` until support
lands; never leave half-contract.

## Flow

```text
bootstrap/doctor → root + compatibility + resolver findings
                         ↓
guide(slug) → state + gates + palette + frontier → actor-aware next action
                         ↓
context/dispatch → Domain 02 manifest/receipt + task facts → pinned envelope
                         ↓
CLI/MCP renderer → same structured result / typed refusal handoff
```

## Guidance truth

Managed template digest derives embedded managed regions/templates only. Scaffold preserves
user-owned bytes. `init --refresh` changes managed regions; bootstrap detects mismatch first.
Examples test extracts only deliberately runnable command blocks; placeholders use one documented
token and fixture substitution. Generated docs and command reference stay synchronized.

## Authority and failure

`actor=agent|human`; `side_effect=read|write|approval|external`; `authority_required` explicit.
Guide does not override dispatch/gate checks. Doctor is read-only. MCP mutation refusal renders
`MCP_HANDOFF_REQUIRED`, required actor, exact CLI command/reference. Stable codes replace prose
parsing. Unknown/missing/digest mismatch fails closed before mutation.

## Migration and verification

- Add V1 envelope behind `--json`/new guide surface. Preserve existing plain text until docs
  migration fixture passes.
- Unit/golden: canonical ordering, digest isolation, resolver precedence, action legality.
- Black-box: fresh init/new/context, managed command extraction, bad path/pin/template/multi-spec.
- Parity: CLI/MCP same fixture result; every suggested command parser-valid.
- Recovery: stale handshake/template/schema, wrong phase, missing context, forbidden MCP action.
- Remote envelope comes last; Domain 05/06 validates transport/worker/authority semantics.

## Risks

- Guide becoming second state machine → derive from existing palette/gates; parity tests.
- Convenience selecting wrong spec → only unambiguous fallback.
- Context bloat → references/required tiers; Domain 02 budget remains fail-closed.
- Digest sameness mistaken for correctness → semantic conformance fixtures required.
