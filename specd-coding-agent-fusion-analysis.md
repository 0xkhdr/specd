# specd-Coding-Agent Fusion: Context-Engineered Adherence & Zero-Overhead Command Discovery

## Analysis & Action Plan

**Version:** 1.0  
**Date:** 2026-06-27  
**Scope:** Ensure a coding agent always adheres to specd rules, discovers commands/options without error-prone trial-and-error, and respects the full configuration (including Brain/Pinky orchestration) with zero specd-induced overhead.

---

## 1. Executive Summary

The core problem is **context drift**: coding agents forget specd rules mid-session, make incorrect tool calls due to incomplete command knowledge, and fail to respect configuration like `subagentMode: delegate` or `orchestration.enabled`. The solution is a **phase-scoped, constitution-driven context fusion** that embeds specd's steering, skills, and command schema directly into the agent's reasoning loop—never as an afterthought, but as the **primary substrate** upon which all agent actions are built.

**The Foundational Principle:** *The agent reasons. The harness enforces. The fusion layer ensures the agent knows what to reason about and how to invoke the enforcer.*

---

## 2. Deep Analysis of specd Architecture

### 2.1 What specd Is

specd is a **deterministic, agent-agnostic, spec-driven coding harness CLI** written in Go (stdlib only). It enforces:

- **Planning Ratchet:** Requirements → Design → Tasks → Execute → Verify → Reflect
- **Validation Gates:** 7 core gates (EARS, design, task-schema, DAG, evidence, sync, traceability) + opt-in acceptance, scope, custom gates
- **DAG-Based Execution:** Wave-structured concurrent task frontier
- **Evidence-Gated Completion:** Tasks complete only against passing `verify` records
- **Agent-Agnostic Interface:** Standardized CLI + MCP server + role prompts
- **Brain/Pinky Orchestration:** Deterministic multi-agent controller (Brain) + ephemeral worker agents (Pinky)

### 2.2 Critical Integration Points

| Integration Point | Mechanism | Agent Impact |
|---|---|---|
| **Steering Constitution** | `.specd/steering/*.md` | Durable rules that outlive chat sessions |
| **Role Personas** | `.specd/roles/*.md` | investigator, builder, reviewer, verifier, brain, pinky |
| **Skills (Progressive Disclosure)** | `.specd/skills/*/SKILL.md` | Stage-specific knowledge loaded only when needed |
| **Context Manifest** | `specd context <slug> --json` | Budgeted, phase-scoped file list with token hints |
| **Command Schema** | `specd help --json` | Machine-discoverable command/flag/exit-code registry |
| **MCP Tool Surface** | `specd mcp` | JSON-RPC 2.0 server exposing all commands as tools |
| **State Machine** | `state.json` per spec | Single source of truth for status, never hand-edited |
| **Dispatch Packets** | `specd dispatch <slug> --json` | Ready-to-run subagent missions with contextManifest |

### 2.3 The Configuration Surface

The full configuration lives in `.specd/config.json`:

```json
{
  "version": 1,
  "defaultVerify": "npm test",
  "report": { "format": "md", "autoRefreshSeconds": 0 },
  "roles": { "subagentMode": "inline" },  // CRITICAL: inline vs delegate
  "promotionThreshold": 3,
  "gates": {
    "traceability": "warn",
    "acceptance": "off",
    "scope": "off",
    "custom": []
  },
  "verify": { "sandbox": "none" },
  "orchestration": {
    "enabled": false,              // CRITICAL: Brain/Pinky on/off
    "approvalPolicy": "manual",    // manual | planning | session
    "workerMode": "host",
    "maxWorkers": 4,
    "maxRetries": 2,
    "sessionTimeoutMinutes": 120,
    "hostReportedCostLimitUSD": 0,
    "transport": {
      "kind": "file",
      "pollIntervalMillis": 500,
      "messageTTLSeconds": 3600,
      "leaseSeconds": 120,
      "heartbeatSeconds": 30
    },
    "program": { "maxConcurrentSpecs": 2 }
  }
}
```

**Key Insight:** The agent MUST read and respect this file on every session start. The `roles.subagentMode` and `orchestration.enabled` fields are not suggestions—they are binding constraints on how the agent must behave.

### 2.4 The Two Execution Modes

