# Unattended approval

`specd approve` records a human decision. Some runs legitimately have no human
at the keyboard — a nightly pipeline, a long batch, an operator who wants to
delegate *their own* authority for a bounded window. This document is how that
is supported, and — just as importantly — what it deliberately does not do.

**Nothing here weakens a gate.** A delegated approval runs the same readiness
evaluation as an interactive one, in the same function, and refuses on the same
findings. There is no `--force`, no `--skip-gates`, and no flag that advances a
spec whose gates fail. If you are looking for one, the answer is no, and the
reason is that an approval nobody could have refused is not an approval.

## What delegation is

A **grant**: a scoped, bounded, revocable delegation of approval authority,
issued by an operator and used later by an automated run.

A grant is bound to a project, a closed set of specs, an exact set of
transitions, a maximum number of uses, and an expiry. It pins the config and
policy digests it was issued under. It cannot authorize anything on the
prohibition list, and it says so on its own record.

It is off by default. Until a project sets `delegation.enabled: true` in
`project.yml`, every delegation path is inert and nothing is written to disk.

## Using it

```bash
# 1. Turn it on for this project (project.yml)
delegation.enabled: true

# 2. Issue a grant. The bearer token prints exactly once.
specd delegate issue payments \
  --grant nightly \
  --transitions approve.requirements,approve.design \
  --uses 2 --expires-in 12h --reason-required

# 3. Store the token in host secret storage. It is not recoverable from .specd/.
export SPECD_GRANT_TOKEN=...

# 4. The unattended run uses it.
specd delegate approve payments \
  --grant nightly --token "$SPECD_GRANT_TOKEN" --reason "nightly unattended run"

# 5. When the window closes.
specd delegate revoke nightly --reason "run finished"
```

`--transitions` names transitions exactly (`approve.<gate>`). There are no
patterns and no wildcards: a grant that covers `approve.design` cannot approve
anything else, and a typo scopes the grant to nothing rather than to everything.

## The transaction

One delegated approval is one transaction, and it is built so that the two
failure modes that matter — a use spent on a refused approval, and an approval
that spends two uses — cannot happen.

1. Take the **authority lock**, then reconcile any reservation an earlier run
   left open (see recovery below).
2. Read the spec's current status and revision, and derive the request id:
   `<spec>:<gate>:<revision>`. That id is the replay key — one transition of one
   spec at one revision gets at most one use, ever.
3. Authorize the request against the grant (token, revocation, expiry, project,
   spec, transition, prohibitions, production permission, policy digests,
   reason, remaining uses) and **reserve** one use. All of it inside the lock,
   so two consumers cannot both see the last use as available.
4. Run the ordinary approval: spec lock, readiness gates, phase ratchet, CAS.
   Byte for byte the path `specd approve` runs.
5. Gates passed → **consume** the reservation. Gates refused, or the revision
   moved → **release** it and return the refusal.

The lock order is fixed and enforced in code: **authority lock before spec
lock**. Taking them the other way round returns `ErrLockOrder` rather than
deadlocking.

### Recovery

If the process dies between the reservation and the commit, the reservation is
left open — and an open reservation counts against the grant's uses, so doing
nothing would silently burn one.

The next delegated approval reconciles it from the ledger rather than from a
timeout. The request id names the spec, the gate, and the revision, so the
question "did that approval commit?" has an exact answer on disk: the approval
record for that gate pins the revision it advanced from.

- Approval record present at that revision → the use was spent. **Consume.**
- Absent → the transaction never committed. **Release**, and the use is
  available again.

Running recovery twice changes nothing. And because the authority lock is held
for the whole transaction, an open reservation observed by anyone else always
belongs to a process that is gone — a live one is never mistaken for a corpse.

## Reading the audit

A delegated approval is never mistaken for a human one. Approval records written
under a grant carry `scope=delegated`, and a companion `delegation:<gate>` record
carries the grant id, the request id (which use it was), the actor class, the
assurance that actor was worth, and the reason:

```
delegated approval grant=nightly use=payments:design:7 actor=unknown assurance=advisory reason=nightly unattended run
```

`actor=unknown assurance=advisory` is the honest default, not a bug. A CLI
process cannot prove who invoked it — OS username, TTY, and `SPECD_ACTOR` are
display provenance, not attestation — so only a governed host declaration raises
the class above `unknown`. See [host-contract.md](host-contract.md).

The grant ledger under `.specd/authority/grants.jsonl` records the same story
from the other side: issue, revoke, and every reservation, consumption, and
release, in append order. It never contains the bearer token — only its SHA-256
digest — and neither do refusals, which name the grant but never echo the secret
they rejected.

## What a grant can never do

Regardless of what an operator writes into one:

- `verify` and `complete-task` — delegation never manufactures evidence.
- `exception` — security exceptions stay human.
- `release`, `deploy`, `archive`, `submit` — nothing ships under delegation.

These are refused when the grant is issued, and recorded as explicit
prohibitions on the grant itself so a reviewer reads them from the record rather
than from this page. A production-profile transition additionally requires the
grant to have been issued with `--production`.

## Refusal codes

Each refusal is stable and names the operator route, rather than asking a script
to parse a sentence:

| Code | Meaning |
|---|---|
| `GRANT_SECRET_INVALID` | The bearer token does not match this grant. |
| `GRANT_REVOKED` | The grant was revoked. Revocation affects future uses only. |
| `GRANT_EXPIRED` | Past `expires_at`. Issue a new grant. |
| `GRANT_EXHAUSTED` | Every use is spent or reserved. |
| `GRANT_SCOPE` | Wrong project, spec, transition, or production permission. |
| `GRANT_PROHIBITED` | The transition is on the prohibition list. |
| `GRANT_POLICY_STALE` | Config or policy changed since the grant was issued. |
| `GRANT_REASON_REQUIRED` | The grant requires a reason and none was given. |
| `GRANT_REPLAY` | This transition already has a use. |
| `GATE_FAILED` | The readiness gates refused — the same refusal `specd approve` returns. No use consumed. |

## Under the controller

The deterministic controller (`specd brain run`) executes tasks; it does not approve them. When
the last task completes and the spec sits at a lifecycle gate, the run halts with its own outcome
rather than exiting as if it had finished:

```
brain run: waiting_approval  (waiting_approval: lifecycle gate tasks requires human approval; run `specd approve payments`)
APPROVAL_REQUIRED: brain run reached the tasks approval gate after 2 dispatch(es); ...
```

The halt is **non-success**, for the same reason a cost brake is: a caller that reads exit 0 here
would treat an unapproved spec as a completed one. It is recorded on the session and surfaced by
`specd status <spec>`:

```
Controller: waiting_approval at the tasks gate
  approve with `specd approve <spec>`, or with an operator-issued grant via `specd delegate approve <spec> --grant <id> --token <bearer>`
```

**Progress is preserved.** The halt writes one marker beside the session; leases, missions, and the
step counter are untouched. Whichever route clears the gate, the next `brain run` continues from
where it stopped — the two routes converge because both write the same approval record.

### The controller never grants itself anything

`specd brain run <spec> --grant <id>` tells the controller that a grant exists. That is all it
does. The controller:

- is never given the bearer token, so it *cannot* spend a grant even if asked to;
- has no code path that issues, revokes, widens, or consumes one (asserted by a test that walks
  the package source, not by this paragraph);
- names the delegated route only when the supplied grant already covers this exact transition.

A grant that expires or is revoked while the controller waits stops being named, and the human
route stands. The controller never falls back to approving without one.

And gate drift outranks every authority: if the readiness gates refuse, the halt names
`specd check <spec>` and neither approval route is offered, because neither would work.
