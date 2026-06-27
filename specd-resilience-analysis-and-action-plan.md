# specd Resilience & Crash Recovery Analysis
## Action Plan: Zero-Bottleneck Status Restoration for Brain/Pinky Orchestration

**Repository:** [0xkhdr/specd](https://github.com/0xkhdr/specd)  
**Analysis Date:** 2026-06-27  
**Scope:** Brain/Pinky multi-agent orchestration layer, session persistence, context management, and usage-limit resilience.

---

## 1. Executive Summary

specd is a spec-driven coding harness CLI with a deterministic Brain/Pinky orchestration architecture. It already possesses **strong foundational resilience** (file-backed ACP, CAS state writes, lease expiry, session persistence, and context compaction). However, **genuine crash-to-continuation resilience**—where a host agent crash, token limit exhaustion, or rate-limit block is a non-event rather than a bottleneck—requires architectural augmentation in four domains:

1. **Proactive Checkpointing & Context Cryonics** — automatic savepoints before limits
2. **Host-Agnostic Session Resumption** — one-command restore after any interruption
3. **Usage-Limit Graceful Degradation** — token/rate-limit awareness with automatic pause-and-resume
4. **Worker State Serialization** — Pinky workers must be able to hibernate and thaw mid-task

This document analyzes the current architecture, identifies exact gaps, and provides a prioritized action plan with implementation paths.

---

## 2. Current Architecture Analysis

### 2.1 What specd Already Does Well

| Capability | Implementation | Resilience Value |
|---|---|---|
| **File-backed ACP** | All Brain↔Pinky communication persists as JSON envelopes in `.specd/subagents/` | Messages survive process death |
| **CAS State Writes** | `state.json` uses revision-number compare-and-swap via `SaveState()` | No lost updates on concurrent access |
| **Advisory Locking** | `WithSpecLock()` with 30s stale reclamation (`SPECD_LOCK_STALE_MS`) | Dead processes don't permanently lock specs |
| **Lease Expiry** | `leaseSeconds` (default 120s) + heartbeat (30s); unclaimed leases are reclaimed | Crashed workers are auto-rescheduled |
| **Session Persistence** | `session.json` stores `OrchestrationSession` with `ContextLedger` | Token history and compaction points survive |
| **Context Compaction** | `brain compact` / `clear` writes phase summaries + ledger checkpoints | Host can `/clear` and continue with summary |
| **Worker Failure Recording** | `recordWorkerFailure()` marks `TimedOut=true`, increments `Retries` | Retry policy applies automatically on next step |
| **Session Resumption** | `brainRunSession()` auto-detects active sessions; `brain run` resumes | Re-running `brain run` continues existing session |
| **Replay & Diff** | `specd replay` reconstructs event timeline from on-disk records | Full audit trail for forensic recovery |
| **Deterministic Decisions** | `DecideOrchestration()` is pure function of (snapshot, policy) | Same state always yields same decision |

### 2.2 The Foundational Split (Why This Matters)

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   HOST AGENT    │◄───►│  specd BRAIN    │◄───►│  Pinky Workers  │
│  (LLM/Context)  │     │ (Deterministic) │     │ (Ephemeral)     │
│                 │     │                 │     │                 │
│ • Holds context │     │ • Holds state   │     │ • Hold lease    │
│ • Spawns workers│     │ • Makes decisions│    │ • Do creative   │
│ • May crash     │     │ • Never calls LLM│    │   work          │
│ • Has token     │     │ • Survives crash │    │ • May crash     │
│   limits        │     │   by design      │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

**The Brain is already crash-resilient.** The vulnerability is the **Host Agent** (where the LLM context lives) and the **Pinky Worker** (where creative work happens). When these crash, the current recovery is **reactive** (re-run `brain run`, wait for lease expiry, lose in-flight context). The goal is to make it **proactive** (predict, checkpoint, resume seamlessly).

---

## 3. Gap Analysis: Where Crashes Become Bottlenecks

### 3.1 Gap R1: No Proactive Token-Limit Checkpointing

**Current behavior:** Context compaction (`brain compact`) is either manual or triggered by phase boundaries / budget thresholds. It does **not** react to the host's actual remaining token window.

**Failure mode:** Host agent (Claude Code, Cursor, etc.) hits its context limit mid-task. The entire conversation context is lost. The worker cannot report back. The lease expires. The task is retried from scratch with a fresh worker that has **no memory** of the partial implementation.

**Bottleneck severity:** HIGH — Every token-limit crash wastes the entire task's progress and consumed tokens.

### 3.2 Gap R2: No Host Context Serialization

**Current behavior:** The `ContextLedger` tracks token estimates and host-reported actuals, but it does **not** capture the host's actual conversation state, file buffers, or working memory.

**Failure mode:** After a host restart, the resumed session has the Brain state (what task is running) but the **host has no idea what was being discussed**. The `specd context` command rebuilds a manifest, but the agent must re-read everything and reconstruct mental state.

**Bottleneck severity:** HIGH — Re-contextualization cost is O(n) with spec complexity.

### 3.3 Gap R3: No Graceful Rate-Limit Pause

**Current behavior:** If the host hits a provider rate limit, the worker cannot heartbeat. The lease expires. The Brain marks it as a retryable failure.

**Failure mode:** Rate limits (common with Claude API, OpenAI tier limits) cause **false-positive task failures**. The task is retried with a fresh worker, consuming **additional** rate-limit quota and tokens.

**Bottleneck severity:** MEDIUM-HIGH — Retry storms amplify rate-limit problems.

### 3.4 Gap R4: No Worker Hibernate/Thaw Protocol

**Current behavior:** A Pinky worker is ephemeral. It claims a lease, does work, verifies, reports. There is no mechanism for a worker to say *"I am 70% done, my context is full, I need to save my progress and resume with a fresh context window."*

**Failure mode:** Long tasks (e.g., refactoring a large module) cannot be paused mid-flight. If the worker's context fills, all progress is lost.

**Bottleneck severity:** MEDIUM — Affects large tasks; smaller tasks fit within typical context windows.

### 3.5 Gap R5: No Automatic Session Resumption on Host Restart

**Current behavior:** `brain run` can resume an active session, but **someone must run it**. If the host agent crashes and restarts, there is no auto-discovery of "I was in the middle of orchestrating spec X."

**Failure mode:** After a host IDE/extension restart, the agent starts fresh. The user must manually remember which spec was active and re-run `brain run`.

**Bottleneck severity:** MEDIUM — Requires human intervention; breaks autonomous agent loops.

### 3.6 Gap R6: `MaxSteps` / `MaxWaits` Stall Without Telemetry

**Current behavior:** `DriveOrchestration` has `MaxSteps=100` and `MaxWaits=8`. If a worker is genuinely making progress but is slow (e.g., large compilation, long tests), the driver may stall or max-step out.

**Failure mode:** The driver loop exits with `DriverStalled` or `DriverMaxSteps`. The host must detect this and restart. There is no "extend session" or "I see you're still working" signal.

**Bottleneck severity:** LOW-MEDIUM — Affects very long-running tasks; default limits are generous.

---

## 4. Action Plan

### Priority Matrix

| Priority | Initiative | Impact | Effort | Owner |
|---|---|---|---|---|
| P0 | **Checkpoint Protocol** (R1, R4) | Eliminates token-limit data loss | Medium | specd core + host adapter |
| P0 | **Auto-Resume Hook** (R5) | Zero-intervention restart | Low | Host adapter / MCP layer |
| P1 | **Rate-Limit Aware Lease** (R3) | Prevents retry storms | Medium | specd core |
| P1 | **Context Manifest Snapshot** (R2) | O(1) re-contextualization | Medium | specd core |
| P2 | **Progress-Weighted MaxWaits** (R6) | Long-task tolerance | Low | specd core |
| P2 | **Cross-Spec Session Recovery** | Multi-spec program resilience | Medium | specd core |

---

## 5. Detailed Implementation Specifications

### 5.1 P0: Checkpoint Protocol (`specd brain checkpoint` / `specd pinky checkpoint`)

**Goal:** Allow a Pinky worker to serialize its partial progress, release its lease, and have a future worker resume from the exact same point with a fresh context window.

#### 5.1.1 Data Model: `CheckpointRecord`

```go
// Add to internal/core/orchestration.go

 type CheckpointRecord struct {
   CheckpointID   string            `json:"checkpointId"`
   SessionID      string            `json:"sessionId"`
   WorkerID       string            `json:"workerId"`
   Spec           string            `json:"spec"`
   TaskID         string            `json:"taskId"`
   Attempt        int               `json:"attempt"`
   CreatedAt      string            `json:"createdAt"`
   // What the worker was doing
   ProgressPercent int              `json:"progressPercent"`
   Summary        string            `json:"summary"`        // "Implemented AuthService.Login; tests failing on JWT signing"
   // Files that have been modified but not yet verified
   ChangedFiles   []string          `json:"changedFiles"`
   GitHead        string            `json:"gitHead"`
   // Context the next worker needs
   ContextManifest contextpkg.MissionContextManifest `json:"contextManifest,omitempty"`
   // Working notes (free-form, for the next worker's benefit)
   WorkingNotes   string            `json:"workingNotes,omitempty"`
   // Verification state at checkpoint time
   PartialVerify  *VerificationRecord `json:"partialVerify,omitempty"`
 }
```

#### 5.1.2 CLI Surface

```bash
 # Worker calls this when it senses token pressure or is asked to checkpoint
 specd pinky checkpoint    --session <id> --worker <id> --spec <slug> --task <id> --attempt <n>    --progress <0-100> --summary "..."    --changed-files "file1.go,file2_test.go"    --working-notes "Need to fix JWT alg mismatch in line 45"    --git-head <sha>

 # Brain recognizes checkpoint on next step and emits "resume-from-checkpoint" decision
 # instead of dispatching fresh

 # Host can also force a checkpoint for all active workers
 specd brain checkpoint --session <id> --reason "host-context-limit"
```

#### 5.1.3 Brain Decision: `OrchestrationResume`

```go
 const OrchestrationResume OrchestrationAction = "resume-from-checkpoint"
```

When the Brain senses a `CheckpointRecord` for a task with no active lease:
- Action = `resume-from-checkpoint`
- Mission includes the checkpoint's `ContextManifest` + `WorkingNotes`
- The new worker starts with: *"You are resuming task T3 from a checkpoint. Previous worker reached 70%. Working notes: ... Changed files: ... Git HEAD: ... Your job is to continue from here, not restart."*

#### 5.1.4 Host Integration Pattern

```python
 # In the host worker loop
 def run_pinky(mission):
     # ... do work ...
     if host_sensing_token_pressure():
         # Save partial progress
         specd_pinky_checkpoint(...)
         # Gracefully exit; next worker will resume
         return
     # ... complete normally ...
 ```

#### 5.1.5 Implementation Path

1. **Phase 1:** Add `CheckpointRecord` to `orchestration.go`, add `pinky checkpoint` command to `cmd/pinky.go`, persist to `.specd/subagents/sessions/<session>/checkpoints/<task>-<attempt>.json`.
2. **Phase 2:** Modify `SenseOrchestration` to check for checkpoints before offering a fresh dispatch. If checkpoint exists and no lease, emit `resume-from-checkpoint`.
3. **Phase 3:** Modify `BuildPinkyMission` to include checkpoint data in the mission brief when resuming.
4. **Phase 4:** Host adapter integration — teach Claude Code / Cursor subagents to call `specd pinky checkpoint` before `/clear` or on token warnings.

---

### 5.2 P0: Auto-Resume Hook (`specd brain resume` + host auto-detect)

**Goal:** When a host agent restarts, it automatically discovers and resumes any active orchestration sessions without human intervention.

#### 5.2.1 specd Core Changes

```bash
 # New command: list all resumable sessions
 specd brain resume --list --json
 # Returns: [{"sessionID": "...", "spec": "...", "status": "running", "pausedSince": "...", "lastDecision": "..."}]

 # Resume a specific session (idempotent)
 specd brain resume --session <id> --json
 # Equivalent to: specd brain step --session <id> ... (reconstructs policy from session.json)
```

#### 5.2.2 Host Auto-Detect Contract

Add to `.specd/config.json`:

```json
 {
   "orchestration": {
     "autoResume": {
       "enabled": true,
       "onHostStart": true,
       "maxAgeMinutes": 60
     }
   }
 }
```

**Host adapter behavior:**
1. On startup, host runs `specd brain resume --list --json`.
2. If any session has `status == "running"` and `updatedAt` within `maxAgeMinutes`, host auto-invokes `specd brain run --session <id>`.
3. If multiple sessions are active, host presents a choice or resumes the most recently updated.

#### 5.2.3 MCP Tool Addition

```json
 {
   "name": "brain_resume",
   "description": "Resume an interrupted orchestration session. Call on startup if brain_status shows a running session.",
   "input_schema": {
     "session": "string (optional — omit to list)",
     "json": "boolean"
   }
 }
```

---

### 5.3 P1: Rate-Limit Aware Lease (`specd pinky suspend` / `specd pinky resume`)

**Goal:** Distinguish "worker is rate-limited and will return" from "worker is dead." Prevent false-positive retries.

#### 5.3.1 Lease Extension Protocol

```bash
 # Worker hits rate limit, expects to resume in 60s
 specd pinky suspend    --session <id> --worker <id> --attempt <n>    --reason "rate-limit" --resume-after-seconds 60

 # This extends the lease by resume-after-seconds + heartbeat buffer
 # Brain does NOT mark as retryable failure

 # Worker is back
 specd pinky resume    --session <id> --worker <id> --attempt <n>
```

#### 5.3.2 Brain Behavior

- `suspend` updates the lease expiry to `now + resumeAfter + heartbeatBuffer`.
- If the worker does not `resume` by the extended deadline, **then** it is treated as a failure.
- `suspend` can be called multiple times (capped at `maxSuspendSeconds`, e.g., 600).
- Reasons: `rate-limit`, `context-compaction`, `provider-maintenance`.

#### 5.3.3 Host Integration

```python
 def run_pinky(mission):
     try:
         do_work()
     except RateLimitError as e:
         specd_pinky_suspend(resume_after=e.retry_after)
         time.sleep(e.retry_after)
         specd_pinky_resume()
         # continue work
 ```

---

### 5.4 P1: Context Manifest Snapshot (`specd context --snapshot`)

**Goal:** Serialize the exact context that was loaded into the worker so a resumed worker can load the same (or equivalent) context without re-computation.

#### 5.4.1 Snapshot Format

```bash
 specd context <spec> --snapshot --out .specd/runtime/sessions/<session>/context-snapshots/<turn>.json
```

Produces:

```json
 {
   "turn": 5,
   "phase": "execute",
   "task": "T3",
   "manifest": { /* full contextManifest */ },
   "loadedFiles": [
     {"path": "src/auth.go", "sha256": "abc...", "lines": [1, 150]},
     {"path": "src/auth_test.go", "sha256": "def...", "lines": [1, 200]}
   ],
   "steeringDigest": "sha256-of-steering-files",
   "memoryDigest": "sha256-of-memory.md",
   "timestamp": "2026-06-27T02:00:00Z"
 }
```

#### 5.4.2 Resume Optimization

When a worker resumes from checkpoint:
1. Load the last snapshot for the task's turn.
2. Compare file SHAs. If unchanged, reference instead of re-loading.
3. If steering/memory changed, load deltas only.
4. This makes re-contextualization O(changed files) instead of O(all files).

---

### 5.5 P2: Progress-Weighted MaxWaits

**Goal:** Prevent `DriverStalled` on slow-but-progressing workers.

#### 5.5.1 Current Code Change

In `internal/core/orchestration_driver.go`, modify the wait logic:

```go
 // Current: waits++ on every non-progress step
 // Proposed: weight waits by worker progress telemetry

 func waitWeight(progress *PinkyProgressReport) int {
     if progress == nil { return 1 }
     // If worker reported progress in last 5 minutes, this is not a stall
     if time.Since(progress.LastReport) < 5*time.Minute { return 0 }
     return 1
 }
```

If any in-flight worker has reported progress within a configurable window (e.g., `progressTimeoutSeconds=300`), `waits` does not increment. This allows long compiles/tests without stall.

---

### 5.6 P2: Cross-Spec Program Recovery

**Goal:** If a program-level orchestration (`brain run --program`) is interrupted, resume the entire program DAG, not just one spec.

#### 5.6.1 Program Session State

Currently, `DriveProgramOrchestration` creates a parent session but child specs have their own sessions. On recovery:

```bash
 specd brain resume --program --session <parent-id>
 # Reconstructs:
 # 1. Which child specs were active
 # 2. Which had running workers
 # 3. Which were complete
 # 4. Re-starts the program driver loop from the current frontier
```

This requires persisting the program-level `inflightKeys` and `childSessions` map to disk on every step (already mostly there via `session.json` + `state.json`, but needs explicit program-state file).

---

## 6. Best Practices for Resilient Agent Orchestration

### 6.1 Host Agent Guidelines

1. **Always checkpoint before `/clear` or context compaction.**
   ```bash
   specd pinky checkpoint --progress <n> --summary "..."
   # THEN clear context
   ```

2. **Implement suspend/resume for every provider error.**
   - Rate limit → `pinky suspend --reason rate-limit --resume-after-seconds <retry-after>`
   - Context limit → `pinky checkpoint` + `pinky suspend --reason context-limit --resume-after-seconds 0`

3. **On startup, always run `brain resume --list` before accepting new user commands.**
   - If a session was running, resume it automatically.
   - This makes crashes transparent to the user.

4. **Never hold a lease without heartbeating.**
   - If the host is doing heavy computation, delegate to a background thread that heartbeats independently.

### 6.2 specd Configuration Best Practices

```json
 {
   "orchestration": {
     "enabled": true,
     "approvalPolicy": "session",
     "maxWorkers": 4,
     "maxRetries": 3,
     "sessionTimeoutMinutes": 240,
     "hostReportedCostLimitUSD": 50.00,
     "transport": {
       "kind": "file",
       "pollIntervalMillis": 500,
       "messageTTLSeconds": 3600,
       "leaseSeconds": 300,
       "heartbeatSeconds": 60
     },
     "resilience": {
       "checkpointEnabled": true,
       "autoResumeEnabled": true,
       "maxSuspendSeconds": 600,
       "progressTimeoutSeconds": 300,
       "contextSnapshotEnabled": true
     }
   }
 }
```

### 6.3 Monitoring & Alerting

| Signal | Action |
|---|---|
| `cost_warn` event | Host should log warning; consider checkpointing active tasks |
| `cost_halt` event | Session will escalate; host should prepare user notification |
| `context.compact` event | Host must perform `/clear`; failure to do so risks token-limit crash |
| `reclaim` event | Worker failed; host should investigate logs before retry |
| `stall` outcome | Driver gave up; host should escalate to human or increase limits |

---

## 7. Implementation Roadmap

### Phase 1: Foundation (Week 1-2)
- [ ] Add `CheckpointRecord` data model
- [ ] Implement `specd pinky checkpoint` command
- [ ] Implement `specd brain checkpoint` command
- [ ] Add `resume-from-checkpoint` decision action
- [ ] Modify `BuildPinkyMission` to include checkpoint context
- [ ] Unit tests for checkpoint flow

### Phase 2: Host Integration (Week 3-4)
- [ ] Implement `specd brain resume --list` and `specd brain resume --session`
- [ ] Add `autoResume` config block
- [ ] Update Claude Code / Cursor / VSCode host adapters to auto-resume on startup
- [ ] Add MCP `brain_resume` tool

### Phase 3: Graceful Degradation (Week 5-6)
- [ ] Implement `specd pinky suspend` / `specd pinky resume`
- [ ] Add rate-limit detection to reference worker implementations
- [ ] Implement `specd context --snapshot`
- [ ] Add progress-weighted wait logic to driver

### Phase 4: Hardening (Week 7-8)
- [ ] Cross-spec program recovery (`brain resume --program`)
- [ ] Stress testing: simulate host crash, token limit, rate limit
- [ ] Documentation: update AGENTS.md with resilience patterns
- [ ] Add `specd doctor --resilience` check

---

## 8. Conclusion

specd's architecture is already **deterministic, file-backed, and lease-safe**—the hard problems of distributed state are solved. The remaining work is **host-agent integration** and **proactive checkpointing** to bridge the gap between "the Brain knows what to do" and "the Host can continue seamlessly after any interruption."

With the Checkpoint Protocol, Auto-Resume Hook, and Rate-Limit Aware Lease, specd's Brain/Pinky model becomes genuinely **crash-proof and limit-proof**—transforming crashes and usage limits from bottlenecks into non-events.

**The agent reasons. The harness enforces. And now, the harness recovers.**

---

*Analysis generated from specd v0.x source code (Go, stdlib-only, MIT License).*
*All code contracts and invariants referenced are drawn from the actual implementation in `internal/core/` and `internal/cmd/`.*