| Mode | How Agent Must Behave | Configuration Trigger |
|---|---|---|
| **Base** (default) | Agent drives every step: `specd next` → implement → `specd verify` → `specd task --status complete` | `orchestration.enabled: false` OR spec `executionMode: base` |
| **Orchestrated** | Agent MUST use Brain/Pinky protocol: `specd brain start/step` → dispatch → Pinky worker claim → heartbeat → verify → report | `orchestration.enabled: true` AND spec `executionMode: orchestrated` |

**Critical Rule:** Brain/Pinky commands **refuse** Base specs. A spec cannot switch back to Base while a Brain session is live. The agent must NEVER attempt to mix Base commands with Orchestrated commands.

---

## 3. Problem Diagnosis: Why Agents Fail to Adhere

### 3.1 Root Causes of Non-Adherence

1. **Steering Amnesia:** Agent forgets `.specd/steering/` files exist or loads them incompletely
2. **Phase Blindness:** Agent doesn't know which phase it's in and uses wrong commands (e.g., `specd next` during planning)
3. **Command Schema Ignorance:** Agent guesses flags/arguments instead of querying `specd help --json`
4. **Configuration Disrespect:** Agent ignores `subagentMode: delegate` and performs inline work, or ignores `orchestration.enabled: true` and uses Base mode
5. **Context Bloat:** Agent loads entire spec history instead of phase-scoped minimal context
6. **Evidence Gate Bypass:** Agent marks tasks complete without running `specd verify`
7. **Role Confusion:** Agent uses `builder` permissions when assigned `investigator`

### 3.2 The Error Spiral

```
Agent guesses command → Wrong flags → Error → Retries with variation → More errors
→ Context wasted on debugging → Agent abandons specd → Work proceeds unstructured
→ specd check fails → Agent confused → More errors
```

**The fix:** Eliminate guessing by making the agent **constitutionally incapable** of issuing a specd command without first consulting the schema and phase briefing.

---

## 4. The Solution: Context-Engineered Fusion Architecture

### 4.1 Design Philosophy

We introduce a **Specd Fusion Layer**—not a new tool, but a **set of context engineering rules** that bind the agent to specd with the same rigor specd binds code to specs. The layer has three pillars:

1. **Constitutional Priming:** specd steering is loaded before ANY user request is processed
2. **Phase-Gated Command Discovery:** The agent only knows the commands relevant to the current phase
3. **Zero-Overhead Context Budgeting:** Every byte loaded is justified by the current phase and role

### 4.2 The Fusion Layer: Three Tiers

```
┌─────────────────────────────────────────────────────────────┐
│  TIER 1: CONSTITUTIONAL PRIMING (Always Loaded)             │
│  • .specd/steering/reasoning.md                             │
│  • .specd/steering/workflow.md                              │
│  • .specd/config.json (full config)                         │
│  • specd help --json (command schema cache)                  │
│  • AGENTS.md (agent workflow guide)                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  TIER 2: PHASE-SCOPED SKILL INJECTION (Loaded on Transition)│
│  • specd context <slug> --json (contextManifest)            │
│  • .specd/skills/specd-<phase>/SKILL.md                      │
│  • .specd/roles/<role>.md (if role assigned)                 │
│  • .specd/steering/memory.md (EXECUTE + REFLECT only)        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  TIER 3: COMMAND DISCOVERY & VALIDATION (Per-Invocation)    │
│  • specd help <command> --json (before any unfamiliar cmd)   │
│  • specd check <slug> (before claiming phase complete)       │
│  • specd schema (for structural validation)                  │
└─────────────────────────────────────────────────────────────┘
```

---

## 5. Implementation: The Adherence Algorithm

### 5.1 Algorithm: `SPECDFUSION` (Session Initialization)

```
FUNCTION InitializeSpecdSession(projectRoot):

    // Step 1: Constitutional Priming (ALWAYS)
    LOAD .specd/steering/reasoning.md
    LOAD .specd/steering/workflow.md
    LOAD .specd/config.json → PARSE INTO configMap

    // Step 2: Cache Command Schema (ALWAYS)
    RUN "specd help --json" → CACHE AS commandSchema

    // Step 3: Detect Execution Mode (CRITICAL)
    IF config.orchestration.enabled == true:
        SET globalMode = "orchestration-capable"
    ELSE:
        SET globalMode = "base-only"

    // Step 4: Detect Subagent Mode (CRITICAL)
    IF config.roles.subagentMode == "delegate":
        SET subagentMode = "delegate"
        ENSURE host supports subagent spawning
    ELSE:
        SET subagentMode = "inline"

    // Step 5: Verify specd health
    RUN "specd doctor" → IF exit != 0 THEN RUN "specd doctor --fix"

    RETURN {configMap, commandSchema, globalMode, subagentMode}
END FUNCTION
```

