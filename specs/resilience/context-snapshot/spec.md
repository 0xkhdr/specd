# Spec — Context Manifest Snapshot (R2)

**Priority:** P1 · **Wave:** 2 · **Gap:** R2 (no host context serialization).

## Introduction

The `ContextLedger` tracks token estimates and host-reported actuals but never captures *what
was actually loaded* into the worker: which files, at which SHAs, plus the steering/memory
digests. After a restart or checkpoint-resume, the worker must re-read everything and rebuild
mental state — re-contextualization is O(all files) with spec complexity.

This spec serializes a **context snapshot** per turn: the manifest plus the exact loaded files
with content SHAs and line ranges, and steering/memory digests. On resume, a worker compares
SHAs and reloads only what changed — re-contextualization becomes O(changed files).

## Current-state grounding

- Manifest type + builder: `MissionContextManifest` in
  `internal/context/manifest_types.go`; `BuildContextManifest` in
  `internal/context/manifest.go`.
- Context command: `internal/cmd/context.go` (builds the manifest the worker loads).
- Runtime paths: `internal/core/runtime_paths.go` — add a `ContextSnapshotDir(sessionID)`.
- Checkpoint coupling: `CheckpointRecord.ContextManifest` (from `checkpoint-protocol`) — the
  snapshot is the richer, file-level companion the resume path consults.

## Requirements

### Requirement 1 — Snapshot format
**User story:** As a resuming worker, I want the exact context the prior turn loaded, so I can
reconstruct it without recomputation.

**Acceptance criteria:**
1. THE SYSTEM SHALL define a `ContextSnapshot` with: `Turn`, `Phase`, `Task`, `Manifest`
   (full `MissionContextManifest`), `LoadedFiles[]{Path, SHA256, Lines:[start,end]}`,
   `SteeringDigest`, `MemoryDigest`, `Timestamp`.
2. THE SYSTEM SHALL compute `SHA256` over file content and `SteeringDigest`/`MemoryDigest` over
   the concatenated steering files / `memory.md` respectively.
3. THE SYSTEM SHALL serialize snapshots as canonical JSON (stable byte output).

### Requirement 2 — `specd context --snapshot`
**User story:** As the host, I want to emit a snapshot for the current turn.

**Acceptance criteria:**
1. WHEN a host runs `specd context <spec> --snapshot --out <path>` THE SYSTEM SHALL build the
   manifest and write a `ContextSnapshot` to `<path>`.
2. WHERE `--out` is omitted THE SYSTEM SHALL default to
   `.specd/runtime/sessions/<session>/context-snapshots/<turn>.json`.
3. THE existing `specd context <spec>` (no `--snapshot`) output SHALL be byte-unchanged.

### Requirement 3 — Resume delta optimization
**User story:** As a resuming worker, I want to skip reloading unchanged files.

**Acceptance criteria:**
1. WHEN resuming, THE SYSTEM SHALL load the latest snapshot for the task's turn and compare each
   `LoadedFiles` SHA against the current file content.
2. IF a file's SHA is unchanged THEN THE SYSTEM SHALL mark it "reference, do not reload" in the
   resume brief.
3. IF `SteeringDigest` or `MemoryDigest` changed THEN THE SYSTEM SHALL flag steering/memory as
   "reload (changed)".
4. THE SYSTEM SHALL surface a per-file changed/unchanged summary so re-contextualization is
   O(changed files).

### Requirement 4 — Config
**Acceptance criteria:**
1. THE SYSTEM SHALL gate snapshot writing behind
   `orchestration.resilience.contextSnapshotEnabled` (default `false`); absent → byte-identical
   config; disabled → `--snapshot` is a no-op error or hidden.

## Design

- `ContextSnapshot` lives in `internal/context/` beside the manifest types; canonical-JSON
  helper mirrors the orchestration canonical path.
- `--snapshot` flag added to `context.go`: after building the manifest, hash each loaded file
  (read content, `crypto/sha256`), compute steering/memory digests (the digest helpers can reuse
  whatever steering/memory loader the manifest builder already calls), write the snapshot.
- A resume-time comparator `DiffContextSnapshot(snapshot, root)` returns
  `{Unchanged[], Changed[], SteeringChanged, MemoryChanged}`; the checkpoint-resume brief
  (from `checkpoint-protocol` T7) consumes it to render the reference/reload summary.
- Path helper `ContextSnapshotDir(sessionID)` → `sessions/<id>/context-snapshots`.

## Coordination
- Consumes the resume brief from `checkpoint-protocol` (T7). If checkpoint-protocol is not yet
  landed, the comparator + `--snapshot` command can ship standalone and wire into the brief when
  checkpoint resume exists.
- Shares the `Resilience` config block.

## Out of scope
- Capturing the host's raw conversation transcript — only the deterministic, file-derived
  context is serialized (the transcript is the host's responsibility).

## Risks
- **SHA drift cost:** hashing many large files per turn adds I/O. Mitigated by only hashing the
  files the manifest already selected (bounded), and gating behind config.
- **Stale snapshot:** comparing against a snapshot from a much earlier turn could over-report
  changes; acceptable — it degrades to today's full reload, never incorrect.
