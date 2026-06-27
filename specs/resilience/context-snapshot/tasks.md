# Tasks — Context Manifest Snapshot (R2)

## Wave 1 — Snapshot type & hashing
- [x] T1 — `ContextSnapshot` type + canonical JSON
  - why: serialize loaded context deterministically (Req 1)
  - role: builder
  - files: internal/context/snapshot.go (new)
  - contract: define `ContextSnapshot` + `LoadedFile{Path,SHA256,Lines}`; canonical-JSON
    encoder mirroring the orchestration path; validator for fields.
  - acceptance: round-trips byte-stable; rejects malformed entries.
  - verify: go test ./internal/context/ -run Snapshot
  - depends: —
  - requirements: 1

- [x] T2 — File + steering/memory digest helpers
  - why: SHAs drive delta detection (Req 1.2)
  - role: builder
  - files: internal/context/snapshot.go
  - contract: `sha256` over file content; `SteeringDigest`/`MemoryDigest` over concatenated
    steering files / memory.md using the loader the manifest builder already uses.
  - acceptance: digests stable for unchanged inputs; change on any byte change.
  - verify: go test ./internal/context/ -run Digest
  - depends: T1
  - requirements: 1

- [x] T3 — `ContextSnapshotDir` path helper + config gate
  - why: default output location + opt-in (Req 2.2, 4)
  - role: builder
  - files: internal/core/runtime_paths.go, internal/core/specfiles.go,
    internal/core/embed_templates/config.json
  - contract: add `ContextSnapshotDir(sessionID)`; add `ContextSnapshotEnabled` to shared
    `Resilience` block (default false, omitempty). Byte-identical config when absent.
  - acceptance: path resolves under session dir; absent config → no new bytes.
  - verify: go test ./internal/core/ -run "RuntimePaths|Config|Drift"
  - depends: —
  - requirements: 2, 4

## Wave 2 — Command & comparator
- [x] T4 — `specd context --snapshot --out`
  - why: emit snapshot for current turn (Req 2)
  - role: builder
  - files: internal/cmd/context.go
  - contract: after building manifest, hash loaded files + digests, write `ContextSnapshot` to
    `--out` or default path; gate on config. Non-snapshot output byte-unchanged.
  - acceptance: snapshot file written with correct SHAs; plain `context` output identical (golden).
  - verify: go test ./internal/cmd/ -run Context
  - depends: T2, T3
  - requirements: 2

- [x] T5 — `DiffContextSnapshot` comparator
  - why: O(changed-files) resume (Req 3)
  - role: builder
  - files: internal/context/snapshot.go
  - contract: compare snapshot SHAs vs current files; return changed/unchanged + steering/memory
    changed flags.
  - acceptance: unchanged files reported reference; any edit → changed; digest change flagged.
  - verify: go test ./internal/context/ -run Diff
  - depends: T2
  - requirements: 3

## Wave 3 — Wire into resume brief
- [x] T6 — Resume brief consumes diff summary
  - why: tell resuming worker what to reload (Req 3)
  - role: builder
  - files: internal/core/pinky_brief.go
  - contract: when a checkpoint-resume brief is rendered and a snapshot exists, run
    `DiffContextSnapshot` and emit a reference/reload summary. If checkpoint-protocol resume not
    present, leave a guarded hook (no-op).
  - acceptance: resume brief lists "reference (unchanged)" vs "reload (changed)" files.
  - verify: go test ./internal/cmd/ -run Brief
  - depends: T5
  - requirements: 3

- [x] T7 — Snapshot + resume delta test
  - why: prove the optimization end-to-end (Req 1,2,3)
  - role: verifier
  - files: internal/cmd/context_manifest_cmd_test.go
  - contract: emit snapshot; modify one file; assert diff reports exactly that file changed and
    the rest reference.
  - acceptance: test green; delta is minimal and correct.
  - verify: go test ./internal/cmd/ -run Context
  - depends: T6
  - requirements: 1, 2, 3
