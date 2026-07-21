# Unattended approval — analysis and recommendation

**Date:** 2026-07-21 · **Status:** proposal, nothing implemented · **Binary:** specd 1.0.0 (2549cf5)

Requirement, as stated by the operator: *"configure specd to allow skip user
approval"* — run a whole spec, or a whole program of specs, without a human
typing `specd approve` at each gate.

This document analyses the requirement, states what already exists, rejects the
readings that would break an invariant, and recommends one shape. Per
`CLAUDE.md` ("never act on your own recommendation in the same run"), no code
here is changed by this analysis.

---

## 1. What exists today

There is no configuration knob. `grep -rni "auto_approve\|skip_approval\|unattended" internal/ docs/`
returns nothing, and no `SPECD_*` env var covers it. That is not an oversight —
approval is a **verb someone invokes**, not a gate that can be switched off.

Four facts constrain any solution:

| Fact | Location | Consequence |
|---|---|---|
| `approve` runs the gate registry and refuses on any error finding | `internal/cmd/lifecycle.go:110` | "Skipping approval" never means skipping gates. There is no `--force`. |
| `HumanOnly` is metadata, never checked at runtime | `internal/core/commands.go:106`; `runApprove` reads it nowhere | Any agent can already run `specd approve`. The invariant holds by convention only. |
| Every record carries `Actor`, from `$SPECD_ACTOR` → OS user → `unknown` | `internal/core/state.go:62`, `recordActor()` at :77 | Provenance is *recorded* but not *classified*. An agent shelling out as the logged-in user writes the same bytes a human does. |
| `approve` into `executing` refuses while any `follows` dependency is incomplete | `internal/cmd/lifecycle.go:118` | Program ordering is already enforced; automation cannot reorder a pipeline. |

So the honest current answer to the requirement is: *it already works, invisibly,
and that is the problem.* The operator can point an agent at `specd approve`
today. Nothing stops it, and afterwards nothing in `state.json` can tell anyone
it happened.

## 2. What the requirement actually asks for

Three different requests hide behind one sentence. They deserve different answers.

- **(a) Remove the human from the loop for an automated test run.** Legitimate.
  The operator *is* the human and is delegating their own authority.
- **(b) Make approvals cheaper because the gates already check the artifacts.**
  Partly true, and partly a misreading — see §3.
- **(c) Advance a spec whose gates fail.** Not in scope. No design below permits
  it, and no future one should.

The reading worth building for is (a): **delegated approval**, declared and
auditable, not a bypass.

## 3. What is lost when a human stops approving

Worth stating plainly, because it is the whole cost of this feature.

The gates check *form*: EARS syntax, section presence, task schema, DAG
acyclicity, evidence integrity, budget. They cannot check *fit* — whether these
requirements are the right requirements, whether this design is the one the
operator wanted, whether the acceptance criteria are demanding enough to mean
anything.

The concrete failure mode is self-serving acceptance: an agent authoring
`tasks.md` and approving it writes criteria it knows it can satisfy. Every gate
passes, evidence is genuine, and the spec certifies work nobody specified. No
gate in the registry detects this, because at every step the artifacts are
well-formed.

This argues for delegation being **visible and reviewable after the fact**, not
for forbidding it.

## 4. Options considered

### A — Do nothing; document the wrapper script

Operator scripts `specd check && specd approve` in a loop. Zero code.

*Rejected as the endpoint.* It is exactly today's situation: it works, and it
leaves approval records that lie by omission. But it is a fine **stopgap**, and
it is what the operator should use this week.

### B — `SPECD_ACTOR` convention plus docs

Tell operators to export `SPECD_ACTOR=delegated-agent` for unattended runs, and
document it. Zero code, real audit signal.

*Rejected as sufficient, kept as the migration path.* It is opt-in honesty: the
runs most worth flagging are the ones least likely to set it. Nothing validates
the value, nothing surfaces it, no gate reacts to it.

### C — First-class delegated approval  ← **recommended**

Delegation becomes a declared configuration state, recorded on the approval and
surfaced by the read paths. Detail in §5.

### D — `brain` auto-advances the lifecycle

Let the controller approve phases itself as part of the wave loop.

*Rejected.* `brain` is deterministic and stays that way — but this collapses the
last checkpoint between "an agent wrote a plan" and "the harness executed it"
into a single unattended process, and does it *silently* by default for everyone.
It also inverts the layering: the controller would gain authority over the
lifecycle that authorises it. If C exists, an operator can already get this by
configuring it — explicitly, per project, on the record.