### 5.2 Algorithm: `PHASEBRIEF` (Per-Spec Context Loading)

```
FUNCTION LoadPhaseContext(specSlug, sessionState):

    // Step 1: Get phase-scoped briefing from specd itself
    RUN "specd context <specSlug> --json" → contextManifest

    // Step 2: Load ONLY the files in contextManifest.LOAD_NOW
    FOR EACH item IN contextManifest.items:
        IF item.required == true AND item.mode == "read-full":
            LOAD item.path
        ELSE IF item.mode == "read-targeted":
            LOAD item.sliceOnly  // bounded window, not full file
        ELSE IF item.mode == "run-command":
            EXECUTE item.command → CACHE output
        ELSE IF item.mode == "reference-if-needed":
            NOTE item.path AS available but do not load yet

    // Step 3: Load phase-specific skill
    phase = DERIVE_PHASE_FROM(specSlug state.json)
    LOAD .specd/skills/specd-<phase>/SKILL.md

    // Step 4: Load role-specific skill if applicable
    IF activeRole IS NOT NULL:
        LOAD .specd/roles/<activeRole>.md
        IF subagentMode == "delegate" AND activeRole != "brain":
            PREPARE subagent spawn with role context

    // Step 5: Load memory ONLY in EXECUTE/REFLECT phases
    IF phase IN ["execute", "verify", "reflect"]:
        LOAD .specd/steering/memory.md

    RETURN {loadedArtifacts, phase, activeRole, tokenBudget: contextManifest.budget}
END FUNCTION
```

### 5.3 Algorithm: `COMMANDDISCOVER` (Zero-Error Command Invocation)

```
FUNCTION InvokeSpecdCommand(commandName, arguments, specSlug):

    // Step 1: Schema lookup (NEVER guess)
    commandMeta = LOOKUP commandSchema[commandName]
    IF commandMeta IS NULL:
        ERROR "Unknown specd command. Available commands: " + LIST_KEYS(commandSchema)

    // Step 2: Validate arguments against schema
    FOR EACH arg IN arguments:
        IF arg NOT IN commandMeta.validFlags AND arg NOT IN commandMeta.validPositionals:
            ERROR "Invalid argument '" + arg + "' for command '" + commandName + "'. Valid flags: " + commandMeta.flags

    // Step 3: Phase compatibility check
    phase = GET_CURRENT_PHASE(specSlug)
    IF commandName IN phaseIncompatibleCommands[phase]:
        ERROR "Command '" + commandName + "' is incompatible with phase '" + phase + "'. Expected commands: " + phaseCompatibleCommands[phase]

    // Step 4: Mode compatibility check (CRITICAL for Brain/Pinky)
    executionMode = GET_SPEC_EXECUTION_MODE(specSlug)
    IF executionMode == "base" AND commandName STARTS_WITH "brain":
        ERROR "Brain commands refused: spec '" + specSlug + "' is in Base mode. Use 'specd mode <slug> --set orchestrated' first."
    IF executionMode == "orchestrated" AND commandName IN ["next", "dispatch"]:
        WARN "Spec is orchestrated. Prefer 'specd brain step' over direct 'specd next'."

    // Step 5: Build and execute
    cmdLine = BUILD_COMMAND(commandName, arguments)
    result = EXECUTE(cmdLine)

    // Step 6: Exit code interpretation (never guess)
    IF result.exitCode == 0:
        RETURN {success: true, output: result.stdout}
    ELSE IF result.exitCode == 1:
        RETURN {success: false, errorType: "gate_failure", details: result.stderr}
    ELSE IF result.exitCode == 2:
        RETURN {success: false, errorType: "usage_error", details: result.stderr}
    ELSE IF result.exitCode == 3:
        RETURN {success: false, errorType: "not_found", details: result.stderr}

    RETURN result
END FUNCTION
```

