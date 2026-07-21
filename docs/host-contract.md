# Host contract

specd is a CLI. It owns the declaration, the gates, the evidence, and the
lifecycle — but it does not sit in front of the agent's file writes, its
subprocesses, or its network calls. The host does.

That gap is what this document is about. An authority packet says a task may
write `internal/core/session.go` and nothing else. specd can *state* that,
*record* it, and *refuse* a completion whose diff contradicts it. What specd
cannot do is stop the write at the moment it happens. Only the process that owns
the tool call can do that.

So the contract below is not a feature list. It is the boundary where specd stops
being able to enforce anything and starts having to trust — and the reason the
trust is written down is that it becomes visible in the assurance level instead
of being assumed silently.

A host that meets none of this is still usable. It is just **advisory**, and
specd will say so rather than presenting it as governed.

Request routing precedes this contract: repository presence alone keeps the request in
general mode and invokes no specd command. Explicit managed activation starts with
`specd handshake bootstrap <slug> --json`. A mode or managed-spec switch invalidates the
previous authority packet; host adapters must not carry it across routes.

## The controls

A host declares these at bootstrap. Each maps to a clause of `agent-driver-protocol`
R5, and each is a control specd cannot implement on the host's behalf.

| Control | Clause | What the host must do |
|---|---|---|
| `gates_mutable_tools_until_bootstrap` | R5.1 | Keep mutable tools unavailable to the agent until bootstrap completes. An agent that can act before it holds an authority packet is acting under no authority at all. |
| `hides_human_only_operations` | R5.2 | Omit human-only operations from the agent's tool surface entirely. Not "refuse on call" — **absent**. A tool an agent can see is a tool it will eventually try. |
| `denies_harness_owned_paths` | R5.2 | Deny ordinary editors write access to harness-owned files: `.specd/specs/*/tasks.md`, `requirements.md`, `design.md`, and everything under `.specd/roles/` and `.specd/steering/`. |
| `checks_authority_expiry_at_invocation` | R5.3 | Check `expires_at` when the tool is invoked, not when the packet is issued. An expiry checked only at issue time is not an expiry. |
| `derives_path_permission` | R5.3 | Derive which paths are writable from `declared_write_paths` on the packet, not from host configuration. |
| `derives_process_permission` | R5.3 | Derive subprocess permission from the packet's sandbox profile. |
| `derives_network_permission` | R5.3 | Derive network permission from `network_policy` on the packet, which defaults to `deny`. |
| `sandbox` | R5.4 | Isolate execution. This is a **ceiling**, not a control — see below. |

## How assurance resolves

The lattice is `advisory < gated < sandboxed`, and resolution only ever lowers.

- Any control unmet → **advisory**. A host missing one of them enforces nothing
  specd can rely on, so the rest do not add up to containment.
- All controls met, no sandbox → **advisory**. Nothing contains a process that
  ignores the rules it agreed to.
- All controls met, sandbox declared → **sandboxed**.

`sandbox` sits apart from the others deliberately. The seven controls are things
a host *does*; the sandbox is whether anything *stops* a host that does them
wrong. That is why no combination of the first seven can substitute for it, and
why `core.AssuranceFor` caps the declared level by the sandbox capability rather
than averaging them.

This is enforced in `internal/integration/hostcontract.go` and asserted by
`TestHostContractAgreesWithTransportAssurance`, which pins the contract's answer
to the one `internal/mcp` already reports — so the two surfaces cannot drift.

## What this contract cannot do

Stated plainly, because a security document that overstates itself is worse than
none:

- **It does not verify the declaration.** A host asserting
  `derives_path_permission` may be lying or simply wrong. specd records the claim
  and labels the session; it does not audit the host's implementation.
- **It does not make an advisory session unsafe to use.** Advisory means findings
  are reported and nothing is contained. That is a perfectly reasonable way to run
  specd — it is just not the same as governed, and the two must not read alike.
- **It is not the last line of defence.** The diff-scope gate
  (`internal/core/gates/diffscope.go`) runs on every transport and every profile,
  and refuses a completion whose diff exceeds the declared scope regardless of
  what the host claimed. A dishonest declaration still cannot get undeclared work
  marked complete. The contract raises the assurance label; the gate is what
  actually holds.

That last point is the important one. The contract is how a host earns a better
label. It is not how work gets approved.

## Declaring conformance

```go
contract := integration.HostContract{
    GatesMutableToolsUntilBootstrap:   true,
    HidesHumanOnlyOperations:          true,
    DeniesHarnessOwnedPaths:           true,
    ChecksAuthorityExpiryAtInvocation: true,
    DerivesPathPermission:             true,
    DerivesProcessPermission:          true,
    DerivesNetworkPermission:          true,
    Sandbox:                           true,
}

conformance := integration.EvaluateHostContract(contract)
// conformance.Assurance -> "sandboxed"
// conformance.Unmet     -> []
// conformance.Governed  -> true
```

`integration.ReferenceHostContract()` returns exactly this. It ships as the one
reference adapter the spec commits to, and its job is to prove the contract is
satisfiable rather than aspirational — a contract no host can meet is a contract
that gets ignored. `TestHostContractReferenceAdapterIsGoverned` fails if a new
required control is ever added without the reference host asserting it.

Over MCP, the declaration arrives as `driver_capabilities` in the `initialize`
params, and the negotiated result comes back as `assurance` alongside a
`driverCapabilities` report (`internal/mcp/server.go` — note the request field is
snake_case and the response key is camelCase). A host that declares nothing gets
`advisory`, which is the fail-safe: `core.ParseAssuranceLevel` maps an
unrecognized value to advisory rather than guessing upward, so a typo cannot
advertise containment that does not exist.

## Unmet controls are reported, not hidden

`HostConformance.Unmet` names each missing control with its clause — for example
`R5.3:derives_network_permission`. A host that comes back advisory can see which
clause to fix rather than only that something is wrong. The order is stable, so
two runs of a conformance report read alike.

## Route completeness

Status guidance, handshake, and drive project canonical operation metadata
through the current CLI transport before calling an action executable. The
projection checks dispatch availability, lifecycle phase, actor class, and
authority. An unavailable human or host operation appears under `handoffs`;
missing dispatch or authority issuer appears under `route_blockers`. Neither is
an agent-executable next action. In production, task mutations remain withheld
until the current transport has a mission-issued `AuthorityV1`; a nominal
command name is not treated as proof that an issuer exists.
