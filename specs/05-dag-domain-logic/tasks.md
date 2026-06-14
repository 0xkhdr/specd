# Stage 05 — Tasks

Branch: `refactor/05-dag-domain-logic`.

## T1 — CriticalPath cycle-safety (F1)
**Files:** `internal/core/dag.go`, `internal/core/dag_test.go`.

1. At the top of `CriticalPath` (`dag.go:265`), add:
   ```go
   if DetectCycle(tasks) != nil {
       return nil
   }
   ```
2. Add doc comment: "Precondition: acyclic. Returns nil if a cycle is present."
3. Tests: linear `T1→T2→T3`; diamond `T1→{T2,T3}→T4`; two disconnected chains
   (longest wins); single node; empty; cyclic→nil.

**Verify:** `go test ./internal/core/ -run DAG`

## T2 — ordinal contract + tests (F2)
**File:** `internal/core/dag_test.go` (and a doc comment in `dag.go:28`).

1. Document: ids are `T\d+` (per `taskRE`); `ordinal` reads the first digit run.
2. Tests asserting total order over `T1..T20`, and that `T10 > T9`
   (numeric, not lexicographic) — this guards the sort in `NextRunnable`.

**Verify:** `go test ./internal/core/ -run Ordinal`

## T3 — Parser robustness (F3)
**Files:** `internal/core/tasksparser.go`, `internal/core/tasksparser_test.go`.

1. Add table tests for malformed inputs; assert no panic and a deterministic
   result or a line-numbered `SpecdError`:
   - empty deps element: `depends: T1, , T3`
   - duplicate task id (define policy: reject with SpecdError; implement if not)
   - checkbox line with no meta block
   - meta line before any task
   - evidence containing `·`
2. `ParseDepends` (find in core): skip empty/whitespace elements after split.
3. Annotation separator safety: in `ApplyTaskAnnotation` (writer, used by
   task.go), escape `·` in evidence (e.g. replace with `·`-safe encoding
   or percent-escape) and decode in `annotCompleteRE` parse — OR assert evidence
   is single-line and document that `·` in evidence is normalized. Keep reader
   backward-compatible.
4. Duplicate-id policy: in `ParseTasks`, track seen ids; on duplicate return
   `SpecdError` with the line number.

**Verify:** `go test ./internal/core/ -run Tasks && go test ./internal/cmd/ -run Task`

## T4 — EARS audit (F4)
**Files:** `internal/core/ears.go`, `internal/core/ears_test.go`, `docs/validation-gates.md`.

1. Enumerate recognized EARS forms in a doc comment + `docs/`.
2. Add table tests: ubiquitous, event (When), state (While), unwanted (If),
   optional (Where), complex/combined; plus false-positive guards (a valid
   requirement that must NOT be flagged).
3. Fix only patterns that misflag a template-valid requirement; otherwise no
   behavior change.

**Verify:** `go test ./internal/core/ -run Ears`

## T5 — Boot determinism (F5)
**Files:** `internal/core/boot.go`, `internal/core/boot_detectors.go`, `internal/core/boot_test.go`.

1. Ensure detectors run in a fixed, documented priority order (stable slice, not
   map iteration). If currently map-ordered, switch to an ordered slice.
2. Document tie-break for polyglot repos (e.g. primary = most files / explicit
   priority list).
3. Golden tests: empty dir, Go-only, Node-only, Go+Node monorepo, unknown →
   assert deterministic `boot.json` shape.
4. Confirm `CheckBootFreshness` has no time/env/random nondeterminism.
   (Regex-in-loop at `boot_detectors.go:45` is hoisted in Stage 06 — leave a
   `// TODO(stage06): hoist` marker.)

**Verify:** `go test ./internal/core/ -run Boot && go test ./internal/cmd/ -run Boot`

## T6 — Enrich freshness contract (F6)
**Files:** `internal/core/enrich*.go`, `internal/core/enrich_test.go`, `docs/`.

1. Document what makes enrichment stale and what `check --enrich` compares.
2. Add fresh vs stale detection tests.

**Verify:** `go test ./internal/core/ -run Enrich && go test ./internal/cmd/ -run Enrich`

## Done-when
- `go vet ./... && gofmt -l . && go test -race ./...` green.
- CriticalPath cycle-safe; parser panic-free with defined duplicate policy;
  boot deterministic with goldens; EARS + enrich documented and tested.