### 5.4 Algorithm: `ORCHESTRATIONADHERENCE` (Brain/Pinky Protocol)

```
FUNCTION AdhereToOrchestration(specSlug, config):

    // CRITICAL: Verify orchestration is enabled project-wide
    IF config.orchestration.enabled != true:
        ERROR "Orchestration not enabled in config. Run 'specd init --orchestration planning' first."

    // CRITICAL: Verify spec is in orchestrated mode
    mode = RUN "specd mode <specSlug> --json"
    IF mode.executionMode != "orchestrated":
        ERROR "Spec not in orchestrated mode. Run 'specd mode <specSlug> --set orchestrated' first."

    // CRITICAL: Respect subagentMode
    IF config.roles.subagentMode == "delegate":
        // Agent MUST spawn subagents, not do work inline
        SET protocol = "delegate-subagent"
    ELSE:
        SET protocol = "inline-host"

    // Step 1: Start Brain session
    brainResult = RUN "specd brain start <specSlug> --approval-policy <policy> --max-workers <n> --max-retries <n> --timeout-seconds <n> --json"
    sessionID = brainResult.sessionID

    // Step 2: Main orchestration loop
    WHILE true:
        stepResult = RUN "specd brain step <specSlug> --session <sessionID> --json"

        IF stepResult.action == "complete-session":
            BREAK
        ELSE IF stepResult.action == "escalate" OR stepResult.action == "policy-violation":
            ERROR "Orchestration blocked: " + stepResult.reason
            BREAK
        ELSE IF stepResult.action == "awaiting-approval":
            HALT "Human approval required. Run 'specd approve <specSlug>' when ready."
            BREAK
        ELSE IF stepResult.action == "dispatch":
            mission = stepResult.mission

            IF protocol == "delegate-subagent":
                // SPAWN SUBAGENT with mission context
                subagent = SPAWN_SUBAGENT(role=mission.role, contextManifest=mission.contextManifest)
                subagent.EXECUTE_PINKY_PROTOCOL(mission, sessionID)
            ELSE:
                // Host performs Pinky protocol inline
                EXECUTE_PINKY_PROTOCOL(mission, sessionID)
        ELSE IF stepResult.action == "wait":
            SLEEP(config.orchestration.transport.pollIntervalMillis / 1000)

    RETURN "Orchestration complete"
END FUNCTION

FUNCTION EXECUTE_PINKY_PROTOCOL(mission, sessionID):
    // Step 1: Claim
    RUN "specd pinky claim --mission <mission.json> --json"

    // Step 2: Heartbeat loop
    WHILE working:
        RUN "specd pinky heartbeat --session <sessionID> --worker <mission.workerID> --attempt 1 --json"
        SLEEP(config.orchestration.transport.heartbeatSeconds)

    // Step 3: Implement work (agent's creative work)
    // ... implement task ...

    // Step 4: Verify (EVIDENCE GATE - NEVER SKIP)
    verifyResult = RUN "specd verify <mission.spec> <mission.taskID> --json"
    IF verifyResult.exitCode != 0:
        RUN "specd pinky block --session <sessionID> --worker <mission.workerID> --spec <mission.spec> --task <mission.taskID> --attempt 1 --reason 'Verification failed' --json"
        RETURN "BLOCKED"

    // Step 5: Report
    RUN "specd pinky report --session <sessionID> --worker <mission.workerID> --spec <mission.spec> --task <mission.taskID> --attempt 1 --verification-ref <verifyResult.ref> --summary 'Done' --json"

    // Step 6: Release
    RUN "specd pinky release --session <sessionID> --worker <mission.workerID> --attempt 1 --json"
    RETURN "COMPLETE"
END FUNCTION
```

---

## 6. Context Engineering Rules for Zero Overhead

### 6.1 The Context Budget Equation

```
Total Context Window = System Prompt + specd Constitution + Phase Skill + Role + Active Spec Artifacts + Working Memory

specd Fusion Target: Constitution + Phase + Role + Targeted Artifacts ≤ 30% of context window
```

### 6.2 Progressive Disclosure Rules