## 5. Recommended design

Four changes. Together they turn an undetectable convention violation into a
declared, auditable operating mode. **No gate is weakened; `approve` keeps
refusing on failing readiness gates in every mode.**

### 5.1 Config: declare delegation

```yaml
approval:
  delegation: off        # off | agent
```

Default `off`, preserving today's behaviour. Add to `internal/core/config_validate.go`
alongside the other keys, and to the shipped `project.yml` template with the
tradeoff stated in the comment. `profile: production` should force it back to
`off` — an unattended production release is not a thing specd should make easy.

### 5.2 Runtime: enforce `HumanOnly` against it

`HumanOnly` stops being decorative. A human-only verb (`approve`, `mode`,
`midreq`, `decision`, `exception`) resolves an actor class:

- `$SPECD_ACTOR_CLASS=agent` (new, narrow, allowlisted like `SPECD_ACTOR`) → the
  caller declares itself an agent
- unset → `human`

With `delegation: off` and class `agent`, the verb refuses:

```
approve refused: human-only verb invoked by a delegated agent;
set approval.delegation: agent in project.yml to permit it
```

**This is attestation, not proof.** An agent that omits the variable is
indistinguishable from a human, exactly as today. The value is that an honest
automated run has a supported way to say what it is, and a dishonest one has to
actively lie rather than merely stay quiet. Anyone who reads this as a security
boundary has misread it, and the doc comment should say so.

### 5.3 State: record it

Add one field to `core.Record`:

```go
Delegated bool `json:"delegated,omitempty"`
```

`omitempty` keeps existing `state.json` files byte-identical and needs no
`StateSchemaVersion` bump — older binaries ignore the unknown key, newer ones
read absence as `false`. Set it from the actor class at stamp time, next to the
existing `Actor` field.

### 5.4 Read paths: surface it

- `specd status <slug> --json` — count delegated approvals per spec.
- `specd status --program` — mark any spec that advanced under delegation.
- `specd check` — a **warning**-severity finding when a spec reached `complete`
  with delegated approvals. Warning, not error: delegation is permitted, and
  the operator asked for it. It should just never be a surprise later.
- `docs/user-guide.md` — one section naming the supported unattended pattern,
  and §3 of this document as its rationale.

### 5.5 Scope explicitly excluded

- No `--force`, no `--skip-gates`, no bypass of any kind.
- No change to evidence: a task still closes only on a passing `specd verify`
  pinned to a resolvable HEAD.
- No auto-advance loop inside `brain` (option D).
- No weakening of the `follows`-dependency block on entering `executing`.

## 6. Cost

| Change | Size |
|---|---|
| `approval.delegation` key + validation + template comment | ~20 lines |
| Actor-class resolution + `HumanOnly` enforcement in the dispatch path | ~30 lines |
| `Record.Delegated` field + stamp | ~5 lines |
| `check` warning gate + status projections | ~40 lines |
| Docs: user-guide section, command-reference regen (`go run ./tools/gendocs`) | mechanical |

Roughly 100 lines and one regenerated doc. It adds a config key and a record
field, and it removes an undetectable convention violation — a net subtraction
in ambiguity, which is the axis `CLAUDE.md`'s subtractive bias actually cares
about.

## 7. What to do now

1. **This week, unattended runs:** option B. Export `SPECD_ACTOR=delegated-agent`
   before the loop, and keep a written record of what was approved and on what
   check output. Costs nothing, and the approval history stays meaningful.
2. **Then:** run option C through specd itself as a spec — it is a well-bounded
   vertical slice with real gates to satisfy, and dogfooding the delegation
   feature under manual approval is the right joke to make on purpose.
3. **Before implementing:** confirm §5.2's actor-class variable belongs in the
   scrubbed-env allowlist next to `SPECD_ACTOR`, and decide whether `profile:
   production` forcing `delegation: off` is a hard error or a warning at load.

## 8. Open questions

- Should delegated approval require a `--reason`, so the *why* is captured even
  when the *who* is a script? Leaning yes for `approve`, no for the others.
- Does a delegated approval invalidate a later human approval of the same gate,
  or stack with it? Current `appendRecord` semantics would stack; that is
  probably right, but it is untested.
- `specd submit` and the delivery gate already require human authority in
  `production`. Confirm delegation cannot reach them by any path.
