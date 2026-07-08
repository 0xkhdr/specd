# scripts/ — CI keep-vs-delete decision log (SPEC-01 T-01-01)

`.github/workflows/ci.yml` referenced eight tooling paths that did not exist in
the repository. SPEC-01 owns the final keep-vs-delete call for each. The working
assumption (analysis plan, Cross-Cutting Concern 1) is that these are **drift to
reconcile** — author the missing script — not an intentional teardown.

Decision: **author all eight** (none deleted). Every stress job maps to a real,
distinct subsystem invariant that SPEC-06 (crash-safety) will deepen; deleting
them would force SPEC-06 to recreate them.

| ci.yml invocation | Decision | Script | Invariant asserted |
|-------------------|----------|--------|--------------------|
| `make perf-gate` | **replace** with script (no root Makefile) | `perf-gate.sh` | A4: disabled-mode context budget does no work |
| `./scripts/coverage-check.sh` | author | `coverage-check.sh` | total coverage ≥ provisional floor (74.0%) |
| `./scripts/stress.sh` | author | `stress.sh` | one-spec state CAS/lock: no lost update (`records == revision`) |
| `./scripts/stress-acp.sh` | author | `stress-acp.sh` | ACP ledger line integrity: no torn line, no duplicate seq |
| `./scripts/stress-orchestration.sh` | author | `stress-orchestration.sh` | session-revision CAS advances; one winner |
| `./scripts/stress-program.sh` | author | `stress-program.sh` | per-spec isolation across concurrent multi-spec recovery |
| `./scripts/stress-brain-recovery.sh` | author | `stress-brain-recovery.sh` | crash recovery re-issues the mission exactly once |
| `./scripts/stress-checkpoint-fault.sh` | author | `stress-checkpoint-fault.sh` | crash mid-checkpoint: no double-claim, no orphaned lease |

## Provisional values SPEC-01 sets (downstream ratchets)

- **Coverage floor:** 74.0% (measured total on the SPEC-01 HEAD: 74.8%). SPEC-05
  owns the coverage policy and ratchets `FLOOR` in `coverage-check.sh` up.
- **`govulncheck` pin:** `v1.5.0` at `ci.yml` (was `@latest`). SPEC-04 owns the
  version rationale.

## Known blocker (see SPEC-01 spec.md → "Blockers Discovered")

The five orchestration stress scripts (`stress-acp`, `stress-orchestration`,
`stress-brain-recovery`, `stress-checkpoint-fault`, `stress-program`) currently
flake (~7%) on a genuine **double-dispatch race in `brain resume`** — a
crash-safety defect owned by SPEC-06, not fixable within SPEC-01's scope. The
scripts are authored and correct; they are the tripwire that exposed the bug.
`perf-gate.sh`, `coverage-check.sh`, and `stress.sh` are deterministic and green.