| Rule | Description | Rationale |
|---|---|---|
| **R1: Never Load Full History** | Only load `memory.md` in EXECUTE/REFLECT phases | Prevents context bloat during planning |
| **R2: Slice, Don't Load** | Use `read-targeted` mode for large files (tasks.md, requirements.md) | Only load the relevant task row or requirement |
| **R3: Skill Before Stage** | Read `specd-<phase>` skill ONLY when entering that phase | Skills are designed for progressive disclosure |
| **R4: Role Isolation** | In `delegate` mode, subagent loads ONLY its role + task context | Host agent sheds worker context immediately |
| **R5: Schema Over Memory** | Query `specd help --json` instead of recalling commands from training | Eliminates hallucinated flags/arguments |
| **R6: Context Manifest Authority** | Treat `specd context --json` output as the **exclusive** load list | The manifest is the single source of truth for what to load |

### 6.3 The `specd context` Manifest as the Load Oracle

The `specd context <slug> --json` output contains a `contextManifest` with:

```json
{
  "contextManifest": {
    "version": 1,
    "estimatedTokens": 5400,
    "budget": 12000,
    "items": [
      {
        "order": 1,
        "kind": "role",
        "mode": "read-full",
        "required": true,
        "tokenHint": 1200,
        "rationale": "Builder role contract for this task"
      },
      {
        "order": 2,
        "kind": "skill",
        "mode": "read-full",
        "required": true,
        "tokenHint": 800,
        "rationale": "specd-execute skill for this phase"
      },
      {
        "order": 3,
        "kind": "artifact",
        "mode": "read-targeted",
        "required": true,
        "tokenHint": 400,
        "rationale": "Task T1 row from tasks.md only"
      }
    ]
  }
}
```

**Agent Rule:** The agent MUST load items in `order`, respecting `mode`. If `estimatedTokens > budget`, the agent MUST call `specd brain compact` or request human guidance—never proceed with overloaded context.

---

## 7. Configuration Respect Matrix

### 7.1 Mandatory Configuration Checks

Before ANY action, the agent MUST verify these configuration fields:

| Config Field | Agent Obligation | Violation Consequence |
|---|---|---|
| `roles.subagentMode` | If `delegate`, spawn subagents per role. If `inline`, switch personas inline. | Context bloat or incorrect permission model |
| `orchestration.enabled` | If `true`, orchestration commands are available. If `false`, Brain/Pinky commands MUST NOT be used. | Command refusal (exit 1) |
| `orchestration.approvalPolicy` | `manual` = human approval required. `planning` = auto-advance planning gates. `session` = auto-advance within session. | Policy violation, session escalation |
| `gates.*` | Respect `warn` vs `error` for traceability, acceptance, scope. | `specd check` fails unexpectedly |
| `verify.sandbox` | If `bwrap`/`container`, use `--sandbox` flag. | Security bypass |
| `defaultVerify` | Use as fallback verify command when task lacks explicit `verify:` | Verification gap |

### 7.2 The `subagentMode` Adherence Protocol

```
IF config.roles.subagentMode == "delegate":
    // Agent MUST use host-native subagent spawning
    FOR EACH task IN frontier:
        packet = specd dispatch <slug> --json
        FOR EACH mission IN packet.missions:
            subagent = HOST_SPAWN_SUBAGENT()
            subagent.context = mission.contextManifest
            subagent.role = mission.role
            subagent.instructions = "Follow specd pinky protocol. Run claim → heartbeat → work → verify → report → release."
            subagent.RUN()
    // Host agent monitors via specd brain step, does NOT implement tasks inline
ELSE:
    // Agent switches personas inline
    FOR EACH task IN frontier:
        persona = LOAD_ROLE(task.role)
        SWITCH_CONTEXT_TO(persona)
        IMPLEMENT(task)
        RUN specd verify
        RUN specd task --status complete
```

---

## 8. Command Discovery Mechanism: The `help --json` Cache

### 8.1 Why Agents Guess (And Why They Mustn't)

Agents often guess command syntax because:
1. They don't know `specd help --json` exists
2. They load help text but not the structured schema
3. They forget the schema between tool calls

### 8.2 The Discovery Cache

```
ON_SESSION_START:
    schema = specd help --json
    CACHE schema in agent's working memory

BEFORE_EVERY_SPECd_COMMAND:
    IF command NOT_IN cache OR arguments uncertain:
        subSchema = specd help <command> --json
        VALIDATE arguments against subSchema.flags and subSchema.positionals

    IF validation fails:
        ERROR with exact valid options from schema
        DO NOT execute
```

