# Requirements — agent-driver-protocol

> Phases B, C, and D of docs/agent-specd-communication-analysis.md: consolidate the
> existing primitives into one continuous driver protocol with no ungoverned gap, and
> define the host contract that converts authority into real restrictions.
> Depends on agent-protocol-clarity (assurance, typed refusal, capability contracts).

## R1 — One canonical agent entry point

owner: maintainer
priority: must
risk: high

- R1.1: When an agent host requests the next action for a spec, the system shall return session identity, current revision, assurance level, permitted actor, legal operation envelope, any human-only action, the selected task, the authority packet, the context-manifest digest, deterministic blockers, and the exact recovery operation in one response.
- R1.2: When the canonical entry point is unavailable or the spec is not driveable, the system shall return a typed refusal naming the actor required to unblock.
- R1.3: When granular commands are used instead, the system shall keep them working so existing operators are not broken.

## R2 — Mutable work belongs to a stateful driver session

owner: maintainer
priority: must
risk: critical

- R2.1: When a host begins governed work, the system shall issue a session identity bound to a spec, a driver identity, and a baseline revision.
- R2.2: When a mutable operation is requested, the system shall require the session identity, expected state revision, handshake digest, authority digest, context receipt, baseline revision, and a single-use operation nonce.
- R2.3: When any of those bindings is stale, reused, or mismatched, the system shall refuse the operation without mutating trusted state.
- R2.4: When an operation completes, the system shall invalidate its nonce so the same action cannot be replayed.

## R3 — Required context is acknowledged before authority activates

owner: maintainer
priority: must
risk: high

- R3.1: When a host requests mutable authority, the system shall require a receipt binding the manifest digest, the required items supplied, the missing items, the host-reported token count, and the host and driver identity.
- R3.2: When a required context lane is missing from the receipt, the system shall withhold mutable authority.

## R4 — The whole diff is validated against declared scope

owner: maintainer
priority: must
risk: critical

- R4.1: When verification or completion is requested, the system shall compare the mission baseline against the complete current diff and reject undeclared modified, created, deleted, or renamed files.
- R4.2: When the diff contains changes that predate the mission baseline, the system shall reject the operation.
- R4.3: When the diff manipulates task markers or harness-owned state directly, the system shall reject the operation.
- R4.4: When the diff overlaps the scope of another active lease, the system shall reject the operation.
- R4.5: When the diff-scope comparison runs, the system shall run it as a core invariant on every transport rather than only under the production profile.

## R5 — The host contract converts authority into real restrictions

owner: maintainer
priority: must
risk: critical

- R5.1: When a host has not completed bootstrap, the system shall require that mutable tools remain unavailable to the agent.
- R5.2: When a host exposes an agent surface, the system shall require that human-only operations be absent from it and that harness-owned files be denied to ordinary editors.
- R5.3: When a host invokes a tool, the system shall require the host to check authority expiry at invocation time and to derive path, process, and network permission from the authority packet.
- R5.4: When a host cannot provide those controls, the system shall label the session advisory and shall not present it as fully governed.

## R6 — Multi-agent execution cannot produce overlapping or stale authority

owner: maintainer
priority: should
risk: high

- R6.1: When a driver session is opened, the system shall bind it to the lease that governs its task.
- R6.2: When two dispatched tasks declare overlapping scope, the system shall detect the overlap before dispatch.
- R6.3: When a mission report arrives against a drifted baseline, the system shall reject it as stale.

## R7 — Protocol conformance is observable without weakening enforcement

owner: maintainer
priority: could
risk: low

- R7.1: When an agent attempts work without bootstrap, replays a stale action, acts without authority, touches an undeclared path, invokes a human-only operation, skips context acknowledgement, claims completion before completion, or mutates .specd directly, the system shall record a deterministic protocol event.
- R7.2: When conformance events are recorded, the system shall keep them observational so no gate outcome depends on them.

## Edge and failure behavior

- When a session expires mid-task, the system shall refuse further operations and shall preserve any evidence already recorded.
- When a host crashes without closing a session, the system shall let the lease and session expire rather than requiring manual repair.
- When a legitimate change touches a generated file outside declared paths, the system shall refuse and shall name the declaration change required, rather than offering a bypass.
- When the same agent process drives two specs, the system shall keep sessions and nonces isolated per spec.

## Non-goals

- Adding any completion bypass, override flag, or way to satisfy a gate without evidence.
- Placing an LLM in any decision, gate, or dispatch path.
- Guaranteeing that a model never states something false; the guarantee is that a false statement cannot alter trusted state.
- Shipping adapters for every host in this spec; one reference adapter proves the contract.