### 8.3 Exit Code Interpretation Cache

| Exit Code | Meaning | Agent Action |
|---|---|---|
| `0` | Success | Proceed |
| `1` | Gate/Validation Failure | Run `specd check <slug>` to diagnose. Do not retry same command blindly. |
| `2` | Usage Error | Re-consult `specd help <command> --json`. Fix arguments. |
| `3` | Not Found | Verify `.specd/` root exists (`specd init`). Verify spec slug exists (`specd new`). |

---

## 9. Phase-Aligned Command Reference

To prevent phase-incompatible commands, the agent MUST internalize this mapping:

| Phase | Valid Commands | Forbidden Commands |
|---|---|---|
| **requirements** (analyze) | `specd check`, `specd approve`, `specd context`, `specd status`, `specd waves` | `specd next`, `specd verify`, `specd task`, `specd brain` (unless orchestrating planning) |
| **design** (plan) | `specd check`, `specd approve`, `specd context`, `specd status` | `specd next`, `specd verify`, `specd task` |
| **tasks** (plan) | `specd check`, `specd approve`, `specd context`, `specd status`, `specd waves` | `specd next`, `specd verify`, `specd task` |
| **executing** (execute) | `specd next`, `specd dispatch`, `specd verify`, `specd task`, `specd brain` (if orchestrated), `specd pinky` | `specd approve` (except for final close) |
| **verifying** (verify) | `specd verify`, `specd check`, `specd approve` (final), `specd report` | `specd next`, `specd task` |
| **complete** (reflect) | `specd report`, `specd memory`, `specd replay` | All execution commands |

---

## 10. Action Plan: Implementation Steps

### Phase A: Foundation (Day 1)

1. **Implement `SPECDFUSION` initialization** in the coding agent's session startup
   - Add mandatory `.specd/steering/*.md` loading before any user prompt processing
   - Cache `specd help --json` in agent's persistent session state
   - Parse `.specd/config.json` into a structured config object accessible to all agent reasoning

2. **Implement configuration sentinel**
   - Before every tool call, check `config.json` for relevant constraints
   - If `orchestration.enabled: true`, route execution through Brain/Pinky protocol
   - If `subagentMode: delegate`, enforce subagent spawning for all role-bound tasks

3. **Implement phase detector**
   - Derive current phase from `specd status <slug> --json` or `state.json`
   - Load phase-compatible command whitelist
   - Block phase-incompatible commands with explanatory error

### Phase B: Command Discovery (Day 2-3)

4. **Implement schema-validated command builder**
   - Create a helper that takes command name + arguments, validates against cached schema
   - Auto-lookup `specd help <command> --json` on cache miss
   - Return structured error with valid options on validation failure
   - Never emit a command without schema validation

5. **Implement exit code interpreter**
   - Wrap all specd invocations in a handler that maps exit codes to agent actions
   - Exit 1 → auto-run `specd check` for diagnosis
   - Exit 2 → auto-consult help schema
   - Exit 3 → auto-verify `.specd/` initialization

### Phase C: Orchestration Adherence (Day 4-5)

6. **Implement Brain/Pinky protocol handler**
   - Create dedicated orchestration loop that wraps `specd brain start/step`
   - Implement Pinky worker lifecycle: claim → heartbeat → work → verify → report → release
   - Enforce evidence gate: verify MUST pass before report is accepted
   - Handle all Brain decisions: dispatch, wait, awaiting-approval, escalate, complete-session

7. **Implement subagent dispatcher**
   - If `subagentMode: delegate`, create subagent spawn logic using `specd dispatch --json` packets
   - Each subagent receives its `contextManifest` + role + mission
   - Host agent monitors via `specd brain step`, never implements tasks inline

### Phase D: Context Optimization (Day 6-7)

8. **Implement `specd context` manifest loader**
   - Replace ad-hoc file loading with `specd context <slug> --json` as the exclusive load oracle
   - Respect `mode` (read-full, read-targeted, run-command, reference-if-needed)
   - Enforce token budget: if `estimatedTokens > budget`, trigger compaction or halt

9. **Implement progressive skill disclosure**
   - Load `specd-foundations` once per session
   - Load `specd-steering` after init, before first spec
   - Load `specd-requirements`/`design`/`tasks`/`execute`/`brain`/`pinky` ONLY when entering that phase
   - Unload previous phase skills to free context

### Phase E: Validation & Hardening (Day 8-10)

10. **Implement self-check protocol**
    - After every phase transition, auto-run `specd check <slug>`
    - Before every task completion, verify `specd verify` record exists
    - Before every `specd approve`, verify all gates pass

11. **Implement configuration drift detection**
    - Hash `.specd/config.json` on session start
    - Re-check hash before major operations
    - If changed, re-run `SPECDFUSION` initialization

12. **Implement error recovery playbook**
    - Create decision tree for each exit code + error message combination
    - Auto-suggest next action based on `specd doctor` output
    - Never retry same failed command more than once without schema re-validation

---

## 11. Best Practices Summary

### The Golden Rules

1. **Constitution First:** Load steering before any user request. specd rules are not optional.
2. **Schema Before Syntax:** Query `specd help --json` before guessing flags. The schema is the source of truth.
3. **Context Manifest Authority:** `specd context --json` decides what you load. Nothing else.
4. **Phase Gates Are Real:** Never use `specd next` in planning. Never use `specd approve` in execution.
5. **Evidence Is Mandatory:** A task without a passing `specd verify` is not complete. Ever.
6. **Configuration Is Binding:** `subagentMode: delegate` means you spawn subagents. `orchestration.enabled: true` means you use Brain/Pinky.
7. **Exit Codes Tell Stories:** 0=ok, 1=gate fail, 2=usage error, 3=not found. Act accordingly.
8. **Memory Is Phase-Scoped:** Load `memory.md` only in EXECUTE/REFLECT. Keep planning clean.
9. **Skills Are Progressive:** Read the skill for your current phase, not all phases.
10. **The Harness Enforces:** You reason. specd enforces. Your job is to know the rules so specd can do its job.

### The Anti-Patterns (Never Do These)

- ❌ Hand-edit `state.json` or `tasks.md` checkboxes
- ❌ Mark a task complete without `specd verify` (or `--unverified --evidence` for read-only roles)
- ❌ Use `specd brain` on a Base-mode spec
- ❌ Guess command flags instead of querying `specd help --json`
- ❌ Load all steering files at once regardless of phase
- ❌ Ignore `subagentMode: delegate` and do work inline
- ❌ Skip `specd check` before claiming a phase is complete
- ❌ Retry a failed command with random flag variations
- ❌ Load full `tasks.md` when `read-targeted` mode gives you just the task row
- ❌ Forget to heartbeat a Pinky lease

---

## 12. Verification Checklist

To confirm the fusion layer is working:

- [ ] Agent loads `.specd/steering/*.md` on every session start
- [ ] Agent caches `specd help --json` and consults it before unfamiliar commands
- [ ] Agent respects `config.json` `subagentMode` and `orchestration.enabled`
- [ ] Agent uses `specd context <slug> --json` as the exclusive load oracle
- [ ] Agent loads phase-specific skills only when entering that phase
- [ ] Agent never issues a phase-incompatible command
- [ ] Agent never marks a task complete without verification evidence
- [ ] Agent runs `specd check` before every `specd approve`
- [ ] Agent correctly interprets all four exit codes (0, 1, 2, 3)
- [ ] Agent follows Brain/Pinky protocol when `executionMode: orchestrated`
- [ ] Agent spawns subagents when `subagentMode: delegate`
- [ ] Agent sends Pinky heartbeats within lease timeout
- [ ] Agent handles Brain decisions: dispatch, wait, awaiting-approval, escalate, complete-session
- [ ] Agent compacts context when budget is exceeded
- [ ] Agent detects configuration drift and re-initializes

---

## 13. Conclusion

The specd-coding-agent fusion is not achieved by adding more tools or wrappers. It is achieved by **context engineering**: making the agent's reasoning substrate identical to specd's constitution, phase system, and command schema. When the agent's context window IS the specd workflow, adherence becomes automatic, command discovery becomes deterministic, and overhead drops to zero because every byte loaded is justified by the current phase and role.

**The agent does not use specd. The agent IS specd-aware.**
